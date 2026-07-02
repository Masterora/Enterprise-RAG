import { Space, Typography } from 'antd'

const metrics = [
  { label: '知识库', value: '0' },
  { label: '文档', value: '0' },
  { label: '索引任务', value: '0' },
  { label: '会话', value: '0' },
]

export function DashboardPage() {
  return (
    <div className="dashboard-page">
      <Space direction="vertical" size={4}>
        <h1 className="page-title">RAG 控制台</h1>
        <p className="page-subtitle">查看知识库、文档索引、任务和会话的整体状态。</p>
      </Space>

      <div className="metric-grid">
        {metrics.map((item) => (
          <div className="metric-card" key={item.label}>
            <div className="metric-label">{item.label}</div>
            <div className="metric-value">{item.value}</div>
          </div>
        ))}
      </div>

      <div className="status-panel">
        <Typography.Title level={4}>系统概览</Typography.Title>
        <Typography.Paragraph>
          当前系统暂无业务数据。创建知识库并上传文档后，这里会展示索引进度和问答使用情况。
        </Typography.Paragraph>
      </div>
    </div>
  )
}
