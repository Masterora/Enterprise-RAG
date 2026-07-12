import { Alert, Button, Input, Modal, Select, Space, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { listDocuments, type DocumentInfo } from '../../../api/documents'
import { searchRetrieval, type RetrievalMetrics } from '../../../api/retrieval'
import { useI18n } from '../../../useI18n'

type Props = {
  open: boolean
  subjectID: string
  onClose: () => void
}

export function RetrievalEvaluationModal({ open, subjectID, onClose }: Props) {
  const { t } = useI18n()
  const [documents, setDocuments] = useState<DocumentInfo[]>([])
  const [query, setQuery] = useState('')
  const [expectedDocIDs, setExpectedDocIDs] = useState<string[]>([])
  const [metrics, setMetrics] = useState<RetrievalMetrics | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open || !subjectID) {
      return
    }
    setMetrics(null)
    void listDocuments({ subject_id: subjectID, status: 'indexed', page: 1, page_size: 100 })
      .then((result) => setDocuments(result.list))
      .catch(() => message.error(t('chat.evaluation.loadFailed')))
  }, [open, subjectID, t])

  async function evaluate() {
    if (!query.trim() || expectedDocIDs.length === 0) {
      message.warning(t('chat.evaluation.required'))
      return
    }
    setLoading(true)
    try {
      const result = await searchRetrieval({
        subject_id: subjectID,
        query: query.trim(),
        top_k: 5,
        expected_doc_ids: expectedDocIDs,
      })
      setMetrics(result.metrics)
    } catch {
      message.error(t('chat.evaluation.failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      open={open}
      title={t('chat.evaluation.title')}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>{t('common.cancel')}</Button>,
        <Button key="run" type="primary" loading={loading} onClick={() => void evaluate()}>
          {t('chat.evaluation.run')}
        </Button>,
      ]}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Typography.Text type="secondary">{t('chat.evaluation.description')}</Typography.Text>
        <Input.TextArea
          value={query}
          autoSize={{ minRows: 2, maxRows: 5 }}
          placeholder={t('chat.evaluation.query')}
          onChange={(event) => setQuery(event.target.value)}
        />
        <Select
          mode="multiple"
          value={expectedDocIDs}
          options={documents.map((document) => ({ label: document.filename, value: document.id }))}
          placeholder={t('chat.evaluation.expected')}
          onChange={setExpectedDocIDs}
          style={{ width: '100%' }}
        />
        {metrics && (
          <Alert
            type={metrics.recall_at_k >= 0.8 ? 'success' : 'warning'}
            showIcon
            message={t('chat.evaluation.result', {
              k: metrics.top_k,
              hits: metrics.recall_hit_count,
              expected: metrics.expected_count,
              recall: `${(metrics.recall_at_k * 100).toFixed(1)}%`,
            })}
            description={t('chat.evaluation.candidates', {
              candidates: metrics.candidate_count,
              returned: metrics.returned_count,
            })}
          />
        )}
      </Space>
    </Modal>
  )
}
