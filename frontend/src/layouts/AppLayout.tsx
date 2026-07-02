import { BookOutlined, DashboardOutlined, FileTextOutlined, MessageOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import { Button, Layout, Menu, Space, Typography, theme } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

const { Header, Sider, Content } = Layout

type AppLayoutProps = {
  isDarkMode: boolean
  onToggleTheme: () => void
}

export function AppLayout({ isDarkMode, onToggleTheme }: AppLayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { token } = theme.useToken()

  return (
    <Layout className={`app-shell ${isDarkMode ? 'theme-dark' : 'theme-light'}`}>
      <Sider
        className="app-sider"
        theme={isDarkMode ? 'dark' : 'light'}
        width={232}
        style={{ borderRight: `1px solid ${token.colorBorder}` }}
      >
        <div className="app-brand" style={{ borderBottom: `1px solid ${token.colorBorder}` }}>
          <div className="app-brand-title" style={{ color: token.colorText }}>
            Enterprise RAG
          </div>
          <div className="app-brand-subtitle" style={{ color: token.colorTextSecondary }}>
            Knowledge Console
          </div>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[location.pathname]}
          items={[
            { key: '/dashboard', icon: <DashboardOutlined />, label: 'Dashboard' },
            { key: '/subjects', icon: <BookOutlined />, label: '知识库' },
            { key: '/documents', icon: <FileTextOutlined />, label: '文档' },
            { key: '/chat', icon: <MessageOutlined />, label: '问答' },
          ]}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          className="app-header"
          style={{
            background: token.colorBgContainer,
            borderBottom: `1px solid ${token.colorBorder}`,
          }}
        >
          <Typography.Text strong style={{ color: token.colorText }}>
            企业知识库 RAG 问答系统
          </Typography.Text>
          <Space>
            <Button
              icon={isDarkMode ? <MoonOutlined /> : <SunOutlined />}
              onClick={onToggleTheme}
              type="text"
              style={{ color: token.colorText }}
            >
              {isDarkMode ? 'Dark' : 'Light'}
            </Button>
          </Space>
        </Header>
        <Content className="app-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
