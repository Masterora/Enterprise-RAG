import { Button, Form, Pagination, Select, Space, Table, Tooltip, Typography, Upload, message, type UploadFile } from 'antd'
import { ClearOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons'
import { useCallback, useEffect, useState } from 'react'
import { clearFailedDocuments, listDocuments, uploadDocument, type DocumentInfo } from '../../api/documents'
import { listSubjects, type SubjectInfo } from '../../api/subjects'
import { useI18n } from '../../useI18n'

export function DocumentsPage() {
  const [form] = Form.useForm()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [documents, setDocuments] = useState<DocumentInfo[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [selectedSubjectId, setSelectedSubjectId] = useState<string>()
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [clearingFailed, setClearingFailed] = useState(false)
  const { t } = useI18n()

  const loadPageData = useCallback(async (nextPage = page, nextSubjectId = selectedSubjectId) => {
    setLoading(true)
    try {
      const [subjectData, documentData] = await Promise.all([
        listSubjects({ page: 1, page_size: 100 }),
        listDocuments({ page: nextPage, page_size: 10, subject_id: nextSubjectId }),
      ])
      if (nextSubjectId && !subjectData.list.some((subject) => subject.id === nextSubjectId)) {
        setSelectedSubjectId(undefined)
      }
      setSubjects(subjectData.list)
      setDocuments(documentData.list)
      setTotal(documentData.total)
      setPage(nextPage)
    } catch {
      message.error(t('documents.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [page, selectedSubjectId, t])

  useEffect(() => {
    void loadPageData()
  }, [loadPageData])

  async function handleUpload(values: { subject_id: string; file?: UploadFile[] }) {
    const files =
      values.file
        ?.map((item) => item.originFileObj)
        .filter((item): item is NonNullable<typeof item> => Boolean(item)) ?? []
    if (files.length === 0) {
      message.warning(t('documents.pickFileWarning'))
      return
    }

    setUploading(true)
    try {
      for (const file of files) {
        await uploadDocument({ subjectId: values.subject_id, file })
      }
      form.resetFields()
      message.success(
        files.length === 1
          ? t('documents.uploadSuccess')
          : t('documents.uploadBatchSuccess', { count: files.length }),
      )
      await loadPageData(1)
    } catch {
      message.error(t('documents.uploadFailed'))
    } finally {
      setUploading(false)
    }
  }

  function renderDocumentStatus(status: string) {
    if (status === 'uploaded') {
      return t('documents.status.uploaded')
    }
    if (status === 'indexed') {
      return t('documents.status.indexed')
    }
    if (status === 'failed') {
      return t('documents.status.failed')
    }
    if (status === 'processing') {
      return t('documents.status.processing')
    }
    return status
  }

  function formatFileSize(size: number) {
    if (!Number.isFinite(size) || size <= 0) {
      return '0 B'
    }
    if (size < 1024) {
      return `${size} B`
    }
    if (size < 1024 * 1024) {
      return `${(size / 1024).toFixed(1)} KB`
    }
    if (size < 1024 * 1024 * 1024) {
      return `${(size / (1024 * 1024)).toFixed(1)} MB`
    }
    return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`
  }

  function formatDateTime(value: string) {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
      return value
    }
    return new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(date)
  }

  const failedCount = documents.filter((item) => item.status === 'failed').length

  async function handleClearFailed() {
    setClearingFailed(true)
    try {
      const deleted = await clearFailedDocuments()
      if (deleted > 0) {
        message.success(t('documents.clearFailedSuccess', { count: deleted }))
      } else {
        message.info(t('documents.clearFailedEmpty'))
      }
      await loadPageData(page)
    } catch {
      message.error(t('documents.clearFailedFailed'))
    } finally {
      setClearingFailed(false)
    }
  }

  return (
    <div className="dashboard-page">
      <div className="status-panel">
        <Typography.Title level={4}>{t('documents.uploadTitle')}</Typography.Title>
        <Form form={form} layout="vertical" onFinish={handleUpload}>
          <Form.Item
            label={t('documents.subject')}
            name="subject_id"
            rules={[{ required: true, message: t('documents.subjectRequired') }]}
          >
            <Select
              placeholder={t('documents.selectSubject')}
              options={subjects.map((subject) => ({ label: subject.name, value: subject.id }))}
            />
          </Form.Item>
          <Form.Item
            label={t('documents.file')}
            name="file"
            rules={[{ required: true, message: t('documents.fileRequired') }]}
            valuePropName="fileList"
            getValueFromEvent={(event) => (Array.isArray(event) ? event : event?.fileList)}
          >
            <Upload beforeUpload={() => false} multiple>
              <Button icon={<UploadOutlined />}>{t('documents.chooseFile')}</Button>
            </Upload>
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={uploading}>
            {t('documents.upload')}
          </Button>
        </Form>
      </div>

      <div className="status-panel">
        <div className="panel-heading">
          <Typography.Title level={4}>{t('documents.listTitle')}</Typography.Title>
          <Space size={8}>
            <Select
              allowClear
              value={selectedSubjectId}
              placeholder={t('documents.filterSubject')}
              style={{ minWidth: 220 }}
              options={subjects.map((subject) => ({ label: subject.name, value: subject.id }))}
              onChange={(value) => {
                setSelectedSubjectId(value)
                void loadPageData(1, value)
              }}
            />
            <Button
              icon={<ReloadOutlined />}
              aria-label={t('common.refresh')}
              onClick={() => void loadPageData(page)}
              loading={loading}
            />
            <Tooltip title={t('common.clearFailed')}>
              <Button
                icon={<ClearOutlined />}
                aria-label={t('common.clearFailed')}
                onClick={() => void handleClearFailed()}
                loading={clearingFailed}
                disabled={failedCount === 0}
              />
            </Tooltip>
          </Space>
        </div>
        <div className="fixed-table-shell">
          <div className="fixed-table-body">
            <Table
              rowKey="id"
              loading={loading}
              dataSource={documents}
              pagination={false}
              columns={[
                { title: t('documents.fileName'), dataIndex: 'filename' },
                { title: t('documents.fileType'), dataIndex: 'file_type' },
                {
                  title: t('documents.fileSize'),
                  dataIndex: 'file_size',
                  render: (_, record) => formatFileSize(record.file_size),
                },
                {
                  title: t('documents.status'),
                  dataIndex: 'status',
                  render: (_, record) => renderDocumentStatus(record.status),
                },
                {
                  title: t('documents.createdAt'),
                  dataIndex: 'created_at',
                  render: (_, record) => formatDateTime(record.created_at),
                },
              ]}
            />
          </div>
          <Pagination
            className="fixed-table-pagination"
            current={page}
            pageSize={10}
            total={total}
            onChange={(nextPage) => void loadPageData(nextPage)}
          />
        </div>
      </div>
    </div>
  )
}
