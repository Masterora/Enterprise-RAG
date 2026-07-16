export type ChatModelOption = {
  vendor: string
  seriesKey: string
  seriesLabel: string
  variantLabel: string
  performanceRank: number
  sortOrder: number
  label: string
  provider: string
  model: string
}

export type ChatModelCascaderOption = {
  label: string
  value: string
  children?: ChatModelCascaderOption[]
}

export type ChatModelMenuOption = {
  label: string
  value: string
}

function createOption(
  vendor: string,
  seriesKey: string,
  seriesLabel: string,
  variantLabel: string,
  performanceRank: number,
  provider: string,
  model: string,
): ChatModelOption {
  const sortOrder = nextSortOrder()
  const normalizedVariant = variantLabel.trim()
  return {
    vendor,
    seriesKey,
    seriesLabel,
    variantLabel: normalizedVariant,
    performanceRank,
    sortOrder,
    label: normalizedVariant ? `${seriesLabel} ${normalizedVariant}` : seriesLabel,
    provider,
    model,
  }
}

let chatModelSortOrder = 0

function nextSortOrder() {
  chatModelSortOrder += 1
  return chatModelSortOrder
}

const OPENAI_MODELS: ChatModelOption[] = [
  createOption('OpenAI', 'openai-gpt-5.6-sol', 'GPT-5.6 Sol', 'Pro', 1, 'openrouter', 'openai/gpt-5.6-sol-pro'),
  createOption('OpenAI', 'openai-gpt-5.6-sol', 'GPT-5.6 Sol', '', 1, 'openrouter', 'openai/gpt-5.6-sol'),
  createOption('OpenAI', 'openai-gpt-5.6-terra', 'GPT-5.6 Terra', 'Pro', 2, 'openrouter', 'openai/gpt-5.6-terra-pro'),
  createOption('OpenAI', 'openai-gpt-5.6-terra', 'GPT-5.6 Terra', '', 3, 'openrouter', 'openai/gpt-5.6-terra'),
  createOption('OpenAI', 'openai-gpt-5.6-luna', 'GPT-5.6 Luna', 'Pro', 4, 'openrouter', 'openai/gpt-5.6-luna-pro'),
  createOption('OpenAI', 'openai-gpt-5.6-luna', 'GPT-5.6 Luna', '', 5, 'openrouter', 'openai/gpt-5.6-luna'),
  createOption('OpenAI', 'openai-gpt-5.5', 'GPT-5.5', 'Pro', 1, 'openrouter', 'openai/gpt-5.5-pro'),
  createOption('OpenAI', 'openai-gpt-5.5', 'GPT-5.5', '', 2, 'openrouter', 'openai/gpt-5.5'),
  createOption('OpenAI', 'openai-gpt-5', 'GPT-5', 'Pro', 1, 'openrouter', 'openai/gpt-5-pro'),
  createOption('OpenAI', 'openai-gpt-5.4', 'GPT-5.4', 'Pro', 1, 'openrouter', 'openai/gpt-5.4-pro'),
  createOption('OpenAI', 'openai-gpt-5.4', 'GPT-5.4', '', 2, 'openrouter', 'openai/gpt-5.4'),
  createOption('OpenAI', 'openai-gpt-5.4', 'GPT-5.4', 'Mini', 3, 'openrouter', 'openai/gpt-5.4-mini'),
  createOption('OpenAI', 'openai-gpt-5.4', 'GPT-5.4', 'Nano', 4, 'openrouter', 'openai/gpt-5.4-nano'),
  createOption('OpenAI', 'openai-gpt-5.3', 'GPT-5.3', 'Chat', 2, 'openrouter', 'openai/gpt-5.3-chat'),
  createOption('OpenAI', 'openai-gpt-5.3', 'GPT-5.3', 'Codex', 3, 'openrouter', 'openai/gpt-5.3-codex'),
  createOption('OpenAI', 'openai-gpt-5.2', 'GPT-5.2', 'Pro', 1, 'openrouter', 'openai/gpt-5.2-pro'),
  createOption('OpenAI', 'openai-gpt-5.2', 'GPT-5.2', '', 2, 'openrouter', 'openai/gpt-5.2'),
  createOption('OpenAI', 'openai-gpt-5.2', 'GPT-5.2', 'Chat', 3, 'openrouter', 'openai/gpt-5.2-chat'),
  createOption('OpenAI', 'openai-gpt-5.1', 'GPT-5.1', '', 2, 'openrouter', 'openai/gpt-5.1'),
  createOption('OpenAI', 'openai-gpt-5.1', 'GPT-5.1', 'Chat', 3, 'openrouter', 'openai/gpt-5.1-chat'),
  createOption('OpenAI', 'openai-gpt-5.1', 'GPT-5.1', 'Codex Max', 3, 'openrouter', 'openai/gpt-5.1-codex-max'),
  createOption('OpenAI', 'openai-gpt-4o', 'GPT-4o', '', 2, 'openrouter', 'openai/gpt-4o'),
  createOption('OpenAI', 'openai-gpt-4o', 'GPT-4o', 'Mini', 3, 'openrouter', 'openai/gpt-4o-mini'),
  createOption('OpenAI', 'openai-gpt-4.1', 'GPT-4.1', '', 2, 'openrouter', 'openai/gpt-4.1'),
  createOption('OpenAI', 'openai-gpt-4.1', 'GPT-4.1', 'Mini', 3, 'openrouter', 'openai/gpt-4.1-mini'),
]

