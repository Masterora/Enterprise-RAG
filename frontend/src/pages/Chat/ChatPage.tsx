import { Button, Form, Input, Select, Space, Tag, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { searchRetrieval, type RetrievalChunk } from '../../api/retrieval'
import { listSubjects, type SubjectInfo } from '../../api/subjects'

export function ChatPage() {
  const [form] = Form.useForm()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [chunks, setChunks] = useState<RetrievalChunk[]>([])
  const [searching, setSearching] = useState(false)

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

  async function handleAsk(values: { subject_id: string; question: string }) {
    setSearching(true)
    try {
      const result = await searchRetrieval({
        subject_id: values.subject_id,
        query: values.question,
        top_k: 5,
      })
      setChunks(result)
      if (result.length === 0) {
        message.warning('没有检索到结果')
      }
    } catch {
      message.error('检索失败')
    } finally {
      setSearching(false)
    }
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
            {searching ? '检索中...' : '检索'}
          </Button>
        </Form>
      </div>
      {chunks.length > 0 && (
        <div className="status-panel">
          <Space direction="vertical">
            <Typography.Title level={4}>检索结果</Typography.Title>
            {chunks.map((chunk) => (
              <div
                key={chunk.id}
                style={{
                  border: '1px solid rgba(15, 23, 42, 0.08)',
                  borderRadius: 12,
                  padding: 16,
                  background: '#fff',
                }}
              >
                <Space size={[8, 8]} wrap>
                  <Tag color="blue">命中章节：{chunk.section || '未命名'}</Tag>
                  <Tag color="gold">来源文档：{chunk.doc_name || '未知文档'}</Tag>
                  <Tag>页码：{chunk.page > 0 ? chunk.page : '无'}</Tag>
                  <Tag color="green">相似度：{chunk.score.toFixed(4)}</Tag>
                </Space>
                <Typography.Paragraph strong style={{ marginTop: 12, marginBottom: 8 }}>
                  正文预览
                </Typography.Paragraph>
                <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                  {chunk.content}
                </Typography.Paragraph>
              </div>
            ))}
          </Space>
        </div>
      )}
    </div>
  )
}
