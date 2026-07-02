import { Empty, Typography } from 'antd'

export function ChatPage() {
  return (
    <div className="dashboard-page">
      <h1 className="page-title">问答</h1>
      <p className="page-subtitle">选择知识库并进行带引用来源的 RAG 问答。</p>
      <div className="status-panel">
        <Typography.Title level={4}>问答模块</Typography.Title>
        <Empty description="暂无问答会话" />
      </div>
    </div>
  )
}
