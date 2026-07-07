/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_CHAT_TOP_K?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
