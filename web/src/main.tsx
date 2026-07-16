import { StrictMode } from 'react'
import 'antd/dist/reset.css'
import { createRoot } from 'react-dom/client'
import './index.css'
import { RootApp } from './RootApp'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RootApp />
  </StrictMode>,
)
