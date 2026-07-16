# 质量验证

项目质量分为代码质量、检索质量、回答质量和运行质量。编译成功只能证明代码可构建，不能证明知识能够被正确召回或回答。

## 代码质量门禁

在项目根目录运行：

```bash
make verify
make verify-race
```

`make verify` 依次执行后端单元测试、静态检查、构建，以及前端代码检查和生产构建。`make verify-race` 对 Agent、检索和 Worker 并发路径执行 Go Race Detector。

## 批量效果评估

`api/cmd/evaluate` 通过正式 API 执行测试，不绕过鉴权、检索服务或模型适配层。评估集由外部 JSON 文件提供，仓库不内置特定知识库、文档名称或固定问题。

评估文件结构：

```json
{
  "name": "知识库回归测试",
  "mode": "retrieval",
  "subject_id": "知识库 ID",
  "top_k": 5,
  "cases": [
    {
      "name": "自然语言问题",
      "query": "待评估的问题",
      "expected_doc_ids": ["应当命中的文档 ID"],
      "expected_chunk_ids": [],
      "expected_route": "rag"
    }
  ]
}
```

`mode` 支持：

- `retrieval`：调用 `/api/retrieval/search`，适合快速调整 TopK、候选倍数、改写和重排配置。
- `answer`：调用 `/api/chat/ask`，除召回和路由外，还验证是否回答、是否正确拒答以及最终引用数量。该模式可以在顶层配置 `llm_provider` 和 `llm_model`，并在用例中配置 `expected_outcome` 为 `answered` 或 `no_answer`。

先从浏览器登录态或登录接口取得测试账号 Token，再运行：

```bash
export RAG_API_TOKEN="测试账号 Token"
make evaluate CASES=/absolute/path/to/cases.json \
  ARGS='-concurrency 2 -output ../artifacts/retrieval-report.md'
```

完整回答会产生模型费用，默认并发为 `1`。提高并发前应确认 Provider 限流和预算。命令在 API 错误或任一用例未通过配置阈值时返回非零状态，可直接接入 CI。

## 指标解释

| 指标 | 证明内容 | 不能证明的内容 |
| --- | --- | --- |
| Recall@K | 标注目标是否进入最终检索结果 | 回答是否完全正确 |
| 路由正确率 | Agent 是否选择预期处理路径 | 工具返回内容是否可靠 |
| 回答结果正确率 | 应回答与应拒答是否符合标注 | 事实、完整性和表达质量 |
| P50/P95/P99 | 测试集端到端延迟分布 | 未压测并发下的系统容量 |
| 引用数量 | 回答是否返回证据 | 引用是否真正支持每条结论 |

正式评估集应覆盖关键词问题、语义改写问题、跨段解释问题、知识库概览、混合意图、长问题、无答案问题和不相关问题。每次调整切块、Embedding、TopK、Query Rewrite、Rerank 或阈值后使用同一测试集回归，才能形成可比较的结果。

## 运行证据

- Prometheus 记录 HTTP、Agent、工具、模型、检索和 Worker 指标。
- Grafana 展示请求趋势、失败率、延迟、Token 和成本。
- Jaeger 验证 HTTP、Agent、模型、检索以及 JetStream Worker 的 Trace 传播。
- 评估报告记录测试集效果，监控指标记录系统运行表现，两者不能互相替代。

简历只应引用固定版本、固定测试集和固定环境下实际生成的结果。文档数量、Chunk 数、并发量和延迟发生变化时，应重新执行评估，不使用历史结果代表当前版本。
