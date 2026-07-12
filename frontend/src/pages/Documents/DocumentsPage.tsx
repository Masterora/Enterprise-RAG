import {
  Button,
  Card,
  Drawer,
  Form,
  List,
  Modal,
  Pagination,
  Progress,
  Select,
  Space,
  Table,
  Tabs,
  Tooltip,
  Typography,
  Upload,
  message,
  type UploadFile,
} from 'antd'
import {
  ClearOutlined,
  DeleteOutlined,
  EyeOutlined,
  ExclamationCircleFilled,
  CloseOutlined,
  ReloadOutlined,
  RetweetOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { useCallback, useEffect, useState, type Key } from 'react'
import {
  clearFailedDocuments,
  deleteDocument,
  getDocumentDetail,
  listDocuments,
  retryIndexTask,
  uploadDocument,
  type DocumentDetail,
  type DocumentInfo,
} from '../../api/documents'
import { listSubjects, type SubjectInfo } from '../../api/subjects'
import { useI18n } from '../../useI18n'
import {
	translateErrorMessage as translateDocumentErrorMessage,
	translateTaskMessage,
} from '../../utils/errorMessage'

const uploadConcurrency = 4

async function uploadFiles(subjectId: string, files: File[]) {
  let nextIndex = 0
  let firstError: unknown

  async function uploadNext() {
    while (nextIndex < files.length) {
      const file = files[nextIndex]
      nextIndex += 1
      try {
        await uploadDocument({ subjectId, file })
      } catch (error) {
        firstError ??= error
      }
    }
  }

  const workerCount = Math.min(uploadConcurrency, files.length)
  await Promise.all(Array.from({ length: workerCount }, () => uploadNext()))
  if (firstError) {
    throw firstError
  }
}

type UploadRenamePlan = {
  originalName: string
  nextName: string
}

function renameFile(file: File, nextName: string) {
  return new File([file], nextName, { type: file.type, lastModified: file.lastModified })
}

function splitFileName(filename: string) {
  const dotIndex = filename.lastIndexOf('.')
  if (dotIndex <= 0) {
    return { base: filename, ext: '' }
  }
  return { base: filename.slice(0, dotIndex), ext: filename.slice(dotIndex) }
}

function buildUploadPlan(files: File[], existingNames: string[]) {
  const usedNames = new Set(existingNames.map((name) => name.trim().toLowerCase()).filter(Boolean))
  const renamedFiles: File[] = []
  const renamePlans: UploadRenamePlan[] = []

  for (const file of files) {
    const originalName = file.name.trim()
    let nextName = originalName
    if (usedNames.has(originalName.toLowerCase())) {
      const { base, ext } = splitFileName(originalName)
      let suffix = 1
      do {
        nextName = `${base}(${suffix})${ext}`
        suffix += 1
      } while (usedNames.has(nextName.toLowerCase()))
      renamePlans.push({ originalName, nextName })
    }
    usedNames.add(nextName.toLowerCase())
    renamedFiles.push(nextName === originalName ? file : renameFile(file, nextName))
  }

  return { files: renamedFiles, renamePlans }
}

function documentStageLabel(status: string, t: (key: string) => string) {
  switch (status) {
    case 'uploaded':
      return t('tasks.status.pending')
    case 'parsing':
    case 'chunking':
    case 'embedding':
    case 'deleting':
      return t('tasks.status.running')
    case 'parsed':
    case 'chunked':
      return t('documents.tasks')
    case 'indexed':
      return t('tasks.status.success')
    default:
      return t('tasks.status.failed')
  }
}

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
  const [detail, setDetail] = useState<DocumentDetail>()
  const [detailLoading, setDetailLoading] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([])
  const [batchOperating, setBatchOperating] = useState(false)
  const [uploadModalOpen, setUploadModalOpen] = useState(false)
  const [uploadFilePage, setUploadFilePage] = useState(1)
  const { t } = useI18n()
  const selectedFiles = Form.useWatch('file', form) as UploadFile[] | undefined
  const uploadFilePageSize = 10
  const pagedSelectedFiles = (selectedFiles ?? []).slice(
    (uploadFilePage - 1) * uploadFilePageSize,
    uploadFilePage * uploadFilePageSize,
  )

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
      setSelectedRowKeys([])
    } catch {
      message.error(t('documents.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [page, selectedSubjectId, t])

  useEffect(() => {
    void loadPageData()
  }, [loadPageData])

  useEffect(() => {
    if (!documents.some((item) => item.progress > 0 && item.progress < 100)) {
      return
    }
    const timer = window.setInterval(() => void loadPageData(page), 3000)
    return () => window.clearInterval(timer)
  }, [documents, loadPageData, page])

  useEffect(() => {
    const totalPages = Math.max(1, Math.ceil((selectedFiles?.length ?? 0) / uploadFilePageSize))
    if (uploadFilePage > totalPages) {
      setUploadFilePage(totalPages)
    }
  }, [selectedFiles, uploadFilePage])

  async function handleUpload(values: { subject_id: string; file?: UploadFile[] }) {
    const originalFiles =
      values.file
        ?.map((item) => item.originFileObj)
        .filter((item): item is NonNullable<typeof item> => Boolean(item)) ?? []
    if (originalFiles.length === 0) {
      message.warning(t('documents.pickFileWarning'))
      return
    }

    setUploading(true)
    setUploadModalOpen(false)
    setUploadFilePage(1)
    form.resetFields()
    try {
      const existing = await listDocuments({
        subject_id: values.subject_id,
        page: 1,
        page_size: 1000,
      })
      const { files, renamePlans } = buildUploadPlan(
        originalFiles,
        existing.list.map((item) => item.filename),
      )

      if (renamePlans.length > 0) {
        const confirmed = await new Promise<boolean>((resolve) => {
          Modal.confirm({
            title: t('documents.duplicateTitle'),
            width: 640,
            content: (
              <div>
                <Typography.Paragraph>{t('documents.duplicateDescription')}</Typography.Paragraph>
                <List
                  size="small"
                  dataSource={renamePlans}
                  renderItem={(item) => (
                    <List.Item>
                      <Space direction="vertical" size={0}>
                        <Typography.Text delete>{item.originalName}</Typography.Text>
                        <Typography.Text strong>{item.nextName}</Typography.Text>
                      </Space>
                    </List.Item>
                  )}
                />
              </div>
            ),
            okText: t('documents.duplicateContinue'),
            cancelText: t('common.cancel'),
            onOk: () => resolve(true),
            onCancel: () => resolve(false),
          })
        })
        if (!confirmed) {
          return
        }
      }

      await uploadFiles(values.subject_id, files)
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
    if (status === 'deleting') {
      return t('documents.status.deleting')
    }
    if (status === 'delete_failed') {
      return t('documents.status.deleteFailed')
    }
    if (['parsing', 'parsed', 'chunking', 'chunked', 'embedding'].includes(status)) {
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

  async function openDetail(id: string) {
    setDetailLoading(true)
    try {
      setDetail(await getDocumentDetail(id))
    } catch {
      message.error(t('documents.detailFailed'))
    } finally {
      setDetailLoading(false)
    }
  }

  function handleDelete(record: DocumentInfo) {
    Modal.confirm({
      title: t('documents.deleteConfirm'),
      content: record.filename,
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteDocument(record.id)
          message.success(t('documents.deleteStarted'))
          await loadPageData(page)
        } catch {
          message.error(t('documents.deleteFailed'))
        }
      },
    })
  }

  async function handleRetry(taskId: string) {
    try {
      await retryIndexTask(taskId)
      message.success(t('documents.retryStarted'))
      if (detail) {
        await openDetail(detail.document.id)
      }
      await loadPageData(page)
    } catch {
      message.error(t('documents.retryFailed'))
    }
  }

  const selectedDocuments = documents.filter((item) => selectedRowKeys.includes(item.id))
  const retryableSelectedDocuments = selectedDocuments.filter((item) => item.status === 'failed')

  async function handleBatchDelete() {
    if (selectedDocuments.length === 0) {
      return
    }
    Modal.confirm({
      title: t('documents.batchDeleteConfirm', { count: selectedDocuments.length }),
      okButtonProps: { danger: true },
      onOk: async () => {
        setBatchOperating(true)
        try {
          await Promise.all(selectedDocuments.map((item) => deleteDocument(item.id)))
          message.success(t('documents.batchDeleteStarted', { count: selectedDocuments.length }))
          await loadPageData(page)
        } catch {
          message.error(t('documents.batchDeleteFailed'))
        } finally {
          setBatchOperating(false)
        }
      },
    })
  }

  async function handleBatchRetry() {
    if (retryableSelectedDocuments.length === 0) {
      return
    }
    setBatchOperating(true)
    try {
      const details = await Promise.all(retryableSelectedDocuments.map((item) => getDocumentDetail(item.id)))
      const taskIDs = details
        .map((item) =>
          [...item.tasks]
            .filter((task) => task.status === 'failed')
            .sort((left, right) => new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime())[0]?.id,
        )
        .filter((item): item is string => Boolean(item))

      await Promise.all(taskIDs.map((taskID) => retryIndexTask(taskID)))
      message.success(t('documents.batchRetryStarted', { count: taskIDs.length }))
      await loadPageData(page)
    } catch {
      message.error(t('documents.batchRetryFailed'))
    } finally {
      setBatchOperating(false)
    }
  }

  function removeSelectedFile(uid: string) {
    const nextFiles = (selectedFiles ?? []).filter((file) => file.uid !== uid)
    form.setFieldValue('file', nextFiles)
  }

  function clearSelectedFiles() {
    form.setFieldValue('file', [])
    setUploadFilePage(1)
  }

  return (
    <div className="dashboard-page documents-page">
      <Card className="page-card" title={t('documents.uploadTitle')}>
        <div className="upload-entry">
          <Space size={12} wrap>
            <Button
              type="primary"
              icon={<UploadOutlined />}
              loading={uploading}
              onClick={() => setUploadModalOpen(true)}
            >
              {t('documents.chooseFile')}
            </Button>
            {selectedFiles && selectedFiles.length > 0 ? (
              <Typography.Text type="secondary">
                {t('documents.selectedFiles', { count: selectedFiles.length })}
              </Typography.Text>
            ) : null}
          </Space>
        </div>
      </Card>

      <Modal
        title={t('documents.uploadTitle')}
        open={uploadModalOpen}
        width={640}
        destroyOnHidden={false}
        okText={t('documents.upload')}
        cancelText={t('common.cancel')}
        okButtonProps={{ loading: uploading }}
        onOk={() => form.submit()}
        onCancel={() => {
          if (!uploading) {
            setUploadModalOpen(false)
          }
        }}
      >
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
            <Upload
              beforeUpload={() => false}
              multiple
              showUploadList={false}
              accept=".md,.txt,.pdf,.doc,.docx,.pptx"
            >
              <Button icon={<UploadOutlined />}>{t('documents.chooseFile')}</Button>
            </Upload>
          </Form.Item>
          {selectedFiles && selectedFiles.length > 0 ? (
            <div className="upload-file-list">
              <div className="upload-file-list-header">
                <Typography.Text type="secondary">
                  {t('documents.selectedFiles', { count: selectedFiles.length })}
                </Typography.Text>
                <Button
                  type="text"
                  size="small"
                  icon={<ClearOutlined />}
                  aria-label={t('documents.clearSelectedFiles')}
                  onClick={clearSelectedFiles}
                />
              </div>
              {pagedSelectedFiles.map((file) => (
                <div className="upload-file-item" key={file.uid}>
                  <Typography.Text ellipsis>{file.name}</Typography.Text>
                  <Button
                    type="text"
                    size="small"
                    icon={<CloseOutlined />}
                    aria-label={t('common.delete')}
                    onClick={() => removeSelectedFile(file.uid)}
                  />
                </div>
              ))}
              {selectedFiles.length > uploadFilePageSize ? (
                <Pagination
                  className="upload-file-pagination"
                  size="small"
                  current={uploadFilePage}
                  pageSize={uploadFilePageSize}
                  total={selectedFiles.length}
                  onChange={setUploadFilePage}
                />
              ) : null}
            </div>
          ) : null}
        </Form>
      </Modal>

      <Card className="page-card" styles={{ body: { paddingTop: 16 } }}>
        <div className="panel-heading">
          <Space size={8} wrap>
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
            {selectedDocuments.length > 0 ? (
              <Typography.Text type="secondary">
                {t('documents.selectedRows', { count: selectedDocuments.length })}
              </Typography.Text>
            ) : null}
          </Space>
          <Space size={8}>
            <Button
              onClick={() => void handleBatchRetry()}
              loading={batchOperating}
              disabled={retryableSelectedDocuments.length === 0}
            >
              {t('documents.batchRetry')}
            </Button>
            <Button
              danger
              onClick={() => void handleBatchDelete()}
              loading={batchOperating}
              disabled={selectedDocuments.length === 0}
            >
              {t('documents.batchDelete')}
            </Button>
            <Button
              icon={<ReloadOutlined />}
              aria-label={t('common.refresh')}
              onClick={() => void loadPageData(page)}
              loading={loading}
            />
            <Tooltip title={t('documents.clearFailedAction')}>
              <Button
                icon={<ClearOutlined />}
                aria-label={t('documents.clearFailedAction')}
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
              tableLayout="fixed"
              rowKey="id"
              loading={loading}
              dataSource={documents}
              pagination={false}
              rowSelection={{
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys),
              }}
              columns={[
                { title: t('documents.fileName'), dataIndex: 'filename', width: 280, ellipsis: true },
                { title: t('documents.fileType'), dataIndex: 'file_type', width: 96 },
                {
                  title: t('documents.fileSize'),
                  dataIndex: 'file_size',
                  width: 110,
                  render: (_, record) => formatFileSize(record.file_size),
                },
                {
                  title: t('documents.status'),
                  dataIndex: 'status',
                  width: 160,
                  render: (_, record) => (
                    <div className="document-status-cell">
                      <div className="document-status-row">
                        <span>{renderDocumentStatus(record.status)}</span>
                        {(record.status === 'failed' || record.status === 'delete_failed') && record.error_message ? (
                          <Tooltip title={translateDocumentErrorMessage(record.error_message, t)}>
                            <ExclamationCircleFilled className="log-event-error-icon" />
                          </Tooltip>
                        ) : null}
                      </div>
                      {record.progress > 0 && record.progress < 100 ? (
                        <Progress percent={record.progress} size="small" showInfo={false} />
                      ) : null}
                    </div>
                  ),
                },
                {
                  title: t('documents.createdAt'),
                  dataIndex: 'created_at',
                  width: 168,
                  render: (_, record) => formatDateTime(record.created_at),
                },
                {
                  title: t('documents.actions'),
                  key: 'actions',
                  width: 104,
                  render: (_, record) => (
                    <Space size={4}>
                      <Tooltip title={t('documents.detail')}>
                        <Button
                          type="text"
                          icon={<EyeOutlined />}
                          aria-label={t('documents.detail')}
                          onClick={() => void openDetail(record.id)}
                        />
                      </Tooltip>
                      <Tooltip title={t('common.delete')}>
                        <Button
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          aria-label={t('common.delete')}
                          disabled={record.status === 'deleting'}
                          onClick={() => handleDelete(record)}
                        />
                      </Tooltip>
                    </Space>
                  ),
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
      </Card>
      <Drawer
        width={720}
        open={Boolean(detail)}
        loading={detailLoading}
        title={detail?.document.filename}
        onClose={() => setDetail(undefined)}
      >
        {detail ? (
          <>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 12, marginBottom: 16 }}>
              <Card size="small">
                <Typography.Text type="secondary">{t('documents.taskStatus')}</Typography.Text>
                <div style={{ fontSize: 20, fontWeight: 600, marginTop: 6 }}>
                  {documentStageLabel(detail.document.status, t)}
                </div>
              </Card>
              <Card size="small">
                <Typography.Text type="secondary">{t('documents.chunks')}</Typography.Text>
                <div style={{ fontSize: 20, fontWeight: 600, marginTop: 6 }}>{detail.chunks.length}</div>
              </Card>
              <Card size="small">
                <Typography.Text type="secondary">{t('documents.tasks')}</Typography.Text>
                <div style={{ fontSize: 20, fontWeight: 600, marginTop: 6 }}>{detail.tasks.length}</div>
              </Card>
              <Card size="small">
                <Typography.Text type="secondary">{t('documents.logs')}</Typography.Text>
                <div style={{ fontSize: 20, fontWeight: 600, marginTop: 6 }}>{detail.logs.length}</div>
              </Card>
            </div>
            {detail.document.error_message ? (
              <Typography.Paragraph type="danger" style={{ marginBottom: 16 }}>
                {translateDocumentErrorMessage(detail.document.error_message, t)}
              </Typography.Paragraph>
            ) : null}
            <Tabs
              items={[
              {
                key: 'chunks',
                label: t('documents.chunks'),
                children: (
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 10 }}
                    dataSource={detail.chunks}
                    columns={[
                      { title: t('documents.chunkIndex'), dataIndex: 'chunk_index', width: 72 },
                      { title: t('documents.section'), dataIndex: 'section', width: 160 },
                      { title: t('documents.content'), dataIndex: 'content' },
                    ]}
                  />
                ),
              },
              {
                key: 'tasks',
                label: t('documents.tasks'),
                children: (
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 10 }}
                    dataSource={detail.tasks}
                    columns={[
                      {
                        title: t('documents.taskType'),
                        dataIndex: 'task_type',
                        render: (value) => t(`tasks.type.${value}`),
                      },
                      {
                        title: t('documents.taskStatus'),
                        dataIndex: 'status',
                        render: (value) => t(`tasks.status.${value}`),
                      },
                      { title: t('documents.retryCount'), dataIndex: 'retry_count' },
                      {
                        title: t('documents.actions'),
                        key: 'actions',
                        render: (_, task) =>
                          task.status === 'failed' ? (
                            <Button
                              type="text"
                              icon={<RetweetOutlined />}
                              onClick={() => void handleRetry(task.id)}
                            >
                              {t('documents.retry')}
                            </Button>
                          ) : null,
                      },
                    ]}
                  />
                ),
              },
              {
                key: 'logs',
                label: t('documents.logs'),
                children: (
                  <Table
                    rowKey="id"
                    size="small"
                    pagination={{ pageSize: 10 }}
                    dataSource={detail.logs}
                    columns={[
                      {
                        title: t('documents.taskStatus'),
                        dataIndex: 'status',
                        render: (value) => t(`tasks.status.${value}`),
                      },
						{
							title: t('documents.logMessage'),
							dataIndex: 'message',
							render: (value: string) => translateTaskMessage(value, t),
						},
						{
							title: t('documents.errorMessage'),
							dataIndex: 'error_message',
							render: (value: string) => value ? translateDocumentErrorMessage(value, t) : '-',
						},
                      {
                        title: t('documents.createdAt'),
                        dataIndex: 'created_at',
                        render: formatDateTime,
                      },
                    ]}
                  />
                ),
              },
              ]}
            />
          </>
        ) : null}
      </Drawer>
    </div>
  )
}
