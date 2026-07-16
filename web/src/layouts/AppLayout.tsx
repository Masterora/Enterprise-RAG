import {
  BookOutlined,
  FileTextOutlined,
  MessageOutlined,
  MoonOutlined,
  SettingOutlined,
  ToolOutlined,
  SunOutlined,
} from '@ant-design/icons'
import { Button, Dropdown, Layout, Menu, Space, Typography, theme, type MenuProps } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { clearAuthToken, getMe, type UserInfo } from '../api/auth'
import { ActionIconButton } from '../components/ActionIconButton'
import { isSupportedLanguage } from '../i18n-core'
import { useI18n } from '../useI18n'

const { Header, Sider, Content } = Layout

type AppLayoutProps = {
  isDarkMode: boolean
  onToggleTheme: () => void
  onLogout: () => void
}

export function AppLayout({ isDarkMode, onToggleTheme, onLogout }: AppLayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { token } = theme.useToken()
  const { t, setLanguage } = useI18n()
  const [currentUser, setCurrentUser] = useState<UserInfo | null>(null)

  useEffect(() => {
    async function syncLanguage() {
      try {
        const user = await getMe()
        setCurrentUser(user)
        if (isSupportedLanguage(user.language)) {
          setLanguage(user.language)
        }
      } catch {
        return
      }
    }

    void syncLanguage()
  }, [setLanguage])

  const displayName = currentUser?.nickname || currentUser?.username || t('common.account')

  const pageMeta = useMemo(() => {
    if (location.pathname === '/subjects') {
      return { title: t('subjects.title'), subtitle: t('subjects.subtitle') }
    }
    if (location.pathname === '/documents') {
      return { title: t('documents.title'), subtitle: t('documents.subtitle') }
    }
    if (location.pathname === '/chat') {
      return { title: t('chat.title'), subtitle: t('chat.subtitle') }
    }
    if (location.pathname === '/logs') {
      return { title: t('logs.title'), subtitle: t('logs.subtitle') }
    }
    if (location.pathname === '/settings/profile') {
      return { title: t('settings.profile'), subtitle: t('settings.profileSubtitle') }
    }
    if (location.pathname === '/settings/model') {
      return { title: t('settings.model.title'), subtitle: t('settings.model.subtitle') }
    }
    if (location.pathname === '/settings/security') {
      return { title: t('settings.passwordPage'), subtitle: t('settings.passwordSubtitle') }
    }
    return { title: t('app.title'), subtitle: t('app.brandSubtitle') }
  }, [location.pathname, t])

  useEffect(() => {
    document.title = `${pageMeta.title} - Enterprise-RAG`
  }, [pageMeta.title])

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'switch',
      label: t('common.switchUser'),
    },
    {
      key: 'logout',
      label: t('common.logout'),
    },
  ]

  function handleLeave() {
    clearAuthToken()
    onLogout()
    navigate('/login')
  }

  function handleUserMenuClick({ key }: { key: string }) {
    if (key === 'switch' || key === 'logout') {
      handleLeave()
    }
  }

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
            {t('app.brandSubtitle')}
          </div>
        </div>
        <Menu
          mode="inline"
          defaultOpenKeys={location.pathname.startsWith('/settings') ? ['settings-group'] : []}
          selectedKeys={[location.pathname]}
          items={[
            { key: '/chat', icon: <MessageOutlined />, label: t('nav.chat') },
            { key: '/subjects', icon: <BookOutlined />, label: t('nav.subjects') },
            { key: '/documents', icon: <FileTextOutlined />, label: t('nav.documents') },
            { key: '/logs', icon: <ToolOutlined />, label: t('nav.logs') },
            {
              key: 'settings-group',
              icon: <SettingOutlined />,
              label: t('nav.settings'),
              children: [
                { key: '/settings/profile', label: t('settings.navProfile') },
                { key: '/settings/model', label: t('settings.navModel') },
                { key: '/settings/security', label: t('settings.navSecurity') },
              ],
            },
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
          <div className="app-header-copy">
            <Typography.Text strong className="app-header-title" style={{ color: token.colorText }}>
              {pageMeta.title}
            </Typography.Text>
            <Typography.Text className="app-header-subtitle" style={{ color: token.colorTextSecondary }}>
              {pageMeta.subtitle}
            </Typography.Text>
          </div>
          <Space>
            <Dropdown menu={{ items: userMenuItems, onClick: handleUserMenuClick }} trigger={['click']}>
              <Button type="text" className="header-user-trigger">
                {displayName}
              </Button>
            </Dropdown>
            <ActionIconButton
              icon={isDarkMode ? <MoonOutlined /> : <SunOutlined />}
              label={isDarkMode ? t('common.dark') : t('common.light')}
              onClick={onToggleTheme}
              effect="theme"
            />
          </Space>
        </Header>
        <Content className="app-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
