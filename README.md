<div align="center">
  <img src="./web/public/enterprise-rag-icon.svg" width="72" alt="Enterprise-RAG" />
  <h1>Enterprise-RAG <sub>v5.0.0</sub></h1>
  <p>面向企业知识的 Agentic RAG 系统</p>
  <p><strong>受控编排 · 知识工具 · 可信引用 · 全程可观测</strong></p>
</div>

Enterprise-RAG 帮助团队把分散的制度、规范、手册和技术资料整理为统一知识库。用户可以直接围绕内部资料提问，查看回答依据并核查原文；资料不足时，系统会明确拒答而不是补全未经证实的内容。

## 能解决什么

| 统一管理企业资料 | 快速获取知识答案 | 核查回答依据 | 了解系统运行状态 |
| :--- | :--- | :--- | :--- |
| 按租户和知识库管理文档、版本与索引状态 | 根据问题选择知识检索、知识概览、文档定位或联网补充 | 展示文档、章节、页码和证据原文 | 查看处理进度、失败原因、调用耗时和检索效果 |

## 使用体验

| 使用阶段 | 用户操作 | 系统反馈 |
| :--- | :--- | :--- |
| 准备知识 | 创建知识库，批量上传 Markdown、TXT、PDF、DOC、DOCX 或 PPTX | 展示解析、切块、向量化和索引状态 |
| 开始问答 | 选择知识库和模型后提问 | 流式展示执行进度并生成回答 |
| 核查结论 | 展开回答中的引用 | 定位来源文档、章节、页码与原文片段 |
| 处理异常 | 取消或恢复运行，查看失败任务并重试 | 保留运行事件、失败原因和恢复依据 |

```text
创建知识库 → 上传文档 → 等待索引完成 → 进入助手 → 提问 → 阅读回答并核查引用
```

## 系统架构

![Enterprise-RAG 系统架构](./docs/system-architecture.svg)

浏览器只访问 Go-zero 服务。Go-zero 负责业务、安全和运行控制；FastAPI / LangGraph 在授权范围内完成 Agent 决策，不能自行扩大租户或知识库权限。

| 业务与安全网关 | Agent 编排 |
| :--- | :--- |
| 账号认证、租户与知识权限、Run 管理、SSE、内部 Tool Gateway、检索和异步文档任务 | 路由规划、工具选择、证据评估、有限查询改写、答案生成、引用校验和拒答 |

```text
在线问答：React → Go-zero → Agent ⇄ Tool Gateway → 检索 / 模型 → 回答与引用
文档索引：上传 → 业务状态 + Outbox → JetStream → Worker → 解析 / 向量化 → Milvus
```

## 核心功能

| 知识管理 | 智能助手 | 日志与评估 |
| :--- | :--- | :--- |
| 知识库创建、编辑与删除 | 流式回答与 Agent 执行步骤 | 文档处理任务和失败原因 |
| 多文件上传、筛选与删除 | 会话保存、改名与删除 | 处理阶段与耗时统计 |
| 租户与知识库双层隔离 | 对话模型切换 | 失败任务重试与 DLQ 状态 |
| 重复文件校验与版本记录 | Run 取消、恢复与事件补拉 | 单问题 Recall@K 评估 |
| 索引模型与分块策略追踪 | 引用展开、原文核查与拒答 | API、Agent、模型和 Worker 指标 |
| 原文件、业务记录和向量联动删除 | 可选联网资料补充 | Outbox、工具调用和模型成本观测 |

## 工程保障

| 安全隔离 | 性能治理 | 可靠恢复 | 可观测性 |
| :--- | :--- | :--- | :--- |
| `tenant_id` 贯穿 PostgreSQL、Milvus、MinIO、Redis、内部工具和 checkpoint | SSE 流式响应、在线与离线链路分离、混合检索、分布式限流和并发控制 | Outbox、显式 ACK、任务租约、幂等 Upsert、DLQ、Run 恢复和事件补拉 | OpenTelemetry、Prometheus、Grafana、Jaeger、Token 与模型成本指标 |

