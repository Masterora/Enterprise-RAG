import { Typography } from 'antd'
import { useEffect, useState } from 'react'
import { listDocuments } from '../../api/documents'
import { listSubjects } from '../../api/subjects'
import { useI18n } from '../../useI18n'

export function DashboardPage() {
  const [subjectTotal, setSubjectTotal] = useState(0)
  const [documentTotal, setDocumentTotal] = useState(0)
  const [indexedDocumentTotal, setIndexedDocumentTotal] = useState(0)
  const [processingDocumentTotal, setProcessingDocumentTotal] = useState(0)
  const { t } = useI18n()

  useEffect(() => {
    async function loadMetrics() {
      const [subjects, documents, indexedDocuments, processingDocuments] = await Promise.all([
        listSubjects({ page: 1, page_size: 1 }),
        listDocuments({ page: 1, page_size: 1 }),
        listDocuments({ status: 'indexed', page: 1, page_size: 1 }),
        listDocuments({ status: 'processing', page: 1, page_size: 1 }),
      ])
      setSubjectTotal(subjects.total)
      setDocumentTotal(documents.total)
      setIndexedDocumentTotal(indexedDocuments.total)
      setProcessingDocumentTotal(processingDocuments.total)
    }

    void loadMetrics()
  }, [])

  const metrics = [
    { label: t('dashboard.metric.subjects'), value: String(subjectTotal) },
    { label: t('dashboard.metric.documents'), value: String(documentTotal) },
    { label: t('dashboard.metric.indexedDocuments'), value: String(indexedDocumentTotal) },
    { label: t('dashboard.metric.processingDocuments'), value: String(processingDocumentTotal) },
  ]

  return (
    <div className="dashboard-page">
      <div className="metric-grid">
        {metrics.map((item) => (
          <div className="metric-card" key={item.label}>
            <div className="metric-label">{item.label}</div>
            <div className="metric-value">{item.value}</div>
          </div>
        ))}
      </div>

      <div className="status-panel">
        <Typography.Title level={4}>{t('dashboard.overviewTitle')}</Typography.Title>
        <Typography.Paragraph>{t('dashboard.overviewText')}</Typography.Paragraph>
      </div>
    </div>
  )
}
