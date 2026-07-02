import { Empty, Typography } from 'antd'

export function DocumentsPage() {
  return (
    <div className="dashboard-page">
      <h1 className="page-title">文档</h1>
      <p className="page-subtitle">上传文档、查看解析状态并管理索引任务。</p>
      <div className="status-panel">
        <Typography.Title level={4}>文档模块</Typography.Title>
        <Empty description="暂无文档数据" />
      </div>
    </div>
  )
}