const ANTHROPIC_MODELS: ChatModelOption[] = [
  createOption('Anthropic', 'anthropic-claude-fable-5', 'Claude Fable 5', '', 1, 'openrouter', 'anthropic/claude-fable-5'),
  createOption('Anthropic', 'anthropic-claude-opus-4.8', 'Claude Opus 4.8', '', 1, 'openrouter', 'anthropic/claude-opus-4.8'),
  createOption('Anthropic', 'anthropic-claude-opus-4.8', 'Claude Opus 4.8', 'Fast', 2, 'openrouter', 'anthropic/claude-opus-4.8-fast'),
  createOption('Anthropic', 'anthropic-claude-opus-4.7', 'Claude Opus 4.7', '', 1, 'openrouter', 'anthropic/claude-opus-4.7'),
  createOption('Anthropic', 'anthropic-claude-opus-4.7', 'Claude Opus 4.7', 'Fast', 2, 'openrouter', 'anthropic/claude-opus-4.7-fast'),
  createOption('Anthropic', 'anthropic-claude-opus-4.6', 'Claude Opus 4.6', '', 1, 'openrouter', 'anthropic/claude-opus-4.6'),
  createOption('Anthropic', 'anthropic-claude-opus-4.6', 'Claude Opus 4.6', 'Fast', 2, 'openrouter', 'anthropic/claude-opus-4.6-fast'),
  createOption('Anthropic', 'anthropic-claude-opus-4.5', 'Claude Opus 4.5', '', 2, 'openrouter', 'anthropic/claude-opus-4.5'),
  createOption('Anthropic', 'anthropic-claude-opus-4.1', 'Claude Opus 4.1', '', 3, 'openrouter', 'anthropic/claude-opus-4.1'),
  createOption('Anthropic', 'anthropic-claude-opus-4', 'Claude Opus 4', '', 4, 'openrouter', 'anthropic/claude-opus-4'),
  createOption('Anthropic', 'anthropic-claude-sonnet-5', 'Claude Sonnet 5', '', 2, 'openrouter', 'anthropic/claude-sonnet-5'),
  createOption('Anthropic', 'anthropic-claude-sonnet-4.6', 'Claude Sonnet 4.6', '', 2, 'openrouter', 'anthropic/claude-sonnet-4.6'),
  createOption('Anthropic', 'anthropic-claude-sonnet-4.5', 'Claude Sonnet 4.5', '', 2, 'openrouter', 'anthropic/claude-sonnet-4.5'),
  createOption('Anthropic', 'anthropic-claude-sonnet-4', 'Claude Sonnet 4', '', 2, 'openrouter', 'anthropic/claude-sonnet-4'),
  createOption('Anthropic', 'anthropic-claude-3.7-sonnet', 'Claude 3.7 Sonnet', '', 3, 'openrouter', 'anthropic/claude-3.7-sonnet'),
  createOption('Anthropic', 'anthropic-claude-haiku-4.5', 'Claude Haiku 4.5', '', 4, 'openrouter', 'anthropic/claude-haiku-4.5'),
]

const GOOGLE_MODELS: ChatModelOption[] = [
  createOption('Google', 'google-gemini-3.1', 'Gemini 3.1', 'Pro Preview', 1, 'openrouter', 'google/gemini-3.1-pro-preview'),
  createOption('Google', 'google-gemini-2.5', 'Gemini 2.5', 'Pro', 1, 'openrouter', 'google/gemini-2.5-pro'),
  createOption('Google', 'google-gemini-2.5', 'Gemini 2.5', 'Flash Lite', 2, 'openrouter', 'google/gemini-2.5-flash-lite'),
  createOption('Google', 'google-gemini-3.5', 'Gemini 3.5', 'Flash', 3, 'openrouter', 'google/gemini-3.5-flash'),
  createOption('Google', 'google-gemini-2.5', 'Gemini 2.5', 'Flash', 3, 'openrouter', 'google/gemini-2.5-flash'),
  createOption('Google', 'google-gemini-3.1', 'Gemini 3.1', 'Flash Lite', 4, 'openrouter', 'google/gemini-3.1-flash-lite'),
]

