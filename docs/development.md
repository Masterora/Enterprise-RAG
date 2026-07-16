# Enterprise-RAG 开发指南

本文面向项目开发者，集中说明技术选型、代码结构、配置关系、开发运行和扩展约定。产品定位与系统能力请先阅读 [项目说明](../README.md)。

## 技术架构

| 区域 | 技术 | 主要职责 |
| --- | --- | --- |
| Web | React、TypeScript、Vite、Ant Design | 页面交互、流式回答、国际化与状态展示 |
| API | Go、Go-zero | 接口接入、鉴权、参数校验与业务编排 |
| Agent | Python、FastAPI、LangGraph | 请求路由、工具编排、检索改写、答案生成与状态持久化 |
| 业务数据 | PostgreSQL | 用户、知识库、文档、知识片段、会话与任务日志 |
| 流量控制 | Redis | 问答、检索和上传接口的跨实例滑动窗口限流 |
| 向量检索 | Milvus | 文档向量存储与相似度召回 |
| 文件存储 | MinIO | 原始上传文件保存 |
| 异步任务 | NATS JetStream | 持久任务、显式 ACK / NAK、延迟重投与消费者组 |
| 模型服务 | OpenRouter | Agent 对话生成、联网搜索、模型切换以及 API 向量化 |

## 目录结构

```text
Enterprise-RAG/
├── api/
│   ├── api/                         按业务模块拆分的接口定义
│   ├── etc/                         服务、基础设施与应用配置
│   ├── rag.go                       API 与 Worker 进程入口
│   └── internal/
│       ├── handler/                 Go-zero 生成的请求入口
│       ├── logic/                   接口业务编排
│       ├── service/                 可复用的领域服务
│       │   └── retrieval/           混合检索、重排、裁剪与单题评估
│       ├── infrastructure/          数据库、模型、存储与解析适配
│       ├── repository/              数据访问
│       ├── worker/                  异步任务消费者
│       ├── presenter/               响应数据组装
│       ├── middleware/              鉴权与分布式限流中间件
│       └── model/                   领域数据结构
├── agent/
│   ├── config/                      FastAPI / LangGraph 配置
│   ├── src/enterprise_agent/
│   │   ├── api/routers/             APIRouter 接口模块
│   │   ├── main.py                  FastAPI 应用工厂与 Router 注册
│   │   ├── dependencies.py          FastAPI 依赖与服务鉴权
│   │   ├── lifespan.py              客户端与 Checkpointer 生命周期
│   │   ├── graph.py                 LangGraph 状态图
│   │   └── persistence.py           PostgreSQL Checkpointer
│   └── tests/                       状态图、接口与持久化测试
├── web/
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
Handler → Logic → Agent HTTP Client → FastAPI → LangGraph → Internal Tool API
```

- `handler` 由 Go-zero 生成，只负责接收请求，不手工加入业务逻辑。
- `logic` 负责权限校验、参数转换和调用 Agent，不包含工具选择分支。
- FastAPI `APIRouter` 负责 Agent 内部接口，LangGraph 负责显式状态转移和答案合成。
- `service` 承担可复用的检索、文档处理和任务能力。
- `repository` 隔离 PostgreSQL 数据访问。
- `infrastructure` 隔离模型 Provider、Milvus、MinIO 和文档解析实现。
- `presenter` 统一把领域结果转换为接口响应。

新增功能时优先沿现有边界扩展，避免在 Handler 中写逻辑，也避免 Logic 直接承载复杂算法或外部服务细节。

## API 组织

`api/api/` 保存接口契约，各业务模块独立维护，由 `rag.api` 聚合并生成统一路由：

| 模块 | 内容 |
| --- | --- |
| `common.api` | 公共结构与健康检查 |
| `auth.api` | 注册、登录和用户信息 |
| `subject.api` | 知识库管理 |
| `document.api` | 文档上传、查询与删除 |
| `retrieval.api` | 检索与效果评估 |
| `chat.api` | 会话、消息与流式问答 |
| `admin.api` | 任务、日志与运维操作 |

