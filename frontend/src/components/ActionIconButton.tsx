import { Button } from 'antd'
import type { ReactNode } from 'react'

type ActionIconButtonProps = {
  icon: ReactNode
  label: string
  onClick?: () => void
  loading?: boolean
  disabled?: boolean
  effect?: 'refresh' | 'theme'
}

export function ActionIconButton({
  icon,
  label,
  onClick,
  loading,
  disabled,
  effect = 'refresh',
}: ActionIconButtonProps) {
  return (
    <Button
      className={`action-icon-button action-icon-button-${effect}`}
      type="text"
      icon={icon}
      aria-label={label}
      onClick={onClick}
      loading={loading}
      disabled={disabled}
    />
  )
}