const DEEPSEEK_MODELS: ChatModelOption[] = [
  createOption('DeepSeek', 'deepseek-v4', 'DeepSeek V4', 'Pro', 1, 'openrouter', 'deepseek/deepseek-v4-pro'),
  createOption('DeepSeek', 'deepseek-v4', 'DeepSeek V4', 'Flash', 3, 'openrouter', 'deepseek/deepseek-v4-flash'),
  createOption('DeepSeek', 'deepseek-r1', 'DeepSeek R1', '', 2, 'openrouter', 'deepseek/deepseek-r1'),
  createOption('DeepSeek', 'deepseek-v3.2', 'DeepSeek V3.2', '', 3, 'openrouter', 'deepseek/deepseek-v3.2'),
  createOption('DeepSeek', 'deepseek-v3', 'DeepSeek V3', '', 4, 'openrouter', 'deepseek/deepseek-chat-v3-0324'),
]

const QWEN_MODELS: ChatModelOption[] = [
  createOption('Qwen', 'qwen-3.7', 'Qwen 3.7', 'Max', 1, 'openrouter', 'qwen/qwen3.7-max'),
  createOption('Qwen', 'qwen-3.7', 'Qwen 3.7', 'Plus', 2, 'openrouter', 'qwen/qwen3.7-plus'),
  createOption('Qwen', 'qwen-3.6', 'Qwen 3.6', 'Max Preview', 1, 'openrouter', 'qwen/qwen3.6-max-preview'),
  createOption('Qwen', 'qwen-3.6', 'Qwen 3.6', 'Plus', 2, 'openrouter', 'qwen/qwen3.6-plus'),
  createOption('Qwen', 'qwen-3-max', 'Qwen 3 Max', '', 1, 'openrouter', 'qwen/qwen3-max'),
  createOption('Qwen', 'qwen-plus', 'Qwen Plus', '2025-07-28', 2, 'openrouter', 'qwen/qwen-plus-2025-07-28'),
  createOption('Qwen', 'qwen-coder', 'Qwen3 Coder', 'Plus', 1, 'openrouter', 'qwen/qwen3-coder-plus'),
  createOption('Qwen', 'qwen-coder', 'Qwen3 Coder', 'Flash', 3, 'openrouter', 'qwen/qwen3-coder-flash'),
  createOption('Qwen', 'qwen-235b', 'Qwen3 235B', '', 3, 'openrouter', 'qwen/qwen3-235b-a22b-2507'),
  createOption('Qwen', 'qwen-coder', 'Qwen3 Coder', '', 4, 'openrouter', 'qwen/qwen3-coder'),
]

const MOONSHOT_MODELS: ChatModelOption[] = [
  createOption('Moonshot', 'moonshot-kimi-k2.6', 'Kimi K2.6', '', 1, 'openrouter', 'moonshotai/kimi-k2.6'),
  createOption('Moonshot', 'moonshot-kimi-k2.5', 'Kimi K2.5', '', 2, 'openrouter', 'moonshotai/kimi-k2.5'),
  createOption('Moonshot', 'moonshot-kimi-k2', 'Kimi K2', 'Thinking', 1, 'openrouter', 'moonshotai/kimi-k2-thinking'),
  createOption('Moonshot', 'moonshot-kimi-k2', 'Kimi K2', '', 2, 'openrouter', 'moonshotai/kimi-k2'),
]

const XAI_MODELS: ChatModelOption[] = [
  createOption('xAI', 'xai-grok-4.20', 'Grok 4.20', '', 1, 'openrouter', 'x-ai/grok-4.20'),
  createOption('xAI', 'xai-grok-4.20', 'Grok 4.20', 'Multi-Agent', 2, 'openrouter', 'x-ai/grok-4.20-multi-agent'),
  createOption('xAI', 'xai-grok-4.3', 'Grok 4.3', '', 2, 'openrouter', 'x-ai/grok-4.3'),
]

