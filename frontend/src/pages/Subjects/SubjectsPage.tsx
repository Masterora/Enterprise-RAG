import { DeleteOutlined, EditOutlined } from '@ant-design/icons'
import { Button, Card, Form, Input, Modal, Pagination, Popconfirm, Select, Space, Table, Typography, message } from 'antd'
import { useCallback, useEffect, useState } from 'react'
import {
  createSubject,
  deleteSubject,
  isSubjectNameConflict,
  listSubjects,
  updateSubject,
  type SubjectInfo,
} from '../../api/subjects'
import { useI18n } from '../../useI18n'

export function SubjectsPage() {
  const [form] = Form.useForm()
  const [editForm] = Form.useForm()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [editingSubject, setEditingSubject] = useState<SubjectInfo | null>(null)
  const [updating, setUpdating] = useState(false)
  const { t } = useI18n()

  const loadSubjects = useCallback(async (nextPage = page) => {
    setLoading(true)
    try {
      const data = await listSubjects({ page: nextPage, page_size: 10 })
      setSubjects(data.list)
      setTotal(data.total)
      setPage(nextPage)
    } catch {
      message.error(t('subjects.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [page, t])

  useEffect(() => {
    void loadSubjects()
  }, [loadSubjects])

  async function handleCreate(values: { name: string; description?: string; visibility?: string }) {
    setSaving(true)
    try {
      await createSubject(values)
      form.resetFields()
      message.success(t('subjects.createSuccess'))
      await loadSubjects(1)
    } catch (error) {
      message.error(
        isSubjectNameConflict(error) ? t('subjects.nameDuplicate') : t('subjects.createFailed'),
      )
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(values: { name: string; description?: string; visibility?: string }) {
    if (!editingSubject) {
      return
    }
    setUpdating(true)
    try {
      await updateSubject({
        id: editingSubject.id,
        name: values.name,
        description: values.description,
        visibility: values.visibility,
      })
      message.success(t('subjects.updateSuccess'))
      setEditingSubject(null)
      editForm.resetFields()
      await loadSubjects(page)
    } catch (error) {
      message.error(
        isSubjectNameConflict(error) ? t('subjects.nameDuplicate') : t('subjects.updateFailed'),
      )
    } finally {
      setUpdating(false)
    }
  }

  async function handleDelete(subject: SubjectInfo) {
    try {
      await deleteSubject(subject.id)
      message.success(t('subjects.deleteSuccess'))
      if (editingSubject?.id === subject.id) {
        setEditingSubject(null)
        editForm.resetFields()
      }
      await loadSubjects(page)
    } catch {
      message.error(t('subjects.deleteFailed'))
    }
  }

  function openEditModal(subject: SubjectInfo) {
    setEditingSubject(subject)
    editForm.setFieldsValue({
      name: subject.name,
      description: subject.description,
      visibility: subject.visibility,
    })
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

  function renderVisibility(value: string) {
    return value === 'public' ? t('subjects.visibility.public') : t('subjects.visibility.private')
  }

  return (
    <div className="dashboard-page subjects-page">
      <Card className="page-card" title={t('subjects.createTitle')}>
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <div className="form-grid">
            <Form.Item
              label={t('subjects.name')}
              name="name"
              rules={[{ required: true, message: t('subjects.nameRequired') }]}
            >
              <Input placeholder={t('subjects.namePlaceholder')} />
            </Form.Item>
            <Form.Item label={t('subjects.visibility')} name="visibility" initialValue="private">
              <Select
                options={[
                  { label: t('subjects.visibility.private'), value: 'private' },
                  { label: t('subjects.visibility.public'), value: 'public' },
                ]}
              />
            </Form.Item>
          </div>
          <Form.Item label={t('subjects.description')} name="description">
            <Input.TextArea rows={3} placeholder={t('subjects.descriptionPlaceholder')} />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saving}>
            {t('subjects.createTitle')}
          </Button>
        </Form>
      </Card>

      <Card className="page-card" styles={{ body: { paddingTop: 16 } }}>
        <div className="panel-heading">
          <Typography.Title level={4}>{t('subjects.listTitle')}</Typography.Title>
          <span className="panel-heading-spacer" />
        </div>
        <div className="fixed-table-shell">
          <div className="fixed-table-body">
            <Table
              tableLayout="fixed"
              rowKey="id"
              loading={loading}
              dataSource={subjects}
              pagination={false}
              columns={[
                { title: t('subjects.name'), dataIndex: 'name', width: 220, ellipsis: true },
                {
                  title: t('subjects.description'),
                  dataIndex: 'description',
                  width: 420,
                  render: (value?: string) => (
                    <Typography.Paragraph
                      className="subject-description-cell"
                      ellipsis={{ rows: 2, tooltip: value || '-' }}
                    >
                      {value || '-'}
                    </Typography.Paragraph>
                  ),
                },
                {
                  title: t('subjects.visibility'),
                  dataIndex: 'visibility',
                  width: 120,
                  render: (_, record) => renderVisibility(record.visibility),
                },
                {
                  title: t('subjects.createdAt'),
                  dataIndex: 'created_at',
                  width: 180,
                  render: (_, record) => formatDateTime(record.created_at),
                },
                {
                  title: t('subjects.actions'),
                  key: 'actions',
                  width: 96,
                  align: 'center',
                  render: (_, record) => (
                    <Space size={4}>
                      <Button
                        className="table-action-button"
                        type="text"
                        icon={<EditOutlined />}
                        aria-label={t('common.edit')}
                        onClick={() => openEditModal(record)}
                      />
                      <Popconfirm
                        title={t('subjects.deleteConfirm')}
                        onConfirm={() => void handleDelete(record)}
                        okText={t('common.delete')}
                        cancelText={t('common.cancel')}
                      >
                        <Button
                          className="table-action-button table-action-danger"
                          type="text"
                          icon={<DeleteOutlined />}
                          aria-label={t('common.delete')}
                        />
                      </Popconfirm>
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
            onChange={(nextPage) => void loadSubjects(nextPage)}
          />
        </div>
      </Card>

      <Modal
        title={t('subjects.updateTitle')}
        open={Boolean(editingSubject)}
        onCancel={() => {
          setEditingSubject(null)
          editForm.resetFields()
        }}
        footer={null}
        destroyOnHidden
      >
        <Form form={editForm} layout="vertical" onFinish={handleUpdate}>
          <Form.Item
            label={t('subjects.name')}
            name="name"
            rules={[{ required: true, message: t('subjects.nameRequired') }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label={t('subjects.description')} name="description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item label={t('subjects.visibility')} name="visibility" initialValue="private">
            <Select
              options={[
                { label: t('subjects.visibility.private'), value: 'private' },
                { label: t('subjects.visibility.public'), value: 'public' },
              ]}
            />
          </Form.Item>
          <div className="modal-actions">
            <Button
              onClick={() => {
                setEditingSubject(null)
                editForm.resetFields()
              }}
            >
              {t('common.cancel')}
            </Button>
            <Button type="primary" htmlType="submit" loading={updating}>
              {t('common.save')}
            </Button>
          </div>
        </Form>
      </Modal>
    </div>
  )
}
