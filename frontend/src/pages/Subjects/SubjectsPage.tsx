import { Button, Form, Input, Select, Table, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { createSubject, listSubjects, type SubjectInfo } from '../../api/subjects'

export function SubjectsPage() {
  const [form] = Form.useForm()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  async function loadSubjects() {
    setLoading(true)
    try {
      const data = await listSubjects({ page: 1, page_size: 50 })
      setSubjects(data.list)
    } catch {
      message.error('知识库列表加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadSubjects()
  }, [])

  async function handleCreate(values: { name: string; description?: string; visibility?: string }) {
    setSaving(true)
    try {
      await createSubject(values)
      form.resetFields()
      message.success('知识库已创建')
      await loadSubjects()
    } catch {
      message.error('知识库创建失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="dashboard-page">
      <div className="page-heading">
        <div>
          <h1 className="page-title">知识库</h1>
          <p className="page-subtitle">创建、维护并进入企业知识库。</p>
        </div>
        <Button onClick={loadSubjects}>刷新</Button>
      </div>

      <div className="status-panel">
        <Typography.Title level={4}>创建知识库</Typography.Title>
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <div className="form-grid">
            <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入知识库名称' }]}>
              <Input placeholder="例如：产品文档" />
            </Form.Item>
            <Form.Item label="可见性" name="visibility" initialValue="private">
              <Select
                options={[
                  { label: '私有', value: 'private' },
                  { label: '公开', value: 'public' },
                ]}
              />
            </Form.Item>
          </div>
          <Form.Item label="描述" name="description">
            <Input.TextArea rows={3} placeholder="记录这个知识库的用途" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saving}>
            创建
          </Button>
        </Form>
      </div>

      <div className="status-panel">
        <Typography.Title level={4}>知识库列表</Typography.Title>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={subjects}
          pagination={false}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '描述', dataIndex: 'description' },
            { title: '可见性', dataIndex: 'visibility' },
            { title: '创建时间', dataIndex: 'created_at' },
          ]}
        />
      </div>
    </div>
  )
}