const GLM_MODELS: ChatModelOption[] = [
  createOption('GLM', 'glm-5.2', 'GLM 5.2', '', 1, 'openrouter', 'z-ai/glm-5.2'),
  createOption('GLM', 'glm-5.1', 'GLM 5.1', '', 2, 'openrouter', 'z-ai/glm-5.1'),
  createOption('GLM', 'glm-5', 'GLM 5', '', 3, 'openrouter', 'z-ai/glm-5'),
]

const MISTRAL_MODELS: ChatModelOption[] = [
  createOption('Mistral', 'mistral-large-2512', 'Mistral Large 2512', '', 1, 'openrouter', 'mistralai/mistral-large-2512'),
  createOption('Mistral', 'mistral-medium-3.5', 'Mistral Medium 3.5', '', 2, 'openrouter', 'mistralai/mistral-medium-3-5'),
  createOption('Mistral', 'mistral-small-2603', 'Mistral Small 2603', '', 3, 'openrouter', 'mistralai/mistral-small-2603'),
]

export const CHAT_MODEL_OPTIONS: ChatModelOption[] = [
  ...OPENAI_MODELS,
  ...ANTHROPIC_MODELS,
  ...GOOGLE_MODELS,
  ...DEEPSEEK_MODELS,
  ...QWEN_MODELS,
  ...MOONSHOT_MODELS,
  ...XAI_MODELS,
  ...GLM_MODELS,
  ...MISTRAL_MODELS,
]

export const DEFAULT_CHAT_MODEL =
  CHAT_MODEL_OPTIONS.find(
    (option) => option.provider === 'openrouter' && option.model === 'openai/gpt-5.6-sol',
  ) ?? CHAT_MODEL_OPTIONS[0]

function sortOptions(options: ChatModelOption[]) {
  return [...options].sort(comparePerformance)
}

function findBestVendorOption(seriesMap?: Map<string, ChatModelOption[]>) {
  if (!seriesMap) {
    return undefined
  }
  const allOptions = Array.from(seriesMap.values()).flat()
  return sortOptions(allOptions)[0]
}

function getVendorSeriesMap(vendor: string) {
  const seriesMap = new Map<string, ChatModelOption[]>()
  for (const option of CHAT_MODEL_OPTIONS) {
    if (option.vendor !== vendor) {
      continue
    }
    const list = seriesMap.get(option.seriesKey) ?? []
    list.push(option)
    seriesMap.set(option.seriesKey, list)
  }
  return seriesMap
}

function getAllVendorSeriesMaps() {
  const vendorMap = new Map<string, Map<string, ChatModelOption[]>>()
  for (const option of CHAT_MODEL_OPTIONS) {
    const seriesMap = vendorMap.get(option.vendor) ?? new Map<string, ChatModelOption[]>()
    const list = seriesMap.get(option.seriesKey) ?? []
    list.push(option)
    seriesMap.set(option.seriesKey, list)
    vendorMap.set(option.vendor, seriesMap)
  }
  return vendorMap
}

function comparePerformance(left: ChatModelOption, right: ChatModelOption) {
  return left.sortOrder - right.sortOrder
}

export function getVendorOptions(): ChatModelMenuOption[] {
  const vendorMap = getAllVendorSeriesMaps()
  return Array.from(vendorMap.entries())
    .sort((left, right) => comparePerformance(findBestVendorOption(left[1])!, findBestVendorOption(right[1])!))
    .map(([vendor]) => ({
      label: vendor,
      value: vendor,
    }))
}

export function getBestVendorModel(vendor: string) {
  const best = findBestVendorOption(getVendorSeriesMap(vendor))
  return best ?? DEFAULT_CHAT_MODEL
}

export function getVendorModelCascaderOptions(vendor: string): ChatModelCascaderOption[] {
  const seriesMap = getVendorSeriesMap(vendor)
  return Array.from(seriesMap.entries())
    .map(([seriesKey, options]) => {
      const sorted = sortOptions(options)
      const best = sorted[0]
      if (sorted.length === 1) {
        return {
          best,
          label: best.label,
          value: buildChatModelValue(best.provider, best.model),
        }
      }
      return {
        best,
        label: best.seriesLabel,
        value: seriesKey,
        children: sorted.map((item) => ({
          label: item.label,
          value: buildChatModelValue(item.provider, item.model),
        })),
      }
    })
    .sort((left, right) => comparePerformance(left.best, right.best))
    .map(({ best: _best, ...option }) => option)
}

export function buildChatModelValue(provider: string, model: string) {
  return `${provider}::${model}`
}

export function parseChatModelValue(value: string) {
  const [provider, model] = value.split('::')
  return { provider: provider ?? '', model: model ?? '' }
}
