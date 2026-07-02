import { useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp, ConfigProvider, theme as antdTheme } from 'antd'
import { BrowserRouter } from 'react-router-dom'
import App from './App'

const queryClient = new QueryClient()
const THEME_STORAGE_KEY = 'enterprise-rag-theme'

export function RootApp() {
  const [isDarkMode, setIsDarkMode] = useState(() => {
    return window.localStorage.getItem(THEME_STORAGE_KEY) === 'dark'
  })

  useEffect(() => {
    document.documentElement.dataset.theme = isDarkMode ? 'dark' : 'light'
    window.localStorage.setItem(THEME_STORAGE_KEY, isDarkMode ? 'dark' : 'light')
  }, [isDarkMode])

  return (
    <ConfigProvider
      theme={{
        algorithm: isDarkMode ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: isDarkMode ? '#6aa89a' : '#226c5f',
          colorBgBase: isDarkMode ? '#141414' : '#f4f7f4',
          colorBgContainer: isDarkMode ? '#1f1f1f' : '#ffffff',
          colorBorder: isDarkMode ? '#303030' : '#dfe6dd',
          colorText: isDarkMode ? '#f5f5f5' : '#1e2528',
          colorTextSecondary: isDarkMode ? '#bfbfbf' : '#63716b',
          borderRadius: 6,
          fontFamily:
            'Aptos, "Satoshi", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
        components: {
          Layout: {
            headerBg: isDarkMode ? '#1b1b1b' : '#ffffff',
            siderBg: isDarkMode ? '#161616' : '#ffffff',
            bodyBg: isDarkMode ? '#141414' : '#f4f7f4',
            triggerBg: isDarkMode ? '#161616' : '#ffffff',
          },
          Menu: {
            itemBg: isDarkMode ? '#161616' : '#ffffff',
            subMenuItemBg: isDarkMode ? '#161616' : '#ffffff',
            itemSelectedBg: isDarkMode ? '#262626' : '#e8f1ee',
            itemHoverBg: isDarkMode ? '#202020' : '#f2f7f5',
            itemColor: isDarkMode ? '#d9d9d9' : '#4b5b55',
            itemSelectedColor: isDarkMode ? '#ffffff' : '#226c5f',
          },
          Button: {
            defaultBg: isDarkMode ? '#1f1f1f' : '#ffffff',
            defaultBorderColor: isDarkMode ? '#303030' : '#d9d9d9',
            defaultColor: isDarkMode ? '#f5f5f5' : '#1e2528',
          },
          Card: {
            colorBgContainer: isDarkMode ? '#1f1f1f' : '#ffffff',
          },
          Table: {
            headerBg: isDarkMode ? '#1f1f1f' : '#fafafa',
            headerColor: isDarkMode ? '#f5f5f5' : '#1e2528',
            rowHoverBg: isDarkMode ? '#262626' : '#f5f5f5',
          },
        },
      }}
    >
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <App
              isDarkMode={isDarkMode}
              onToggleTheme={() => setIsDarkMode((value) => !value)}
            />
          </BrowserRouter>
        </QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  )
}
