import { Button, Form, Input } from 'antd'
import { useNavigate } from 'react-router-dom'

export function LoginPage() {
  const navigate = useNavigate()

  return (
    <main className="login-page">
      <section className="login-panel">
        <h1 className="login-title">Enterprise RAG</h1>
        <p className="login-desc">登录企业知识库问答系统。</p>
        <Form layout="vertical" onFinish={() => navigate('/dashboard')}>
          <Form.Item label="用户名" name="username" initialValue="demo">
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item label="密码" name="password" initialValue="demo">
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block>
            进入系统
          </Button>
        </Form>
      </section>
    </main>
  )
}