接口定义变更后先执行格式与语法检查：

```bash
goctl api format --dir api/api
```

需要重新生成 Handler、路由或类型时执行：

```bash
cd api
goctl api go -api api/rag.api -dir .
```

生成的 Handler 保持原样，业务实现放入对应 Logic 或 Service。

## 配置结构

后端入口配置加载应用与基础设施配置：

| 配置文件 | 内容 |
| --- | --- |
| `rag-api.yaml` | API 监听地址与共享配置入口 |
| `infrastructure.yaml` | PostgreSQL、Redis、NATS JetStream、MinIO 与 Milvus 连接信息 |
| `application.yaml` | Worker、切块、检索、Agent、OpenRouter 向量、指标和评估参数 |

密钥不写入仓库。API 与 Agent 使用相同的服务间令牌；模型密钥优先从 Web 的「设置 → 模型服务」按租户配置，保存后两个服务立即生效。环境变量只作为开发回退：

```bash
# 可选：不使用页面配置时设置
export OPENROUTER_API_KEY=你的密钥
export AGENT_SERVICE_TOKEN=至少16位的随机服务间密钥
export RAG_AUTH_SECRET=至少32位的随机认证密钥
export REDIS_PASSWORD=至少16位的随机Redis密码
export RAG_POSTGRES_DSN='postgresql://rag:经过URL编码的密码@localhost:55432/rag?sslmode=disable'
export AGENT_POSTGRES_DSN="$RAG_POSTGRES_DSN"
```

认证密钥没有开发默认值，缺失或少于 32 位时 API 会拒绝启动。页面保存的模型密钥使用认证密钥派生出的加密密钥保护，因此更换 `RAG_AUTH_SECRET` 前需要重新配置模型密钥。Compose 使用 `.env` 中的完整 `POSTGRES_DSN`，数据库密码包含特殊字符时必须先进行 URL 编码，不能把原始密码直接拼接进连接地址。

检索与向量索引使用相同的向量维度。更换向量模型时，需要同步确认配置维度、Milvus Collection 和已有文档索引，避免新旧向量不兼容。

## 本地开发

### 1. 启动开发依赖

```bash
docker compose up -d postgres redis nats minio etcd milvus prometheus otel-collector jaeger grafana
```

基础服务包括 PostgreSQL、Redis、NATS JetStream、MinIO、Etcd、Milvus 和可观测组件。

### 2. 启动后端

```bash
cd api
go run .
```

后端默认监听 `http://localhost:9999`。API 和 Worker 共用进程与连接池，Worker 通过 JetStream 消费解析、切块、向量化和删除任务；问答、检索和上传接口通过 Redis 共享限流状态。

`GET /healthz` 只表示进程存活；`GET /readyz` 会检查 PostgreSQL、Redis、JetStream、MinIO、Milvus 和 Agent，部署平台应使用 `/readyz` 作为就绪探针。

### 3. 启动 Agent

```bash
cd agent
uv sync --locked --extra dev
uv run fastapi dev
```

Agent 默认监听 `http://localhost:8000`。API 与 Agent 必须使用相同的 `AGENT_SERVICE_TOKEN`。

`GET /health` 是存活探针，`GET /ready` 会验证 LangGraph Checkpointer 的 PostgreSQL 连接。Compose 中 Agent 仅暴露在内部服务网络，不发布宿主机端口。

### 4. 启动前端

```bash
cd web
pnpm install
pnpm dev
```

默认访问 `http://localhost:5173`。

## 数据与索引

数据库迁移位于 `api/internal/infrastructure/postgres/migrations/`，一个业务表对应一个迁移文件。迁移按编号顺序执行，已经投入使用的迁移不回写修改；表结构变化通过新增迁移表达。

文档入库会同时产生三类数据：

