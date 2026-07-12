import { useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp, ConfigProvider, theme as antdTheme } from 'antd'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { getAuthToken } from './api/auth'
import { I18nProvider } from './i18n'

const queryClient = new QueryClient()
const THEME_STORAGE_KEY = 'enterprise-rag-theme'

export function RootApp() {
  const [isDarkMode, setIsDarkMode] = useState(() => {
    return window.localStorage.getItem(THEME_STORAGE_KEY) === 'dark'
  })
  const [authToken, setAuthToken] = useState(() => getAuthToken())

  useEffect(() => {
    document.documentElement.dataset.theme = isDarkMode ? 'dark' : 'light'
    window.localStorage.setItem(THEME_STORAGE_KEY, isDarkMode ? 'dark' : 'light')
  }, [isDarkMode])

  return (
    <ConfigProvider
      theme={{
        algorithm: isDarkMode ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: isDarkMode ? '#7c9cff' : '#315efb',
          colorBgBase: isDarkMode ? '#0f1115' : '#f3f5f9',
          colorBgContainer: isDarkMode ? '#171a21' : '#ffffff',
          colorBorder: isDarkMode ? '#2a2f3a' : '#d9dee8',
          colorText: isDarkMode ? '#f3f6fb' : '#1a2233',
          colorTextSecondary: isDarkMode ? '#a4adbd' : '#667085',
          borderRadius: 6,
          fontFamily:
            'Aptos, "Satoshi", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
        components: {
          Layout: {
            headerBg: isDarkMode ? '#12151c' : '#ffffff',
            siderBg: isDarkMode ? '#11141a' : '#ffffff',
            bodyBg: isDarkMode ? '#0f1115' : '#f3f5f9',
            triggerBg: isDarkMode ? '#11141a' : '#ffffff',
          },
          Menu: {
            itemBg: isDarkMode ? '#11141a' : '#ffffff',
            subMenuItemBg: isDarkMode ? '#11141a' : '#ffffff',
            itemSelectedBg: isDarkMode ? '#1c2330' : '#eef3ff',
            itemHoverBg: isDarkMode ? '#171d28' : '#f6f8fd',
            itemColor: isDarkMode ? '#c7d0e0' : '#516076',
            itemSelectedColor: isDarkMode ? '#f3f6fb' : '#315efb',
          },
          Button: {
            defaultBg: isDarkMode ? '#171a21' : '#ffffff',
            defaultBorderColor: isDarkMode ? '#2a2f3a' : '#d9dee8',
            defaultColor: isDarkMode ? '#f3f6fb' : '#1a2233',
          },
          Card: {
            colorBgContainer: isDarkMode ? '#171a21' : '#ffffff',
            headerBg: isDarkMode ? '#171a21' : '#ffffff',
          },
          Collapse: {
            headerBg: isDarkMode ? '#171a21' : '#ffffff',
            contentBg: isDarkMode ? '#171a21' : '#ffffff',
            borderlessContentBg: isDarkMode ? '#171a21' : '#ffffff',
          },
          Input: {
            activeBorderColor: isDarkMode ? '#7c9cff' : '#315efb',
            hoverBorderColor: isDarkMode ? '#3a4354' : '#315efb',
            colorBgContainer: isDarkMode ? '#11141a' : '#ffffff',
            colorText: isDarkMode ? '#f3f6fb' : '#1a2233',
            colorTextPlaceholder: isDarkMode ? '#7f8aa0' : '#98a2b3',
          },
          Select: {
            optionSelectedBg: isDarkMode ? '#1c2330' : '#eef3ff',
            colorBgContainer: isDarkMode ? '#11141a' : '#ffffff',
            colorText: isDarkMode ? '#f3f6fb' : '#1a2233',
          },
          Tabs: {
            itemColor: isDarkMode ? '#a4adbd' : '#667085',
            itemSelectedColor: isDarkMode ? '#f3f6fb' : '#1a2233',
            inkBarColor: isDarkMode ? '#7c9cff' : '#315efb',
          },
          Table: {
            headerBg: isDarkMode ? '#171a21' : '#f8faff',
            headerColor: isDarkMode ? '#f3f6fb' : '#1a2233',
            rowHoverBg: isDarkMode ? '#1b2130' : '#f6f8fd',
          },
        },
      }}
    >
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <I18nProvider>
            <BrowserRouter>
              <App
                authToken={authToken}
                onAuthChange={setAuthToken}
                isDarkMode={isDarkMode}
                onToggleTheme={() => setIsDarkMode((value) => !value)}
              />
            </BrowserRouter>
          </I18nProvider>
        </QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  )
}
