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

## 常用检查

```bash
cd backend && go build ./...
cd frontend && npm run lint
cd frontend && npm run build
```
