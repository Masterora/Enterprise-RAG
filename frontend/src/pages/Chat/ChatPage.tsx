import { Button, Input, Select, message } from 'antd'
import { SendOutlined } from '@ant-design/icons'
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type UIEvent } from 'react'
import { streamChat } from '../../api/chat'
import { listSubjects, type SubjectInfo } from '../../api/subjects'
import { useI18n } from '../../useI18n'
import { ConversationTurn } from './components/ConversationTurn'
import { SessionList } from './components/SessionList'
import type { ChatMessage, ChatSession } from './types'
import { buildSessionTitle, CHAT_STORAGE_KEY, loadStoredSessions } from './utils'

const MAX_ASK_ATTEMPTS = 3
const ASK_TIMEOUT_MS = 30_000
const DEFAULT_CHAT_TOP_K = 5
const CHAT_TOP_K = Math.max(1, Number(import.meta.env.VITE_CHAT_TOP_K ?? DEFAULT_CHAT_TOP_K))

export function ChatPage() {
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [subjectID, setSubjectID] = useState('')
  const [question, setQuestion] = useState('')
  const [sessions, setSessions] = useState<ChatSession[]>(() => loadStoredSessions())
  const [activeSessionID, setActiveSessionID] = useState('')
  const [editingSessionID, setEditingSessionID] = useState('')
  const [editingTitle, setEditingTitle] = useState('')
  const [asking, setAsking] = useState(false)
  const chatScrollRef = useRef<HTMLDivElement | null>(null)
  const autoFollowRef = useRef(true)
  const { t } = useI18n()

  const orderedSessions = useMemo(() => [...sessions].sort((a, b) => b.updatedAt - a.updatedAt), [sessions])
  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionID) ?? null,
    [activeSessionID, sessions],
  )
  const subjectOptions = useMemo(
    () => subjects.map((subject) => ({ label: subject.name, value: subject.id })),
    [subjects],
  )
  const messages = activeSession?.messages ?? []

  useEffect(() => {
    window.localStorage.setItem(CHAT_STORAGE_KEY, JSON.stringify(sessions))
  }, [sessions])

  useEffect(() => {
    async function loadSubjects() {
      try {
        const data = await listSubjects({ page: 1, page_size: 100 })
        setSubjects(data.list)
      } catch {
        message.error(t('chat.loadSubjectsFailed'))
      }
    }

    void loadSubjects()
  }, [t])

  useEffect(() => {
    if (sessions.length === 0 || activeSessionID) {
      return
    }
    setActiveSessionID(sessions[0].id)
  }, [activeSessionID, sessions])

  useEffect(() => {
    if (subjects.length === 0) {
      setSubjectID('')
      return
    }
    if (activeSession?.subjectID) {
      setSubjectID(activeSession.subjectID)
      return
    }
    setSubjectID((current) => {
      if (current && subjects.some((subject) => subject.id === current)) {
        return current
      }
      return subjects[0].id
    })
  }, [activeSession?.subjectID, subjects])

  useEffect(() => {
    autoFollowRef.current = true
  }, [activeSessionID])

  useEffect(() => {
    if (!chatScrollRef.current || !autoFollowRef.current) {
      return
    }
    chatScrollRef.current.scrollTo({
      top: chatScrollRef.current.scrollHeight,
      behavior: 'smooth',
    })
  }, [activeSession?.id, activeSession?.messages])

  function updateSession(sessionID: string, updater: (session: ChatSession) => ChatSession) {
    setSessions((current) => current.map((session) => (session.id === sessionID ? updater(session) : session)))
  }

  function createSession(nextSubjectID?: string) {
    const timestamp = Date.now()
    const session: ChatSession = {
      id: crypto.randomUUID(),
      title: t('chat.newSession'),
      subjectID: nextSubjectID ?? subjectID,
      createdAt: timestamp,
      updatedAt: timestamp,
      messages: [],
    }
    setSessions((current) => [session, ...current])
    setActiveSessionID(session.id)
    setSubjectID(session.subjectID)
    return session
  }

  function ensureActiveSession() {
    return activeSession ?? createSession(subjectID)
  }

  function startRenameSession(session: ChatSession) {
    setEditingSessionID(session.id)
    setEditingTitle(session.title)
  }

  function confirmRenameSession(sessionID: string) {
    const nextTitle = editingTitle.trim()
    updateSession(sessionID, (session) => ({
      ...session,
      title: nextTitle,
      updatedAt: Date.now(),
    }))
    setEditingSessionID('')
    setEditingTitle('')
  }

  function cancelRenameSession() {
    setEditingSessionID('')
    setEditingTitle('')
  }

  function deleteSession(sessionID: string) {
    const nextSessions = orderedSessions.filter((session) => session.id !== sessionID)
    setSessions((current) => current.filter((session) => session.id !== sessionID))
    if (activeSessionID === sessionID) {
      const nextActiveSession = nextSessions[0]
      setActiveSessionID(nextActiveSession?.id ?? '')
      setSubjectID(nextActiveSession?.subjectID ?? subjects[0]?.id ?? '')
    }
    if (editingSessionID === sessionID) {
      cancelRenameSession()
    }
  }

  function updateMessage(sessionID: string, messageID: string, patch: Partial<ChatMessage>) {
    updateSession(sessionID, (session) => ({
      ...session,
      updatedAt: Date.now(),
      messages: session.messages.map((item) => (item.id === messageID ? { ...item, ...patch } : item)),
    }))
  }

  function appendAnswer(sessionID: string, messageID: string, content: string) {
    updateSession(sessionID, (session) => ({
      ...session,
      updatedAt: Date.now(),
      messages: session.messages.map((item) =>
        item.id === messageID ? { ...item, answer: item.answer + content } : item,
      ),
    }))
  }

  async function handleAsk() {
    const trimmedQuestion = question.trim()
    if (!subjectID) {
      message.warning(t('chat.selectSubjectWarning'))
      return
    }
    if (!trimmedQuestion) {
      message.warning(t('chat.inputWarning'))
      return
    }

    const session = ensureActiveSession()
    const messageID = crypto.randomUUID()
    const nextTimestamp = Date.now()
    setQuestion('')
    setAsking(true)

    updateSession(session.id, (current) => ({
      ...current,
      title: current.messages.length === 0 ? buildSessionTitle(trimmedQuestion) : current.title,
      subjectID,
      updatedAt: nextTimestamp,
      messages: [
        ...current.messages,
        {
          id: messageID,
          question: trimmedQuestion,
          answer: '',
          status: t('chat.preparing'),
          chunks: [],
          loading: true,
        },
      ],
    }))

    try {
      await askWithRetry(session.id, messageID, trimmedQuestion, subjectID)
    } catch {
      updateMessage(session.id, messageID, {
        status: t('chat.failed'),
        loading: false,
      })
      message.error(t('chat.failedToast'))
    } finally {
      setAsking(false)
    }
  }

  async function askWithRetry(sessionID: string, messageID: string, currentQuestion: string, currentSubjectID: string) {
    let lastError: unknown

    for (let attempt = 1; attempt <= MAX_ASK_ATTEMPTS; attempt += 1) {
      const controller = new AbortController()
      let hasAnswer = false
      let timeout = window.setTimeout(() => controller.abort(), ASK_TIMEOUT_MS)
      let settleTimeout = 0
      const resetTimeout = () => {
        window.clearTimeout(timeout)
        timeout = window.setTimeout(() => controller.abort(), ASK_TIMEOUT_MS)
      }
      const resetSettleTimeout = () => {
        if (!hasAnswer) {
          return
        }
        window.clearTimeout(settleTimeout)
        settleTimeout = window.setTimeout(() => {
          controller.abort()
        }, 2500)
      }

      try {
        if (attempt > 1) {
          updateMessage(sessionID, messageID, {
            answer: '',
            chunks: [],
            status: t('chat.retrying', { attempt }),
            loading: true,
          })
        }

        await streamChat(
          {
            subject_id: currentSubjectID,
            query: currentQuestion,
            top_k: CHAT_TOP_K,
          },
          {
            onEvent: resetTimeout,
            onStatus: (status) => updateMessage(sessionID, messageID, { status }),
            onSources: (chunks) => updateMessage(sessionID, messageID, { chunks }),
            onDelta: (content) => {
              hasAnswer = true
              resetSettleTimeout()
              appendAnswer(sessionID, messageID, content)
            },
            onDone: () => {
              window.clearTimeout(settleTimeout)
              updateMessage(sessionID, messageID, { status: t('chat.done'), loading: false })
            },
            onError: () => {
              window.clearTimeout(settleTimeout)
              updateMessage(sessionID, messageID, { status: t('chat.retryPrepare') })
            },
          },
          controller.signal,
        )
        window.clearTimeout(timeout)
        window.clearTimeout(settleTimeout)
        return
      } catch (error) {
        window.clearTimeout(timeout)
        window.clearTimeout(settleTimeout)
        if (hasAnswer) {
          updateMessage(sessionID, messageID, { status: t('chat.done'), loading: false })
          return
        }
        lastError = error
        if (attempt < MAX_ASK_ATTEMPTS) {
          updateMessage(sessionID, messageID, {
            status: t('chat.retryNotice', { attempt: attempt + 1 }),
          })
          continue
        }
      }
    }

    throw lastError
  }

  function handleInputKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) {
      return
    }
    event.preventDefault()
    void handleAsk()
  }

  function handleSubjectChange(value: string) {
    setSubjectID(value)
    if (!activeSession) {
      return
    }
    updateSession(activeSession.id, (session) => ({ ...session, subjectID: value }))
  }

  function handleChatScroll(event: UIEvent<HTMLDivElement>) {
    const container = event.currentTarget
    const remaining = container.scrollHeight - container.scrollTop - container.clientHeight
    autoFollowRef.current = remaining < 48
  }

  return (
    <div className="chat-page">
      <SessionList
        subjects={subjects}
        sessions={orderedSessions}
        activeSessionID={activeSessionID}
        editingSessionID={editingSessionID}
        editingTitle={editingTitle}
        onEditingTitleChange={setEditingTitle}
        onCreateSession={() => createSession(subjectID || subjects[0]?.id || '')}
        onSelectSession={setActiveSessionID}
        onRenameSession={startRenameSession}
        onConfirmRenameSession={confirmRenameSession}
        onCancelRenameSession={cancelRenameSession}
        onDeleteSession={deleteSession}
      />

      <section className="chat-main">
        <div className="chat-toolbar">
          <Select
            className="chat-subject-select"
            placeholder={t('chat.selectSubject')}
            value={subjectID || undefined}
            options={subjectOptions}
            onChange={handleSubjectChange}
          />
        </div>

        {messages.length === 0 ? (
          <div className="chat-scroll" ref={chatScrollRef} onScroll={handleChatScroll}>
            <div className="chat-empty-state">
              <h1>{t('chat.title')}</h1>
              <p>{t('chat.subtitle')}</p>
            </div>
          </div>
        ) : (
          <div className="chat-scroll" ref={chatScrollRef} onScroll={handleChatScroll}>
            <div className="chat-thread">
              {messages.map((item) => (
                <ConversationTurn key={item.id} message={item} />
              ))}
            </div>
          </div>
        )}

        <div className="chat-composer">
          <Input.TextArea
            value={question}
            onChange={(event) => setQuestion(event.target.value)}
            onKeyDown={handleInputKeyDown}
            placeholder={t('chat.inputPlaceholder')}
            autoSize={{ minRows: 1, maxRows: 10 }}
            disabled={asking}
            className="chat-input"
          />
          <Button
            type="primary"
            icon={<SendOutlined />}
            loading={asking}
            aria-label={t('chat.send')}
            onClick={() => void handleAsk()}
          />
        </div>
      </section>
    </div>
  )
}
