package main

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type BlogPost struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type dbPostRow struct {
	ID        string
	Title     string
	Content   string
	CreatedAt string
	UpdatedAt string
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db := mustOpenDB()
	defer db.Close()

	r := gin.Default()

	// 健康检查（Zeabur / Uptime 用）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"author": "yuyue3",
			"time":   time.Now(),
		})
	})

	// 获取所有文章（按更新时间倒序：最新在前）
	r.GET("/posts", func(c *gin.Context) {
		list, err := listPosts(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list)
	})

	// 获取单篇文章
	r.GET("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		post, err := getPost(db, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, post)
	})

	// 创建文章
	r.POST("/posts", func(c *gin.Context) {
		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		now := time.Now().UTC()
		post := BlogPost{
			ID:        uuid.NewString(),
			Title:     input.Title,
			Content:   input.Content,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := insertPost(db, post); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, post)
	})

	// 更新文章
	r.PUT("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		updated := time.Now().UTC()
		post, err := updatePost(db, id, input.Title, input.Content, updated)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, post)
	})

	// 删除文章
	r.DELETE("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		ok, err := deletePost(db, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}

		c.Status(http.StatusNoContent)
	})

	// 启动服务（Zeabur 关键点）
	_ = r.Run(":" + port)
}

// -------------------- DB init & migrations --------------------

func mustOpenDB() *sql.DB {
	// 建议你在 Zeabur 挂载 Volume 到 /data，然后设置 DB_PATH=/data/blog.db
	// 本地默认存 ./data/blog.db
	dbPath := os.Getenv("DB_PATH")
	if strings.TrimSpace(dbPath) == "" {
		dbPath = "./data/blog.db"
	}

	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}

	// SQLite 一些推荐设置（更适合服务端）
	// busy_timeout 避免并发写入时立刻报 “database is locked”
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000; PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;`); err != nil {
		panic(err)
	}

	if err := migrate(db); err != nil {
		panic(err)
	}

	return db
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS posts (
  id         TEXT PRIMARY KEY,
  title      TEXT NOT NULL,
  content    TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_posts_updated_at ON posts(updated_at);
`)
	return err
}

// -------------------- DB helpers (CRUD) --------------------

func insertPost(db *sql.DB, post BlogPost) error {
	_, err := db.Exec(
		`INSERT INTO posts (id, title, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		post.ID,
		post.Title,
		post.Content,
		post.CreatedAt.Format(time.RFC3339Nano),
		post.UpdatedAt.Format(time.RFC3339Nano),
	)
	return err
}

func getPost(db *sql.DB, id string) (BlogPost, error) {
	var row dbPostRow
	err := db.QueryRow(
		`SELECT id, title, content, created_at, updated_at FROM posts WHERE id = ?`,
		id,
	).Scan(&row.ID, &row.Title, &row.Content, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return BlogPost{}, err
	}

	return toBlogPost(row)
}

func listPosts(db *sql.DB) ([]BlogPost, error) {
	rows, err := db.Query(
		`SELECT id, title, content, created_at, updated_at
		 FROM posts
		 ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]BlogPost, 0, 32)
	for rows.Next() {
		var r dbPostRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Content, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		post, err := toBlogPost(r)
		if err != nil {
			return nil, err
		}
		list = append(list, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func updatePost(db *sql.DB, id, title, content string, updatedAt time.Time) (BlogPost, error) {
	res, err := db.Exec(
		`UPDATE posts
		 SET title = ?, content = ?, updated_at = ?
		 WHERE id = ?`,
		title,
		content,
		updatedAt.Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return BlogPost{}, err
	}

	aff, err := res.RowsAffected()
	if err != nil {
		return BlogPost{}, err
	}
	if aff == 0 {
		return BlogPost{}, sql.ErrNoRows
	}

	// 更新后再取一遍，保证返回完整对象（含 created_at）
	return getPost(db, id)
}

func deletePost(db *sql.DB, id string) (bool, error) {
	res, err := db.Exec(`DELETE FROM posts WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	aff, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return aff > 0, nil
}

func toBlogPost(r dbPostRow) (BlogPost, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return BlogPost{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if err != nil {
		return BlogPost{}, err
	}
	return BlogPost{
		ID:        r.ID,
		Title:     r.Title,
		Content:   r.Content,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
