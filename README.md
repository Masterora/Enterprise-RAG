# Enterprise-RAG v1.0.0

企业知识库 RAG 基础版，提供知识库管理、文档入库、切块索引和检索验证能力。

## 项目结构

`backend/` Go + go-zero API 服务  
`frontend/` React + Vite 前端  
`docker-compose.yml` 本地依赖环境

## 本地启动

```bash
docker compose up -d

cd backend
go run .

cd ../frontend
npm install
npm run dev
```

访问地址：

```text
http://localhost:5173
```

环境要求：

- 前端需要 Node `20.19+` 或 `22.12+`。
- 本项目的 Postgres 默认映射为宿主机 `55432` 端口，避免和本机已有 `5432` 实例冲突。
- 当前默认使用 `mock` embedding 便于本地联调。
- 如需切换 `OpenAI` embedding，把 `Embedding.Provider` 改成 `openai`，并设置 `OPENAI_API_KEY`。

## 使用流程

1. 先进入 `登录` 页面，输入用户名和密码。
2. 首次登录会自动创建用户，后续使用同一组账号登录。
3. 进入 `知识库` 页面，创建一个知识库。
4. 进入 `文档` 页面，选择知识库并上传 `Markdown` 或 `PDF`。
5. 上传成功后，文档会经历 `uploaded -> parsing -> parsed -> chunking -> chunked -> embedding -> indexed`。
6. 进入 `问答` 页面，选择知识库并发起检索，查看命中章节、来源文档、页码、相似度和正文预览。

## v1.0.0 功能范围

- 用户登录与 `me` 接口
- 知识库基础 CRUD
- 文档上传到 MinIO
- 文档解析、切块、向量化、Milvus 入库的异步任务链路
- 解析格式支持 `Markdown` 和 `PDF`
- 检索接口与检索结果展示

说明：

- `Markdown` 文档没有真实页码，检索结果中的页码会显示为空。
- 当前默认使用 `mock` embedding 做本地链路验证；切换到 `OpenAI` 后可得到正式向量效果。

## 数据库迁移

后端启动时会自动执行未应用的 migration。

```text
backend/internal/infrastructure/postgres/migrations
```

修改表结构时不要改旧 migration，新增一个更高版本的 `.up.sql` 文件，例如：

```text
000002_add_subject_indexes.up.sql
```
当前项目启动流程只执行 `.up.sql`。

## API

所有业务接口统一使用 `POST`。

```text
POST /api/auth/login
POST /api/users/me
POST /api/subjects/create
POST /api/subjects/update
POST /api/subjects/delete
POST /api/subjects/list
POST /api/subjects/detail
POST /api/documents/upload
POST /api/documents/list
POST /api/retrieval/search
```

## 常用检查

```bash
cd backend && go build ./...
cd frontend && npm run lint
cd frontend && npm run build
```
