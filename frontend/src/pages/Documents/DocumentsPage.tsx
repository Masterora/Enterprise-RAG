import { Button, Form, Select, Table, Typography, Upload, message, type UploadFile } from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { listDocuments, uploadDocument, type DocumentInfo } from '../../api/documents'
import { listSubjects, type SubjectInfo } from '../../api/subjects'

export function DocumentsPage() {
  const [form] = Form.useForm()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [documents, setDocuments] = useState<DocumentInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)

  async function loadPageData() {
    setLoading(true)
    try {
      const [subjectData, documentData] = await Promise.all([
        listSubjects({ page: 1, page_size: 100 }),
        listDocuments({ page: 1, page_size: 50 }),
      ])
      setSubjects(subjectData.list)
      setDocuments(documentData.list)
    } catch {
      message.error('文档数据加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadPageData()
  }, [])

  async function handleUpload(values: { subject_id: string; file?: UploadFile[] }) {
    const file = values.file?.[0]?.originFileObj
    if (!file) {
      message.warning('请选择要上传的文件')
      return
    }

    setUploading(true)
    try {
      await uploadDocument({ subjectId: values.subject_id, file })
      form.resetFields()
      message.success('文档已上传')
      await loadPageData()
    } catch {
      message.error('文档上传失败')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="dashboard-page">
      <div className="page-heading">
        <div>
          <h1 className="page-title">文档</h1>
          <p className="page-subtitle">上传文档、查看解析状态并管理索引任务。</p>
        </div>
        <Button onClick={loadPageData}>刷新</Button>
      </div>

      <div className="status-panel">
        <Typography.Title level={4}>上传文档</Typography.Title>
        <Form form={form} layout="vertical" onFinish={handleUpload}>
          <Form.Item
            label="知识库"
            name="subject_id"
            rules={[{ required: true, message: '请选择知识库' }]}
          >
            <Select
              placeholder="选择知识库"
              options={subjects.map((subject) => ({ label: subject.name, value: subject.id }))}
            />
          </Form.Item>
          <Form.Item
            label="文件"
            name="file"
            rules={[{ required: true, message: '请选择文件' }]}
            valuePropName="fileList"
            getValueFromEvent={(event) => (Array.isArray(event) ? event : event?.fileList)}
          >
            <Upload beforeUpload={() => false} maxCount={1}>
              <Button icon={<UploadOutlined />}>选择文件</Button>
            </Upload>
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={uploading}>
            上传
          </Button>
        </Form>
      </div>

      <div className="status-panel">
        <Typography.Title level={4}>文档列表</Typography.Title>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={documents}
          pagination={false}
          columns={[
            { title: '文件名', dataIndex: 'filename' },
            { title: '类型', dataIndex: 'file_type' },
            { title: '大小', dataIndex: 'file_size' },
            { title: '状态', dataIndex: 'status' },
            { title: '创建时间', dataIndex: 'created_at' },
          ]}
        />
      </div>
    </div>
  )
}