- Run Controller 负责超时、取消、并发和资源生命周期；PostgreSQL Checkpointer 只负责保存与恢复 LangGraph 状态。
- 检索结果携带证据 ID、文档版本和内容哈希。Agent 只能引用本次运行获得的证据，校验失败时会修正或拒答。
- Chat 与 Embedding 通过 Provider 配置接入。索引记录 Provider、模型和向量维度，避免不兼容的向量混用。

## 项目结构

```text
Enterprise-RAG/
├── api/          Go-zero API、领域服务与异步 Worker
├── agent/        FastAPI、LangGraph 编排与状态持久化
├── web/          React Web 应用
├── deploy/       监控与可观测配置
├── docs/         开发、运行时与质量验证文档
└── tests/        端到端测试
```

## 快速开始

### 环境要求

| Docker | Go | Python / uv | Node.js | pnpm |
| :--- | :--- | :--- | :--- | :--- |
| Docker Compose | `1.26.3+` | `3.12+` / `0.11+` | `20.19+` 或 `22.12+` | `9+` |

### 容器化启动

```bash
cp .env.example .env
# 编辑 .env，设置服务令牌、认证密钥及存储密码
docker compose up -d
```

Compose 会启动 Web、Go-zero、FastAPI / LangGraph、PostgreSQL、Redis、NATS JetStream、MinIO、Etcd、Milvus，以及 Prometheus、Grafana、OpenTelemetry Collector 和 Jaeger。

启动后访问 `http://localhost:8080`，在「设置 → 模型服务」中粘贴 OpenRouter API Key 并保存。系统会先验证可用性，再按租户加密保存；API 与 Agent 立即生效，不需要修改环境变量或重启容器。

### 必需配置

| 配置 | 说明 |
| :--- | :--- |
| `AGENT_SERVICE_TOKEN` | API 与 Agent 共用的服务间令牌，至少 16 位随机字符 |
| `RAG_AUTH_SECRET` | 用户令牌签名密钥，至少 32 位随机字符 |
| `POSTGRES_PASSWORD` / `POSTGRES_DSN` | PostgreSQL 密码和连接地址；特殊字符需要 URL 编码 |
| `REDIS_PASSWORD` | Redis 密码，至少 16 位 |
| `MINIO_ROOT_PASSWORD` | MinIO 管理密码 |

`OPENROUTER_API_KEY` 仅作为可选的服务端回退配置。正式部署前必须替换 `.env` 中的服务令牌、认证密钥与存储密码，并为数据库、对象存储和观测后端配置持久化方案。

### 服务入口

| 服务 | 默认地址 | 用途 |
| :--- | :--- | :--- |
| Web | `http://localhost:8080` | 使用知识库和智能助手 |
| Grafana | `http://localhost:3000` | 查看运行总览、失败率和链路耗时 |
| Jaeger | `http://localhost:16686` | 定位 Agent、检索、模型和工具调用耗时 |
| Prometheus | `http://localhost:9090` | 查询原始指标和告警状态 |

本地源码调试和单服务启动方式见[开发指南](./docs/development.md)。

## 质量验证

```bash
make verify
make verify-compose
make verify-race
make test-e2e
make evaluate CASES=/absolute/path/to/cases.json
```

`make verify` 检查 API、Agent 和 Web。端到端测试使用隔离环境覆盖上传、Outbox 发布、索引、检索、问答引用、Run 取消与恢复以及分布式限流，不消耗外部模型额度。

批量评估通过正式 API 统计 Recall@K、路由正确率、回答结果、引用数量和端到端延迟，并输出 Markdown 报告。

## 使用边界

- 扫描图片、复杂图表、加密文件和低质量文档可能影响解析精度。
- 联网资料只作为知识库之外的补充，不能替代企业内部正式依据。
- 单题 Recall@K 用于检查指定问题的召回结果，不代表所有问题都能得到正确回答。
- 法律、医疗、财务和生产控制等高风险结论仍需专业人员确认。

## 项目文档

- [开发指南](./docs/development.md)：环境、配置、代码分层和测试方法。
- [Agent 运行时设计](./docs/agent-runtime.md)：规划、工具、安全边界和恢复机制。
- [质量验证](./docs/quality-evaluation.md)：评估指标、验证方法和报告格式。
