import { App as AntdApp, Button, Dropdown, Input, Modal, Space, type MenuProps } from 'antd'
import { CheckOutlined, CloseOutlined, DeleteOutlined, EditOutlined, MessageOutlined, PlusOutlined } from '@ant-design/icons'
import { useI18n } from '../../../useI18n'
import type { SubjectInfo } from '../../../api/subjects'
import type { ChatSession } from '../types'

type SessionListProps = {
  subjects: SubjectInfo[]
  sessions: ChatSession[]
  activeSessionID: string
  editingSessionID: string
  editingTitle: string
  onEditingTitleChange: (value: string) => void
  onCreateSession: () => void
  onSelectSession: (sessionID: string) => void
  onRenameSession: (session: ChatSession) => void
  onConfirmRenameSession: (sessionID: string) => void
  onCancelRenameSession: () => void
  onDeleteSession: (sessionID: string) => void
}

export function SessionList({
  subjects,
  sessions,
  activeSessionID,
  editingSessionID,
  editingTitle,
  onEditingTitleChange,
  onCreateSession,
  onSelectSession,
  onRenameSession,
  onConfirmRenameSession,
  onCancelRenameSession,
  onDeleteSession,
}: SessionListProps) {
	const { message } = AntdApp.useApp()
  const { t } = useI18n()

  function confirmDelete(sessionID: string) {
    Modal.confirm({
      title: t('chat.deleteSessionConfirm'),
      okText: t('common.confirm'),
      cancelText: t('common.cancel'),
      onOk: () => onDeleteSession(sessionID),
    })
  }

  function handleConfirmRename(sessionID: string) {
    if (!editingTitle.trim()) {
      message.warning(t('chat.renameEmpty'))
      return
    }
    onConfirmRenameSession(sessionID)
  }

  return (
    <aside className="chat-session-pane">
      <Button block icon={<PlusOutlined />} onClick={onCreateSession}>
        {t('chat.newSession')}
      </Button>
      <div className="chat-session-list">
        {sessions.length === 0 ? (
          <div className="chat-session-empty">
            <MessageOutlined />
            <span>{t('chat.noSessions')}</span>
          </div>
        ) : (
          sessions.map((session) => {
            const subject = subjects.find((item) => item.id === session.subjectID)
            const selected = session.id === activeSessionID
            const isEditing = session.id === editingSessionID
            const menuItems: MenuProps['items'] = [
              {
                key: 'rename',
                icon: <EditOutlined />,
                label: t('chat.renameSession'),
              },
              {
                key: 'delete',
                icon: <DeleteOutlined />,
                label: t('chat.deleteSession'),
                danger: true,
              },
            ]

            return (
              <Dropdown
                key={session.id}
                trigger={isEditing ? [] : ['contextMenu']}
                menu={{
                  items: menuItems,
                  onClick: ({ key }) => {
                    if (key === 'rename') {
                      onRenameSession(session)
                      return
                    }
                    if (key === 'delete') {
                      confirmDelete(session.id)
                    }
                  },
                }}
              >
                <div
                  className={`chat-session-item ${selected ? 'is-active' : ''}`}
                  onClick={() => onSelectSession(session.id)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      onSelectSession(session.id)
                    }
                  }}
                  role="button"
                  tabIndex={0}
                >
                  <div className="chat-session-item-row">
                    {isEditing ? (
                      <Input
                        value={editingTitle}
                        autoFocus
                        size="small"
                        maxLength={60}
                        onChange={(event) => onEditingTitleChange(event.target.value)}
                        onClick={(event) => event.stopPropagation()}
                        onPressEnter={() => handleConfirmRename(session.id)}
                      />
                    ) : (
                      <strong>{session.title}</strong>
                    )}
                    {isEditing && (
                      <Space size={4} className="chat-session-actions" onClick={(event) => event.stopPropagation()}>
                        <Button
                          type="text"
                          size="small"
                          icon={<CheckOutlined />}
                          aria-label={t('chat.renameSession')}
                          onClick={() => handleConfirmRename(session.id)}
                        />
                        <Button
                          type="text"
                          size="small"
                          icon={<CloseOutlined />}
                          aria-label={t('common.cancel')}
                          onClick={onCancelRenameSession}
                        />
                      </Space>
                    )}
                  </div>
                  <span>{subject?.name || t('common.unknownDocument')}</span>
                </div>
              </Dropdown>
            )
          })
        )}
      </div>
    </aside>
  )
}
