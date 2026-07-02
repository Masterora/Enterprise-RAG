import { Button, Form, Input, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { login, setAuthToken } from '../../api/auth'

export function LoginPage() {
  const [form] = Form.useForm()
  const navigate = useNavigate()

  async function handleLogin(values: { username: string; password: string }) {
    try {
      const result = await login(values)
      setAuthToken(result.token)
      message.success(`欢迎，${result.user.username}`)
      navigate('/dashboard')
    } catch {
      message.error('登录失败')
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel">
        <h1 className="login-title">Enterprise RAG</h1>
        <p className="login-desc">登录企业知识库问答系统。</p>
        <Form form={form} layout="vertical" onFinish={handleLogin}>
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
