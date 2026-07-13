# Enterprise-RAG 开发指南

本文面向项目开发者，集中说明技术选型、代码结构、配置关系、开发运行和扩展约定。产品定位与系统能力请先阅读 [项目说明](../README.md)。

## 技术架构

| 区域 | 技术 | 主要职责 |
| --- | --- | --- |
| Web | React、TypeScript、Vite、Ant Design | 页面交互、流式回答、国际化与状态展示 |
| API | Go、Go-zero | 接口接入、鉴权、参数校验与业务编排 |
| Agent | Go | 任务规划、工具注册、并行执行、边界控制与观察汇总 |
| 业务数据 | PostgreSQL | 用户、知识库、文档、知识片段、会话与任务日志 |
| 向量检索 | Milvus | 文档向量存储与相似度召回 |
| 文件存储 | MinIO | 原始上传文件保存 |
| 异步任务 | NATS Queue Group | 解析、切分、向量化和删除任务流转 |
| 模型服务 | Provider 接口 + OpenRouter | 对话生成、向量化、联网搜索、模型切换与失败重试 |

## 目录结构

```text
Enterprise-RAG/
├── backend/
│   ├── api/                         按业务模块拆分的接口定义
│   ├── etc/                         服务、基础设施与应用配置
│   ├── rag.go                       API 与 Worker 进程入口
│   └── internal/
│       ├── handler/                 Go-zero 生成的请求入口
│       ├── logic/                   接口业务编排
│       ├── service/                 可复用的领域服务
│       │   ├── agent/               Agent 运行时、规划器、注册表与工具
│       │   ├── retrieval/           混合检索、重排、裁剪与单题评估
│       │   └── chatflow/            答案规范化、引用与持久化辅助
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
Handler  →  Logic  →  Agent Runtime  →  Tool  →  Service / Infrastructure
```

- `handler` 由 Go-zero 生成，只负责接收请求，不手工加入业务逻辑。
- `logic` 负责权限校验、参数转换和调用 Agent，不包含工具选择分支。
- `agent` 负责规划、工具校验、并行执行、观察汇总和答案合成。
- `service` 承担可复用的检索、文档处理和任务能力。
- `repository` 隔离 PostgreSQL 数据访问。
- `infrastructure` 隔离模型 Provider、Milvus、MinIO 和文档解析实现。
- `presenter` 统一把领域结果转换为接口响应。

新增功能时优先沿现有边界扩展，避免在 Handler 中写逻辑，也避免 Logic 直接承载复杂算法或外部服务细节。

## API 组织

`backend/api/` 保存接口契约，各业务模块独立维护，由 `rag.api` 聚合并生成统一路由：

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
goctl api format --dir backend/api
```

需要重新生成 Handler、路由或类型时执行：

```bash
cd backend
goctl api go -api api/rag.api -dir .
```

生成的 Handler 保持原样，业务实现放入对应 Logic 或 Service。

## 配置结构

后端入口配置加载应用与基础设施配置：

| 配置文件 | 内容 |
| --- | --- |
| `rag-api.yaml` | API 监听地址与共享配置入口 |
| `infrastructure.yaml` | PostgreSQL、NATS、MinIO 与 Milvus 连接信息 |
| `application.yaml` | Worker、切块、检索、Agent、模型、可靠性、指标和评估参数 |

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

基础服务包括 PostgreSQL、NATS、MinIO、Etcd、Milvus 和 Prometheus。

### 2. 启动后端

```bash
cd backend
go run .
```

后端默认监听 `http://localhost:9999`。API 和 Worker 共用进程与依赖连接，Worker 通过 NATS 并发消费解析、切块、向量化和删除任务。

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

## Agent 工具扩展

Agent 运行时位于 `backend/internal/service/agent/`。工具必须实现统一接口并提供名称、用途和 JSON Schema，由注册表控制可见性。新增工具时：

1. 实现 `Tool` 接口，工具只完成一个边界清晰的能力。
2. 使用 JSON Schema 描述参数，不在规划 Prompt 中写特定知识库规则。
3. 在默认注册表注册工具，并通过 `Agent.EnabledTools` 控制是否开放。
4. 设置合理超时，限制输入和输出规模，不信任文档或网页中的指令。
5. 返回结构化来源和指标，避免让模型从自然语言中反向解析工具结果。
6. 补充参数校验、失败、超时、重复调用和并发测试。

Agent 使用显式状态机执行“规划、工具执行、结果观察”循环。每轮独立工具并行执行，结果按原计划顺序汇总；最大迭代数、单轮及总工具预算、跨轮去重和无有效结果共同控制终止。

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

`deploy/monitoring/prometheus.yaml` 负责抓取宿主机后端的 `/metrics`，Targets 页面应显示 `enterprise-rag` 为 `UP`。Grafana 自动配置 Prometheus、Jaeger 数据源及运行总览，无需在页面中手工添加数据源。

后端通过 go-zero 的 HTTP Trace 中间件建立请求根 Span，并在 Agent 规划、工具执行、回答生成、检索改写、Embedding、Milvus 检索、关键词召回、重排和文档 Worker 阶段建立子 Span。文档任务发布时把 W3C `traceparent` 写入 NATS 消息头，Worker 消费后恢复上下文，因此上传、解析、切块和向量化能够在 Jaeger 中形成同一条调用链。业务 JSON 不保存 Trace ID，追踪属性也禁止写入问题正文、用户、知识库和文档内容。Jaeger 使用内存存储，适合本地开发；生产部署应改用持久化后端并调整采样率。

当前告警规则只在 Prometheus 中展示状态；如需邮件、企业微信或钉钉通知，还需要部署 Alertmanager 并配置通知接收端。

## 状态一致性

Agent 使用内存状态机约束单次请求的规划、执行、观察和生成过程。文档与索引任务使用 PostgreSQL 条件更新约束持久化状态：只有允许的前态才能进入下一状态，自动重试和人工重试使用独立的回退转移。重复或乱序的 NATS 消息无法覆盖更晚状态，删除中的文档也不会被迟到的解析任务恢复。

## 检索与回答扩展

检索能力位于 `backend/internal/service/retrieval/`，回答辅助能力位于 `backend/internal/service/chatflow/`。调整检索策略时，应分别观察：

- 目标知识是否进入候选结果
- 重排后是否保留有效片段
- 最终引用是否真正支撑回答
- 无关问题是否触发正确兜底
- 延迟和模型调用次数是否可接受

新的问题能力应优先实现为工具，不应在 Chat Logic 中增加固定关键词路由。新增 Agent 事件时需要同步补充 SSE 解析、会话持久化和多语言文案。

## 测试与检查

后端：

```bash
cd backend
go test ./...
go test -race ./internal/service/agent ./internal/service/retrieval
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
