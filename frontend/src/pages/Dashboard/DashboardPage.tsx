import { Space, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { listDocuments } from '../../api/documents'
import { listSubjects } from '../../api/subjects'

export function DashboardPage() {
  const [subjectTotal, setSubjectTotal] = useState(0)
  const [documentTotal, setDocumentTotal] = useState(0)

  useEffect(() => {
    async function loadMetrics() {
      const [subjects, documents] = await Promise.all([
        listSubjects({ page: 1, page_size: 1 }),
        listDocuments({ page: 1, page_size: 1 }),
      ])
      setSubjectTotal(subjects.total)
      setDocumentTotal(documents.total)
    }

    void loadMetrics()
  }, [])

  const metrics = [
    { label: '知识库', value: String(subjectTotal) },
    { label: '文档', value: String(documentTotal) },
    { label: '索引任务', value: '0' },
    { label: '会话', value: '0' },
  ]

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
          可以先创建知识库，再上传文档。文档上传后会进入 uploaded 状态，后续接入解析和向量化任务。
        </Typography.Paragraph>
      </div>
    </div>
  )
}
