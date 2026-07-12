# Enterprise-RAG 开发指南

本文面向项目开发者，集中说明技术选型、代码结构、配置关系、开发运行和扩展约定。产品定位与系统能力请先阅读 [项目说明](../README.md)。

## 技术架构

| 区域 | 技术 | 主要职责 |
| --- | --- | --- |
| Web | React、TypeScript、Vite、Ant Design | 页面交互、流式回答、国际化与状态展示 |
| API | Go、Go-zero | 接口接入、鉴权、参数校验与业务编排 |
| 业务数据 | PostgreSQL | 用户、知识库、文档、知识片段、会话与任务日志 |
| 向量检索 | Milvus | 文档向量存储与相似度召回 |
| 文件存储 | MinIO | 原始上传文件保存 |
| 异步任务 | NATS JetStream | 解析、切分、向量化和删除任务流转 |
| 模型服务 | OpenRouter | 对话模型、向量模型与联网搜索入口 |

## 目录结构

```text
Enterprise-RAG/
├── backend/
│   ├── api/                         按业务模块拆分的接口定义
│   ├── etc/                         服务、基础设施与应用配置
│   └── internal/
│       ├── handler/                 Go-zero 生成的请求入口
│       ├── logic/                   接口业务编排
│       ├── service/                 可复用的领域服务
│       ├── infrastructure/          数据库、模型、存储与解析适配
│       ├── repository/              数据访问
│       ├── worker/                  异步任务消费者
│       ├── presenter/               响应数据组装
│       ├── middleware/              鉴权等中间件
│       └── model/                   领域数据结构
├── frontend/
│   └── src/
│       ├── api/                     后端请求封装
│       ├── components/              公共界面组件
│       ├── i18n/                    多语言资源
│       ├── layouts/                 页面布局
│       ├── pages/                   业务页面
│       ├── types/                   前端类型
│       └── utils/                   通用工具
├── docs/                            项目文档与架构图
└── docker-compose.yml               本地基础服务
```

## 后端分层约定

请求按以下方向流转：

```text
Handler  →  Logic  →  Service  →  Repository / Infrastructure
```

- `handler` 由 Go-zero 生成，只负责接收请求，不手工加入业务逻辑。
- `logic` 负责权限校验、参数转换和业务用例编排。
- `service` 承担可复用的检索、问答、文档处理和任务能力。
- `repository` 隔离 PostgreSQL 数据访问。
- `infrastructure` 隔离 Milvus、MinIO、模型服务和文档解析实现。
- `presenter` 统一把领域结果转换为接口响应。

新增功能时优先沿现有边界扩展，避免在 Handler 中写逻辑，也避免 Logic 直接承载复杂算法或外部服务细节。

## API 组织

`backend/api/rag.api` 是接口定义入口，各业务模块独立维护：

| 模块 | 内容 |
| --- | --- |
| `common.api` | 公共结构与健康检查 |
| `auth.api` | 注册、登录和用户信息 |
| `subject.api` | 知识库管理 |
| `document.api` | 文档上传、查询与删除 |
| `retrieval.api` | 检索与效果评估 |
| `chat.api` | 会话、消息与流式问答 |
| `admin.api` | 任务、日志与运维操作 |

接口定义变更后，在项目根目录执行：

```bash
goctl api go -api backend/api/rag.api -dir backend
```

生成后应检查路由、类型和依赖注入变更。生成的 Handler 保持原样，业务实现放入对应 Logic 或 Service。

## 配置结构

后端从 `backend/etc/rag-api.yaml` 启动，并继续加载两类配置：

| 配置文件 | 内容 |
| --- | --- |
| `rag-api.yaml` | 服务名称、监听地址、端口、超时和配置入口 |
| `infrastructure.yaml` | PostgreSQL、Redis、NATS、MinIO 与 Milvus 连接信息 |
| `application.yaml` | Worker、切块、检索、可靠性、模型和评估参数 |
| `evaluation-cases.yaml` | 固定检索评估问题集 |

密钥不写入仓库。当前模型服务使用环境变量：

```bash
export OPENROUTER_API_KEY=你的密钥
```

检索与向量索引使用相同的向量维度。更换向量模型时，需要同步确认配置维度、Milvus Collection 和已有文档索引，避免新旧向量不兼容。

## 本地开发

### 1. 启动基础服务

```bash
docker compose up -d
```

基础服务包括 PostgreSQL、NATS、MinIO、Etcd 和 Milvus。

### 2. 启动后端

```bash
cd backend
go run .
```

默认监听 `http://localhost:9999`。

### 3. 启动前端

```bash
cd frontend
pnpm install
pnpm dev
```

默认访问 `http://localhost:5173`。

## 数据与索引

数据库迁移位于 `backend/internal/infrastructure/postgres/migrations/`，一个业务表对应一个迁移文件。迁移按编号顺序执行，已经投入使用的迁移不回写修改；表结构变化通过新增迁移表达。

文档入库会同时产生三类数据：

```text
MinIO 原始文件
PostgreSQL 文档、知识片段与任务记录
Milvus 知识片段向量
```

涉及文档删除、重新索引或知识库删除时，必须同步考虑三类数据的一致性。

## 文档处理扩展

文档解析器位于 `backend/internal/infrastructure/parser/`，不同文件格式保持独立实现。新增格式时需要同时完成：

1. 文件类型识别与上传校验
2. 解析器实现
3. 标题、页码、表格等元数据输出
4. 错误信息转换与多语言展示
5. 正常文件、空文件、损坏文件和特殊结构测试

解析结果统一进入切块流程，不应在格式解析器中重复实现索引或检索逻辑。

## 检索与问答扩展

检索能力位于 `backend/internal/service/retrieval/`，问答编排位于 `backend/internal/service/chatflow/`。调整检索策略时，应分别观察：

- 目标知识是否进入候选结果
- 重排后是否保留有效片段
- 最终引用是否真正支撑回答
- 无关问题是否触发正确兜底
- 延迟和模型调用次数是否可接受

新增问答路由时，需要同时补充路由判断、处理流程、状态事件、评估用例和多语言文案，避免只完成后端分支而没有可见反馈。

## 测试与检查

后端：

```bash
cd backend
go test ./...
go vet ./...
go build ./...
```

前端：

```bash
cd frontend
pnpm lint
pnpm build
```

涉及 RAG 效果的修改不能只以编译通过为结论。至少需要上传真实文档，验证关键词问题、自然语言问题、解释型问题、概览问题、无答案问题和引用对应关系。

## 提交前检查

- Handler 是否保持生成状态
- Logic 是否只做接口用例编排
- 公共能力是否放在合适的 Service 中
- 外部依赖是否通过 Infrastructure 隔离
- 新增提示和错误是否完成中文、英文、日文翻译
- 文档删除与索引更新是否保持数据一致
- 后端测试、静态检查和构建是否通过
- 前端检查和生产构建是否通过
- README 与开发文档是否和当前实现一致
