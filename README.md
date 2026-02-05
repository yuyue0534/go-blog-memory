# go-blog-memory

👉 一个“内存型博客管理系统”

特点：
- ✅ Go 语言
- ✅ 不依赖数据库（所有数据在内存）
- ✅ 服务持续运行（Koyeb Free OK）
- ✅ REST API
- ✅ 支持增删改查
- ✅ 并发安全
- ✅ 重启即清空（符合“缓存型”定位）
- ✅ 可直接部署到 Koyeb

博客模型 BlogPost
- id
- title
- content
- created_at
- updated_at

**API列表：**

| 方法     | 路径          | 说明     |
| ------ | ----------- | ------ |
| GET    | /health     | 健康检查   |
| GET    | /posts      | 获取所有文章 |
| GET    | /posts/{id} | 获取单篇   |
| POST   | /posts      | 创建文章   |
| PUT    | /posts/{id} | 更新文章   |
| DELETE | /posts/{id} | 删除文章   |
