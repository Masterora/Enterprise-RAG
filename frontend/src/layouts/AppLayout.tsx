import { BookOutlined, DashboardOutlined, FileTextOutlined, MessageOutlined } from '@ant-design/icons'
import { Layout, Menu, Space, Typography } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

const { Header, Sider, Content } = Layout

export function AppLayout() {
  const location = useLocation()
  const navigate = useNavigate()

  return (
    <Layout className="app-shell">
      <Sider className="app-sider" theme="light" width={232}>
        <div className="app-brand">
          <div className="app-brand-title">Enterprise RAG</div>
          <div className="app-brand-subtitle">Knowledge Console</div>
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
        <Header className="app-header">
          <Typography.Text strong>企业知识库 RAG 问答系统</Typography.Text>
          <Space>
            <Typography.Text type="secondary">local</Typography.Text>
          </Space>
        </Header>
        <Content className="app-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