```text
MinIO 原始文件
PostgreSQL 文档、知识片段与任务记录
Milvus 知识片段向量
```

涉及文档删除、重新索引或知识库删除时，必须同步考虑三类数据的一致性。

## 文档处理扩展

文档解析器位于 `api/internal/infrastructure/parser/`，不同文件格式保持独立实现。新增格式时需要同时完成：

1. 文件类型识别与上传校验
2. 解析器实现
3. 标题、页码、表格等元数据输出
4. 错误信息转换与多语言展示
5. 正常文件、空文件、损坏文件和特殊结构测试

解析结果统一进入切块流程，不应在格式解析器中重复实现索引或检索逻辑。

## Agent 工具扩展

Agent 运行时位于 `agent/src/enterprise_agent/`。FastAPI 使用 `APIRouter` 拆分接口，LangGraph 节点通过 Go-zero 暴露的内部工具端点获取知识数据。新增能力时：

1. 在 `graph.py` 中增加边界清晰的节点和显式状态字段。
2. 工具数据通过 `backend_client.py` 调用内部 API，不直接访问 Go 领域仓储。
3. 使用 Pydantic 模型约束请求、响应和状态，不接受未知业务字段。
4. 设置合理超时，限制输入、上下文和引用数量。
5. 补充路由分支、失败、超时、流式事件和 Checkpointer 测试。

Agent 使用 LangGraph 显式执行路由、检索、联网补充、结构化证据评估、有限改写、答案合成和引用校验。`tenant_id:run_id` 作为 checkpoint thread，证据不足或引用校验失败会进入明确拒答。

## 生产指标

启用 `Metrics.Enabled` 后，后端通过 `Metrics.Path`（默认 `/metrics`）提供 Prometheus 文本指标。指标覆盖 HTTP 请求、Agent 执行与状态转移、工具调用、模型调用、Token 与费用、检索候选与返回数量、文档 Worker 任务结果和各阶段耗时。

```bash
curl http://localhost:9999/metrics
```

指标标签必须保持低基数，禁止加入用户、知识库、文档、问题、会话和模型 ID。`enterprise_rag_model_tokens_total` 按输入、输出和总 Token 记录 Provider 返回的真实用量；`enterprise_rag_model_cost_usd_total` 记录 Provider 返回的美元费用，不根据字符数或静态价格表推算。生产环境由 Prometheus 定时抓取该端点，并基于失败率、P95/P99 延迟、Agent 迭代数、Worker 重试率和模型预算配置告警。

仓库中的 `deploy/monitoring/alerts.yaml` 提供 HTTP、Agent、模型、检索和 Worker 的基础告警规则。阈值是运行起点，部署后应根据真实请求量、模型 SLA 和文档规模调整，不能直接把本地测试耗时当作生产基线。

Docker Compose 同时启动以下观测组件：

| 组件 | 地址 | 职责 |
| --- | --- | --- |
| Grafana | `http://localhost:3000` | 展示预置的 Enterprise RAG 运行总览 |
| Jaeger | `http://localhost:16686` | 查询单次请求的分布式调用链 |
| Prometheus | `http://localhost:9090` | 存储、查询指标并执行告警规则 |
| OpenTelemetry Collector | `localhost:4317` / `localhost:4318` | 接收 OTLP 数据并转发到追踪后端 |

`deploy/monitoring/prometheus.yaml` 通过 Compose 服务名抓取 API 和 Agent 的 `/metrics`，Targets 页面应显示 `enterprise-rag` 与 `enterprise-rag-agent` 为 `UP`。Grafana 自动配置 Prometheus、Jaeger 数据源及运行总览，无需在页面中手工添加数据源。

