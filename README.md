# Enterprise-RAG v1.0.0

Enterprise-RAG 是一个企业知识库问答系统。

它可以把企业内部的文档整理成可检索的知识库，用户通过网页完成知识库管理、文档上传和问答。系统会根据知识库中的资料生成答案，并附带引用来源，方便核对答案来自哪一份文档、哪一段内容。

## 适用场景

- 企业内部制度、流程、手册查询
- 项目文档和交付文档问答
- 产品说明、技术文档检索
- 需要“答案 + 引用来源”的知识问答场景

## 系统组成

本系统主要由以下部分组成：

- **管理界面**：用于登录系统、创建知识库、上传文档和发起问答
- **文档处理模块**：负责解析上传的文件，并整理成可以检索的内容
- **问答模块**：根据用户问题查找相关资料，生成答案并返回引用来源
- **数据存储模块**：负责保存用户、知识库、文档和索引数据

可以把它理解为一个“文档管理 + 智能检索 + 引用问答”的系统。

## 系统架构

![系统框架图](./docs/system-architecture.svg)

## 运行环境

启动本项目需要以下环境：

- Docker 与 Docker Compose
- Go `1.26+`
- Node.js `20.19+` 或 `22.12+`
- pnpm `9+`

## 启动步骤

### 1. 启动基础依赖服务

```bash
docker compose up -d
```

### 2. 配置模型密钥

当前默认使用 OpenRouter 作为模型服务。启动后端前，先设置环境变量：

```bash
export OPENROUTER_API_KEY=你的密钥
```

如需调整模型配置，可修改 [backend/etc/rag-api.yaml](/Users/ran/Project/Enterprise-RAG/backend/etc/rag-api.yaml)。

### 3. 启动后端服务

```bash
cd backend
go run .
```

后端默认地址：

```text
http://localhost:9999
```

### 4. 启动前端页面

```bash
cd frontend
pnpm install
pnpm dev
```

前端默认地址：

```text
http://localhost:5173
```

## go-zero 代码生成

如需根据 API 定义重新生成 go-zero 相关文件，可在项目根目录执行：

```bash
goctl api go -api backend/api/rag.api -dir backend
```

## 使用流程

系统启动后，可以按下面的顺序使用：

1. 注册或登录系统
2. 创建一个知识库
3. 进入文档页面，选择知识库并上传文件
4. 等待文档完成索引
5. 进入问答页面，选择知识库并输入问题
6. 查看系统生成的答案和对应引用来源

## 目录说明

```text
Enterprise-RAG/
├── backend/             后端服务
├── frontend/            前端页面
└── docker-compose.yml   本地依赖服务配置
```

## 常用检查

```bash
cd backend && go test ./...
cd frontend && pnpm build
```
