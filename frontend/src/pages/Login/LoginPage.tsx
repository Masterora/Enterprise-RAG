import { MoonOutlined, SunOutlined } from '@ant-design/icons'
import { Button, Form, Input, Tabs, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { login, register, setAuthToken } from '../../api/auth'
import { ActionIconButton } from '../../components/ActionIconButton'
import { useI18n } from '../../useI18n'

type LoginPageProps = {
  isDarkMode: boolean
  onToggleTheme: () => void
  onAuthSuccess: (token: string) => void
}

export function LoginPage({ isDarkMode, onToggleTheme, onAuthSuccess }: LoginPageProps) {
  const [loginForm] = Form.useForm()
  const [registerForm] = Form.useForm()
  const navigate = useNavigate()
  const { t } = useI18n()

  async function handleLogin(values: { username: string; password: string }) {
    try {
      const result = await login(values)
      setAuthToken(result.token)
      onAuthSuccess(result.token)
      message.success(t('login.success', { username: result.user.nickname || result.user.username }))
      navigate('/dashboard')
    } catch {
      message.error(t('login.failed'))
    }
  }

  async function handleRegister(values: {
    username: string
    password: string
    confirm_password: string
    nickname?: string
    email?: string
  }) {
    try {
      const result = await register(values)
      setAuthToken(result.token)
      onAuthSuccess(result.token)
      message.success(
        t('register.success', { username: result.user.nickname || result.user.username }),
      )
      navigate('/dashboard')
    } catch {
      message.error(t('register.failed'))
    }
  }

  return (
    <main className={`login-page ${isDarkMode ? 'theme-dark' : 'theme-light'}`}>
      <section className="login-panel">
        <div className="login-topbar">
          <ActionIconButton
            icon={isDarkMode ? <MoonOutlined /> : <SunOutlined />}
            label={isDarkMode ? t('common.dark') : t('common.light')}
            onClick={onToggleTheme}
            effect="theme"
          />
        </div>
        <h1 className="login-title">Enterprise RAG</h1>
        <p className="login-desc">{t('login.description')}</p>
        <Tabs
          items={[
            {
              key: 'login',
              label: t('login.tab'),
              children: (
                <Form form={loginForm} layout="vertical" onFinish={handleLogin}>
                  <Form.Item
                    label={t('login.username')}
                    name="username"
                    required
                    rules={[{ required: true, message: t('register.usernameRequired') }]}
                  >
                    <Input autoComplete="username" />
                  </Form.Item>
                  <Form.Item
                    label={t('login.password')}
                    name="password"
                    required
                    rules={[{ required: true, message: t('register.passwordRequired') }]}
                  >
                    <Input.Password autoComplete="current-password" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" block>
                    {t('login.submit')}
                  </Button>
                </Form>
              ),
            },
            {
              key: 'register',
              label: t('register.tab'),
              children: (
                <Form form={registerForm} layout="vertical" onFinish={handleRegister}>
                  <Form.Item
                    label={t('login.username')}
                    name="username"
                    required
                    extra={t('register.usernameReadonly')}
                    rules={[{ required: true, message: t('register.usernameRequired') }]}
                  >
                    <Input autoComplete="username" />
                  </Form.Item>
                  <Form.Item
                    label={t('login.nickname')}
                    name="nickname"
                    required
                    rules={[{ required: true, message: t('register.nicknameRequired') }]}
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item
                    label={t('login.email')}
                    name="email"
                    required
                    rules={[
                      { required: true, message: t('register.emailRequired') },
                      { type: 'email', message: t('settings.emailInvalid') },
                    ]}
                  >
                    <Input autoComplete="email" />
                  </Form.Item>
                  <Form.Item
                    label={t('login.password')}
                    name="password"
                    required
                    rules={[{ required: true, message: t('register.passwordRequired') }]}
                  >
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                  <Form.Item
                    label={t('register.confirmPassword')}
                    name="confirm_password"
                    required
                    dependencies={['password']}
                    rules={[
                      { required: true, message: t('register.confirmRequired') },
                      ({ getFieldValue }) => ({
                        validator(_, value) {
                          if (!value || getFieldValue('password') === value) {
                            return Promise.resolve()
                          }
                          return Promise.reject(new Error(t('register.confirmMismatch')))
                        },
                      }),
                    ]}
                  >
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" block>
                    {t('register.submit')}
                  </Button>
                </Form>
              ),
            },
          ]}
        />
      </section>
    </main>
  )
}
