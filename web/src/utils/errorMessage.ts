type Translate = (key: string) => string

const errorRules: Array<{ patterns: string[]; key: string }> = [
  { patterns: ['incorrect api key', 'invalid api-key', 'invalid_api_key', 'api key is required'], key: 'logs.error.invalidApiKey' },
	{ patterns: ['model is required', 'base url is required', 'only the configured openrouter provider is supported', 'openrouter api key is required'], key: 'logs.error.providerConfig' },
  { patterns: ['insufficient credits', 'status=402'], key: 'logs.error.insufficientCredits' },
  { patterns: ['status=429', 'rate limit'], key: 'logs.error.rateLimit' },
  { patterns: ['request timeout', 'deadline exceeded', 'timeout'], key: 'logs.error.timeout' },
  { patterns: ['no rows in result set', 'knowledge base not found'], key: 'logs.error.noRows' },
  { patterns: ['openrouter embedding request failed'], key: 'logs.error.openrouterEmbedding' },
  { patterns: ['task is no longer retryable', 'index task is not retryable'], key: 'logs.error.taskNotRetryable' },
  { patterns: ['unsupported task type'], key: 'logs.error.unsupportedTaskType' },
  { patterns: ['no document chunks found'], key: 'logs.error.noChunks' },
  { patterns: ['embedding vector count mismatch', 'embedding response count mismatch'], key: 'logs.error.embeddingMismatch' },
  { patterns: ['milvus vectors still exist'], key: 'logs.error.vectorDeleteIncomplete' },
  { patterns: ['unable to parse .doc', '无法解析 .doc'], key: 'logs.error.docConversionRequired' },
	{ patterns: ['has no readable content', '未提取到可读文字', 'slides not found', 'converted text is empty'], key: 'logs.error.noReadableContent' },
  { patterns: ['unsupported document type', '暂不支持该文件格式'], key: 'logs.error.unsupportedDocumentType' },
]

export function translateErrorMessage(errorMessage: string, t: Translate) {
  const message = errorMessage.trim()
  if (!message) {
    return ''
  }
  const normalized = message.toLowerCase()
  if (
    normalized.includes('pdf') &&
    (normalized.includes('加密') || normalized.includes('encryption version') || normalized.includes('filter /standard'))
  ) {
    return t('logs.error.encryptedPdf')
  }
  if (normalized.includes('未启用 ocr') || normalized.includes('ocr is not enabled') || normalized.includes('image-heavy')) {
    return t('logs.error.ocrRequired')
  }
  const matched = errorRules.find((rule) => rule.patterns.some((pattern) => normalized.includes(pattern)))
  return t(matched?.key ?? 'logs.error.unknown')
}

export function translateTaskMessage(rawMessage: string, t: Translate) {
	const message = rawMessage.trim()
	if (!message) {
		return '-'
	}
	if (message.toLowerCase().startsWith('parse retry scheduled')) {
		return t('logs.message.retryScheduled')
	}
	if (message.toLowerCase() === 'document parsed') {
		return t('logs.message.documentParsed')
	}
	return t('logs.message.unknown')
}
