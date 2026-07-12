import { Button, Card, Form, Input, message } from 'antd'
import axios from 'axios'
import { updatePassword } from '../../api/auth'
import { useI18n } from '../../useI18n'

type PasswordFormValues = {
  old_password: string
  new_password: string
  confirm_password: string
}

export function SettingsPasswordPage() {
  const [form] = Form.useForm<PasswordFormValues>()
  const { t } = useI18n()

  async function handleSubmit(values: PasswordFormValues) {
    try {
      await updatePassword(values)
      form.resetFields()
      message.success(t('settings.password.saveSuccess'))
    } catch (error) {
      if (axios.isAxiosError(error)) {
        const errorMessage = error.response?.data?.error ?? error.response?.data?.message ?? ''
        if (typeof errorMessage === 'string' && errorMessage.includes('old password is incorrect')) {
          form.setFields([{ name: 'old_password', errors: [t('settings.password.oldIncorrect')] }])
          return
        }
      }
      message.error(t('settings.password.saveFailed'))
    }
  }

  return (
    <div className="dashboard-page settings-page">
      <Card className="page-card">
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item
            label={t('settings.password.old')}
            name="old_password"
            rules={[{ required: true, message: t('settings.password.oldRequired') }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            label={t('settings.password.new')}
            name="new_password"
            rules={[{ required: true, message: t('settings.password.newRequired') }]}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item
            label={t('settings.password.confirm')}
            name="confirm_password"
            dependencies={['new_password']}
            rules={[
              { required: true, message: t('settings.password.confirmRequired') },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) {
                    return Promise.resolve()
                  }
                  return Promise.reject(new Error(t('settings.password.confirmMismatch')))
                },
              }),
            ]}
          >
            <Input.Password />
          </Form.Item>
          <Button type="primary" htmlType="submit">
            {t('common.save')}
          </Button>
        </Form>
      </Card>
    </div>
  )
}
