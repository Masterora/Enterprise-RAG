import {
  Alert,
  App as AntdApp,
  Button,
  Card,
  Descriptions,
  Input,
  Space,
  Tag,
  Typography,
} from 'antd'
import { useEffect, useState } from 'react'
import {
  getModelServiceSettings,
  updateModelServiceSettings,
  type ModelServiceSettings,
} from '../../api/settings'
import { useI18n } from '../../useI18n'

export function ModelSettingsPage() {
  const { message } = AntdApp.useApp()
  const { t } = useI18n()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [apiKey, setAPIKey] = useState('')
  const [settings, setSettings] = useState<ModelServiceSettings | null>(null)

  useEffect(() => {
    async function loadSettings() {
      setLoading(true)
      try {
        setSettings(await getModelServiceSettings())
      } catch {
        message.error(t('settings.model.loadFailed'))
      } finally {
        setLoading(false)
      }
    }

    void loadSettings()
  }, [message, t])

  async function saveAPIKey() {
    const value = apiKey.trim()
    if (!value) {
      return
    }
    setSaving(true)
    try {
      setSettings(await updateModelServiceSettings(value))
      setAPIKey('')
      message.success(t('settings.model.saveSuccess'))
    } catch {
      message.error(t('settings.model.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  const configured = settings?.api_key_configured ?? false
  const source = settings?.configuration_source ?? 'none'

  return (
    <div className="dashboard-page settings-page model-settings-page">
      <Card
        className="page-card model-key-card"
        loading={loading}
        title={t('settings.model.apiKeyTitle')}
        extra={
          settings ? (
            <Tag color={configured ? 'success' : 'warning'}>
              {t(configured ? 'settings.model.configured' : 'settings.model.missing')}
            </Tag>
          ) : null
        }
      >
        <Space direction="vertical" size={16} className="model-settings-content">
          <div className="model-key-form">
            <Input.Password
              value={apiKey}
              onChange={(event) => setAPIKey(event.target.value)}
              onPressEnter={() => void saveAPIKey()}
              placeholder={t('settings.model.apiKeyPlaceholder')}
              autoComplete="new-password"
              disabled={saving}
            />
            <Button
              type="primary"
              loading={saving}
              disabled={!apiKey.trim()}
              onClick={() => void saveAPIKey()}
            >
              {t('settings.model.saveAndVerify')}
            </Button>
          </div>

          <Alert
            showIcon
            type={configured ? 'success' : 'warning'}
            message={t(
              configured
                ? 'settings.model.configuredMessage'
                : 'settings.model.missingMessage',
            )}
          />

          <Typography.Link href="https://openrouter.ai/keys" target="_blank" rel="noreferrer">
            {t('settings.model.openRouterKeys')}
          </Typography.Link>
        </Space>
      </Card>

      <Card className="page-card" loading={loading} title={t('settings.model.runtimeTitle')}>
        {settings ? (
          <Descriptions
            column={{ xs: 1, sm: 1, md: 2 }}
            size="small"
            className="model-settings-details"
          >
            <Descriptions.Item label={t('settings.model.provider')}>
              {settings.provider}
            </Descriptions.Item>
            <Descriptions.Item label={t('settings.model.configurationSource')}>
              {t(`settings.model.source.${source}`)}
            </Descriptions.Item>
            <Descriptions.Item label={t('settings.model.embeddingModel')}>
              {settings.embedding_model}
            </Descriptions.Item>
            <Descriptions.Item label={t('settings.model.dimension')}>
              {t('settings.model.dimensionValue', {
                count: settings.embedding_dimension,
              })}
            </Descriptions.Item>
            <Descriptions.Item label={t('settings.model.apiKeyHint')}>
              {settings.api_key_hint || '-'}
            </Descriptions.Item>
            <Descriptions.Item label={t('settings.model.baseURL')} span={2}>
              <Typography.Text code>{settings.base_url}</Typography.Text>
            </Descriptions.Item>
          </Descriptions>
        ) : null}
      </Card>
    </div>
  )
}
