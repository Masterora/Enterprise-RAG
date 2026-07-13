import { App as AntdApp, Button, Card, Form, Input, Select, Typography } from 'antd'
import { useCallback, useEffect, useState } from 'react'
import { getMe, updateUser, type UserInfo } from '../../api/auth'
import { useI18n, type Language } from '../../useI18n'
import { isSupportedLanguage, LANGUAGE_LABELS } from '../../i18n-core'

type SettingsFormValues = {
  nickname?: string
  email?: string
  language: Language
  username: string
}

export function SettingsPage() {
	const { message } = AntdApp.useApp()
  const [form] = Form.useForm<SettingsFormValues>()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const { t, setLanguage } = useI18n()

  const applyUser = useCallback(
    (user: UserInfo) => {
      form.setFieldsValue({
        username: user.username,
        nickname: user.nickname,
        email: user.email,
        language: (user.language as Language) || 'zh-CN',
      })
      if (isSupportedLanguage(user.language)) {
        setLanguage(user.language)
      }
    },
    [form, setLanguage],
  )

  useEffect(() => {
    async function loadProfile() {
      setLoading(true)
      try {
        const user = await getMe()
        applyUser(user)
      } catch {
        message.error(t('settings.loadFailed'))
      } finally {
        setLoading(false)
      }
    }

    void loadProfile()
  }, [applyUser, message, t])

  async function handleSubmit(values: SettingsFormValues) {
    setSaving(true)
    try {
      const user = await updateUser({
        nickname: values.nickname,
        email: values.email,
        language: values.language,
      })
      applyUser(user)
      message.success(t('settings.saveSuccess'))
    } catch {
      message.error(t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="dashboard-page settings-page">
      <Card className="page-card">
        <Form form={form} layout="vertical" onFinish={handleSubmit} disabled={loading}>
          <Form.Item label={t('settings.username')}>
            <Typography.Text>{form.getFieldValue('username') || '-'}</Typography.Text>
          </Form.Item>
          <Form.Item
            label={t('settings.nickname')}
            name="nickname"
            required
            rules={[{ required: true, message: t('settings.nicknameRequired') }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={t('settings.email')}
            name="email"
            required
            rules={[
              { required: true, message: t('settings.emailRequired') },
              { type: 'email', message: t('settings.emailInvalid') },
            ]}
          >
            <Input />
          </Form.Item>
          <Form.Item label={t('settings.language')} name="language" initialValue="zh-CN">
            <Select
              options={[
                { label: LANGUAGE_LABELS['zh-CN'], value: 'zh-CN' },
                { label: LANGUAGE_LABELS['en-US'], value: 'en-US' },
                { label: LANGUAGE_LABELS['ja-JP'], value: 'ja-JP' },
              ]}
            />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saving}>
            {t('common.save')}
          </Button>
        </Form>
      </Card>
    </div>
  )
}
