import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'framework',
              test: /node_modules.*\/(react|react-dom|react-router|scheduler|@tanstack)\//,
              priority: 30,
            },
            {
              name: 'antd',
              test: /node_modules.*\/antd\//,
              priority: 20,
              maxSize: 450 * 1024,
            },
            {
              name: 'antd-support',
              test: /node_modules.*\/(@ant-design|rc-[^/]+|@rc-component)\//,
              priority: 20,
            },
            {
              name: 'markdown',
              test: /node_modules.*\/(react-markdown|remark|rehype|unified|micromark|mdast|hast)[^/]*\//,
              priority: 10,
            },
            {
              name: 'vendor',
              test: /node_modules/,
            },
          ],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:9999',
        changeOrigin: true,
      },
    },
  },
})