文档处理任务使用 PostgreSQL Outbox 与业务状态同事务落库。发布器把待发布事件投递到 JetStream 并指数退避；Worker 在解析、分块、向量和状态写入成功后 ACK，永久失败或非法消息写入独立 `DOCUMENT_TASKS_DLQ`。文档和 chunk 同时记录内容哈希、文档版本、分块策略版本及 Embedding Provider、模型和维度，避免模型或策略切换后混用索引。

后端通过 go-zero 的 HTTP Trace 中间件建立请求根 Span，并在 Agent 规划、工具执行、回答生成、检索改写、Embedding、Milvus 检索、关键词召回、重排和文档 Worker 阶段建立子 Span。文档任务发布时把 W3C `traceparent` 写入 JetStream 消息头，Worker 消费后恢复上下文，因此上传、解析、切块和向量化能够在 Jaeger 中形成同一条调用链。业务 JSON 不保存 Trace ID，追踪属性也禁止写入问题正文、用户、知识库和文档内容。Jaeger 使用内存存储，适合本地开发；生产部署应改用持久化后端并调整采样率。

当前告警规则只在 Prometheus 中展示状态；如需邮件、企业微信或钉钉通知，还需要部署 Alertmanager 并配置通知接收端。

## 状态一致性

Agent 使用 LangGraph 约束单次请求的路由、工具执行、结果评估、检索改写和生成过程，并通过 PostgreSQL Checkpointer 按 `run_id` 持久化执行状态。文档与索引链路遵循以下完成条件：

- 发布方收到 JetStream 服务端确认后，任务才视为已进入队列。
- Worker 先以数据库租约原子认领任务，再执行带截止时间的阶段处理。
- 下游任务使用确定性 ID；任务记录创建和消息发布都可安全重试。
- 当前任务状态和下游消息均落地后才 ACK；临时错误更新重试状态后 NAK，永久错误持久化失败终态后 ACK。
- 重复、迟到或租约过期后的消息只会恢复未完成阶段，不能覆盖更晚的文档状态。

## 超时、取消与资源生命周期

- HTTP 请求、Agent 调用、内部工具、模型请求、Redis 操作和每个 Worker 任务都使用有界超时并传递取消信号。
- SSE 客户端断开后取消上游流；心跳协程在 Handler 返回前退出，不遗留后台发送任务。
- Worker 停机时先停止接收新消息，再等待在途任务；超过停机预算后取消任务，未 ACK 消息由 JetStream 重新投递。
- PostgreSQL、Redis、NATS JetStream 和 HTTP 客户端由服务生命周期统一创建与关闭；PostgreSQL 连接池设置连接数、存活时间、空闲时间和健康检查周期。
- 自动重试不创建脱离请求生命周期的 Goroutine，延迟重投由 JetStream 管理。

## 检索与回答扩展

检索能力位于 `api/internal/service/retrieval/`，答案生成和引用筛选位于 `agent/src/enterprise_agent/`。调整检索策略时，应分别观察：

- 目标知识是否进入候选结果
- 重排后是否保留有效片段
- 最终引用是否真正支撑回答
- 无关问题是否触发正确兜底
- 延迟和模型调用次数是否可接受

新的问题能力应优先实现为工具，不应在 Chat Logic 中增加固定关键词路由。新增 Agent 事件时需要同步补充 SSE 解析、会话持久化和多语言文案。

## 测试与检查

API：

```bash
cd api
go test ./...
go vet ./...
go build ./...
```

Agent：

```bash
cd agent
uv sync --locked --extra dev
.venv/bin/ruff check src tests
.venv/bin/mypy src
.venv/bin/pytest
.venv/bin/fastapi dev
```

前端：

```bash
cd web
pnpm lint
pnpm build
```

统一门禁与跨服务端到端测试：

```bash
make verify
make verify-compose
make test-e2e
```

端到端测试使用独立 Compose 项目和一次性数据卷，覆盖注册、知识库创建、文档上传、JetStream 异步索引、检索、Agent 编排、模型生成、引用返回和 Redis 限流；测试完成后自动清理，不访问真实模型服务。

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
