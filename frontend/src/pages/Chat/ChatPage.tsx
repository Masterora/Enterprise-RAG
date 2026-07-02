import { Button, Form, Input, Select, Space, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { listSubjects, type SubjectInfo } from '../../api/subjects'

export function ChatPage() {
  const [form] = Form.useForm()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [answer, setAnswer] = useState('')

  useEffect(() => {
    async function loadSubjects() {
      try {
        const data = await listSubjects({ page: 1, page_size: 100 })
        setSubjects(data.list)
      } catch {
        message.error('知识库列表加载失败')
      }
    }

    void loadSubjects()
  }, [])

  function handleAsk(values: { subject_id: string; question: string }) {
    const subject = subjects.find((item) => item.id === values.subject_id)
    setAnswer(
      `已收到问题：“${values.question}”。当前知识库为“${subject?.name ?? '未选择'}”。真实 RAG 检索和大模型回答将在问答链路接入后返回。`,
    )
  }

  return (
    <div className="dashboard-page">
      <h1 className="page-title">问答</h1>
      <p className="page-subtitle">选择知识库并进行带引用来源的 RAG 问答。</p>
      <div className="status-panel">
        <Typography.Title level={4}>问答模块</Typography.Title>
        <Form form={form} layout="vertical" onFinish={handleAsk}>
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
            label="问题"
            name="question"
            rules={[{ required: true, message: '请输入问题' }]}
          >
            <Input.TextArea rows={4} placeholder="输入你想询问的问题" />
          </Form.Item>
          <Button type="primary" htmlType="submit">
            提问
          </Button>
        </Form>
      </div>
      {answer && (
        <div className="status-panel">
          <Space direction="vertical">
            <Typography.Title level={4}>回答</Typography.Title>
            <Typography.Paragraph>{answer}</Typography.Paragraph>
          </Space>
        </div>
      )}
    </div>
  )
}
