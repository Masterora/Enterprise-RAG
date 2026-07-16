# Agent 运行时设计

## 服务边界

FastAPI / LangGraph 是独立的 Agent 服务，负责请求路由、知识工具调用、检索结果评估、有限次数的查询改写和答案生成。Go-zero 负责用户鉴权、知识库权限、文档与会话数据、检索实现和流式协议转换。Agent 不直接访问业务仓储，两项服务通过带 `X-Service-Token` 的内部 HTTP 契约交互。

该边界保证模型编排与业务数据权限分离：LangGraph 只决定进入哪条受控链路，真正的数据访问仍由 Go 服务执行并校验用户与知识库范围。

## 运行链路

```text
请求校验
   |
   v
语义路由 ---- knowledge_search / knowledge_overview / document_navigation
   |
   v
内部工具调用 ---- 服务令牌 / 超时 / 严格请求响应模型
   |
   v
证据评估 ---- 结构化充分性判断 / 有预算时改写 / 不足时拒答
   |
   v
答案生成 ---- 仅使用工具返回资料 / 引用结构与语义校验
   |
   v
结果持久化 ---- 回答 / 引用 / 指标 / 执行步骤
```

启用联网搜索时，普通知识检索链路可以补充公开网页来源。知识概览和文档定位保持独立路径，避免用固定关键词在 Go Chat Logic 中复制路由逻辑。

## LangGraph 状态与持久化

图节点包括 `route`、`retrieve`、`overview`、`navigation`、`web_search`、`evaluate`、`rewrite`、`synthesize`、`validate` 和 `refuse`。条件边只允许以下行为：

- 路由节点选择知识检索、知识概览或文档定位。
- 普通知识检索可按请求配置进入联网补充。
- 证据评估输出 `sufficient`、`rewrite` 或 `refuse` 及覆盖率、权威性、冲突和缺失项。
- 允许改写时最多重新检索配置次数；仍不充分时进入明确拒答。
- 候选答案必须同时通过引用编号和语义支持校验，失败只修正一次，仍失败则拒答。

每次执行使用 `tenant_id:run_id` 作为 LangGraph `thread_id`。正式运行通过 `AsyncPostgresSaver` 将 checkpoint 保存到独立的 `langgraph` schema。业务侧另存 `chat_runs` 和带序号的 `chat_run_events`；checkpoint 用于失败或取消后的图恢复，事件表用于状态查询和断线后按 `after_sequence` 补拉，两者职责不同。默认在成功完成后清理 checkpoint，避免把证据正文长期复制到 checkpoint 表；运行结果和审计事件仍保留。

浏览器只连接 Go API。运行接口全部使用 POST：`/api/chat/runs/detail`、`events`、`cancel` 和 `resume`。取消先持久化 `cancel_requested`，本实例立即取消，其他实例通过短间隔轮询响应；恢复沿用同一租户作用域内的 checkpoint。

## 内部工具契约

| 工具 | Go-zero 职责 | Agent 使用方式 |
| --- | --- | --- |
| `knowledge_search` | 混合召回、重排、阈值过滤和引用片段返回 | 普通知识问答与改写重试 |
| `knowledge_overview` | 汇总知识库文档和代表性内容 | 整库内容概览 |
| `document_navigation` | 定位相关文档、章节和片段 | 文档或主题查找 |
| `web_search` | 不访问内部仓储，由模型 Provider 返回公开来源 | 用户明确启用时补充外部资料 |

内部请求与响应均由 Pydantic 严格校验，未知字段被拒绝。Go 工具端点只接受服务令牌鉴权，不暴露给前端；工具请求同时携带用户和知识库标识，由 Go 领域服务继续执行权限约束。

## 可靠性与安全

- Agent 服务令牌至少 16 位，API 与 Agent 必须配置相同值。
- 问题长度、上下文长度、引用数量和改写次数均有明确上限。
- API、Agent、模型与内部工具分别配置超时；同步调用超时返回 504，客户端断开或流式超时会取消上游图执行。
- 模型输出不能覆盖系统指令，最终答案只能引用本次工具返回的来源；未通过校验的候选答案不会先流给浏览器。
- `tenant_id` 由认证上下文注入并贯穿 PostgreSQL、Milvus、MinIO、Redis、checkpoint 和运行事件，Agent 不能扩大知识库范围。
- 模型和 Embedding 通过 Provider 接口选择 `openrouter` 或 `openai_compatible`；请求不能临时切换到未配置的 Provider。
- 对外错误经过安全转换，日志保留内部原因但不返回密钥和连接信息。
- Agent 的内部 BackendClient 不继承宿主机代理环境，服务间请求不会被本地代理截获。
- 指标标签不包含用户、知识库、文档、问题、会话或模型 ID，避免高基数和内容泄漏。

## 可观测性

Agent 暴露 `/health` 和 `/metrics`。指标覆盖执行结果、耗时、进行中请求、图转移、工具调用、模型调用、Token 和费用。OpenTelemetry 将 FastAPI、HTTPX 和图节点调用接入统一 Trace，Go-zero 继续负责入口请求和 JetStream 文档任务链路。

前端只展示面向用户的任务步骤、耗时、状态和结果，不展示 LangGraph 节点名称或内部状态流。框架级细节通过日志、指标和 Trace 供开发与运维定位。

## 测试策略

单元测试覆盖图路由、结果不足后的改写、流式事件、超时取消、接口鉴权和 PostgreSQL Checkpointer。仓库级端到端测试使用隔离 Compose 项目，真实启动 Go-zero、FastAPI / LangGraph、PostgreSQL、Redis、NATS JetStream、MinIO 和 Milvus，并注入仅供测试的 OpenRouter 协议桩，验证完整链路而不消耗外部模型额度。

```bash
make verify
make verify-compose
make test-e2e
```

测试协议桩只存在于 `tests/e2e/`，不会进入生产镜像或运行配置。生产使用租户在模型服务页面保存的 OpenRouter 密钥；环境变量仅作为回退配置。
