# Enterprise-RAG

企业知识库 RAG 问答系统，支持知识库、文档索引和问答模块的后续扩展。

## 项目结构

```text
backend/           Go + go-zero API 服务
frontend/          React + Vite 前端
docker-compose.yml 本地依赖环境
```

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

## 使用流程

1. 进入 `知识库` 页面，创建一个知识库。
2. 进入 `文档` 页面，选择知识库并上传文件。
3. 上传成功后，文档会以 `uploaded` 状态出现在文档列表。
4. 进入 `问答` 页面，选择知识库并输入问题。

当前版本已经接入知识库创建、知识库列表、文档上传和文档列表。问答页先提供输入和模拟回答，真实 RAG 检索、文档解析、切块、embedding 和大模型回答将在后续模块接入。

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
POST /api/subjects/create
POST /api/subjects/list
POST /api/subjects/detail
POST /api/documents/upload
POST /api/documents/list
```

## 常用检查

```bash
cd backend && go build ./...
cd frontend && npm run lint
cd frontend && npm run build
```
