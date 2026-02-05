package main

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BlogPost struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BlogStore struct {
	mu    sync.RWMutex
	posts map[string]BlogPost
}

func NewBlogStore() *BlogStore {
	return &BlogStore{
		posts: make(map[string]BlogPost),
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store := NewBlogStore()
	r := gin.Default()

	// 健康检查（Koyeb / Uptime 用）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now(),
		})
	})

	// 获取所有文章
	r.GET("/posts", func(c *gin.Context) {
		store.mu.RLock()
		defer store.mu.RUnlock()

		list := make([]BlogPost, 0, len(store.posts))
		for _, post := range store.posts {
			list = append(list, post)
		}
		c.JSON(http.StatusOK, list)
	})

	// 获取单篇文章
	r.GET("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		store.mu.RLock()
		post, ok := store.posts[id]
		store.mu.RUnlock()

		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
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

		now := time.Now()
		post := BlogPost{
			ID:        uuid.NewString(),
			Title:     input.Title,
			Content:   input.Content,
			CreatedAt: now,
			UpdatedAt: now,
		}

		store.mu.Lock()
		store.posts[post.ID] = post
		store.mu.Unlock()

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

		store.mu.Lock()
		defer store.mu.Unlock()

		post, ok := store.posts[id]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}

		post.Title = input.Title
		post.Content = input.Content
		post.UpdatedAt = time.Now()
		store.posts[id] = post

		c.JSON(http.StatusOK, post)
	})

	// 删除文章
	r.DELETE("/posts/:id", func(c *gin.Context) {
		id := c.Param("id")

		store.mu.Lock()
		defer store.mu.Unlock()

		if _, ok := store.posts[id]; !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		delete(store.posts, id)
		c.Status(http.StatusNoContent)
	})

	// 启动服务（Koyeb 关键点）
	r.Run(":" + port)
}
