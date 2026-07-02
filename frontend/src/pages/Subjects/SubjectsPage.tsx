import { Empty, Typography } from 'antd'

export function SubjectsPage() {
  return (
    <div className="dashboard-page">
      <h1 className="page-title">知识库</h1>
      <p className="page-subtitle">创建、维护并进入企业知识库。</p>
      <div className="status-panel">
        <Typography.Title level={4}>知识库模块</Typography.Title>
        <Empty description="暂无知识库数据" />
      </div>
    </div>
  )
}
