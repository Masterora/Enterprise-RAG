import { App as AntdApp, Button } from 'antd'
import { DownOutlined } from '@ant-design/icons'
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type UIEvent } from 'react'
import {
  createChatSession,
  deleteChatSession,
  listChatSessions,
  updateChatSession,
  type StoredChatSession,
} from '../../api/chat'
import { listSubjects, type SubjectInfo } from '../../api/subjects'
import { useI18n } from '../../useI18n'
import { ConversationTurn } from './components/ConversationTurn'
import { ChatComposer } from './components/ChatComposer'
import { ChatEmptyState } from './components/ChatEmptyState'
import {
  buildChatModelValue,
  CHAT_MODEL_OPTIONS,
  DEFAULT_CHAT_MODEL,
  getBestVendorModel,
  getVendorModelCascaderOptions,
  getVendorOptions,
  parseChatModelValue,
} from './models'
import { SessionList } from './components/SessionList'
import { RetrievalEvaluationModal } from './components/RetrievalEvaluationModal'
import { useSubjectOverview } from './hooks/useSubjectOverview'
import { classifyChatFailure, localizeChatStatus, streamAnswer } from './streaming'
import type { ChatMessage, ChatSession } from './types'
import { buildSessionTitle } from './utils'
import './chat.css'

const ASK_TIMEOUT_MS = 30_000
const DEFAULT_CHAT_TOP_K = 5
const CHAT_TOP_K = Math.max(1, Number(import.meta.env.VITE_CHAT_TOP_K ?? DEFAULT_CHAT_TOP_K))

type SelectedLLM = {
  provider: string
  model: string
}

function mapStoredSession(
  session: StoredChatSession,
  doneStatus: string,
  insufficientStatus: string,
  webSearchStatus: string,
): ChatSession {
  return {
    id: session.id,
    title: session.title,
    subjectID: session.subject_id,
    llmProvider: session.llm_provider || DEFAULT_CHAT_MODEL.provider,
    llmModel: session.llm_model || DEFAULT_CHAT_MODEL.model,
    createdAt: Date.parse(session.created_at),
    updatedAt: Date.parse(session.updated_at),
    messages: session.messages.map((item) => ({
      id: item.id,
      question: item.question,
      answer: item.answer,
      status: item.metrics?.answered === false ? insufficientStatus : doneStatus,
      chunks: item.chunks ?? [],
      externalLinks: item.external_links ?? [],
      metrics: item.metrics,
      modelLabel: item.model_label || item.model_id,
      modelID: item.model_id,
      webSearch: item.web_search,
      processSteps: item.web_search ? [webSearchStatus] : [],
      agentSteps: item.agent_steps ?? [],
      startedAt: Date.parse(item.created_at),
      finishedAt: Date.parse(item.created_at),
      loading: false,
    })),
  }
}

export function ChatPage() {
	const { message } = AntdApp.useApp()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [subjectsLoaded, setSubjectsLoaded] = useState(false)
  const [subjectID, setSubjectID] = useState('')
  const [question, setQuestion] = useState('')
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [activeSessionID, setActiveSessionID] = useState('')
  const [editingSessionID, setEditingSessionID] = useState('')
  const [editingTitle, setEditingTitle] = useState('')
  const [asking, setAsking] = useState(false)
  const [subjectSelectOpen, setSubjectSelectOpen] = useState(false)
  const [modelSelectOpen, setModelSelectOpen] = useState(false)
  const [vendorSelectOpen, setVendorSelectOpen] = useState(false)
  const [expandedSeriesKey, setExpandedSeriesKey] = useState('')
  const [expandedSeriesIndex, setExpandedSeriesIndex] = useState(-1)
  const [evaluationOpen, setEvaluationOpen] = useState(false)
  const [webSearch, setWebSearch] = useState(false)
  const [showScrollToBottom, setShowScrollToBottom] = useState(false)
  const [selectedLLM, setSelectedLLM] = useState<SelectedLLM>({
    provider: DEFAULT_CHAT_MODEL.provider,
    model: DEFAULT_CHAT_MODEL.model,
  })
  const chatScrollRef = useRef<HTMLDivElement | null>(null)
  const autoFollowRef = useRef(true)
  const selectedLLMRef = useRef(selectedLLM)
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
  const activeModelOption = useMemo(
    () =>
      CHAT_MODEL_OPTIONS.find(
        (option) =>
          option.provider === selectedLLM.provider &&
          option.model === selectedLLM.model,
      ) || DEFAULT_CHAT_MODEL,
    [selectedLLM.model, selectedLLM.provider],
  )
  const vendorOptions = useMemo(() => getVendorOptions(), [])
  const modelOptions = useMemo(
    () => getVendorModelCascaderOptions(activeModelOption.vendor),
    [activeModelOption.vendor],
  )
  const messages = activeSession?.messages ?? []
  const { subjectOverview, subjectOverviewLoading } = useSubjectOverview(subjectID)

  useEffect(() => {
    async function loadSessions() {
      try {
        const stored = await listChatSessions()
        setSessions(stored.map((session) =>
          mapStoredSession(
            session,
            t('chat.done'),
            t('chat.insufficient'),
            t('chat.process.webSearchEnabled'),
          ),
        ))
      } catch {
        message.error(t('chat.loadSessionsFailed'))
      }
    }
    void loadSessions()
  }, [message, t])

  useEffect(() => {
    selectedLLMRef.current = selectedLLM
  }, [selectedLLM])

  useEffect(() => {
    async function loadSubjects() {
      try {
        const data = await listSubjects({ page: 1, page_size: 100 })
        setSubjects(data.list)
      } catch {
        message.error(t('chat.loadSubjectsFailed'))
      } finally {
        setSubjectsLoaded(true)
      }
    }

    void loadSubjects()
  }, [message, t])

  useEffect(() => {
    if (sessions.length === 0 || activeSessionID) {
      return
    }
    setActiveSessionID(sessions[0].id)
  }, [activeSessionID, sessions])

  useEffect(() => {
    if (!subjectsLoaded) {
      return
    }

    if (subjects.length === 0) {
      setSubjectID('')
      if (activeSession?.subjectID) {
        updateSession(activeSession.id, (session) => ({ ...session, subjectID: '' }))
      }
      return
    }

    const hasSubject = (value: string) => subjects.some((subject) => subject.id === value)

    if (activeSession?.subjectID) {
      if (hasSubject(activeSession.subjectID)) {
        setSubjectID(activeSession.subjectID)
      } else {
        setSubjectID('')
        updateSession(activeSession.id, (session) => ({ ...session, subjectID: '' }))
      }
      return
    }

    setSubjectID((current) => (current && hasSubject(current) ? current : subjects[0].id))
  }, [activeSession?.id, activeSession?.subjectID, subjects, subjectsLoaded])

  useEffect(() => {
    autoFollowRef.current = true
    setShowScrollToBottom(false)
  }, [activeSessionID])

  useEffect(() => {
    if (!activeSession) {
      setSelectedLLM({
        provider: DEFAULT_CHAT_MODEL.provider,
        model: DEFAULT_CHAT_MODEL.model,
      })
      return
    }
    setSelectedLLM({
      provider: activeSession.llmProvider || DEFAULT_CHAT_MODEL.provider,
      model: activeSession.llmModel || DEFAULT_CHAT_MODEL.model,
    })
  }, [activeSession])

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

  async function createSession(nextSubjectID?: string) {
    const timestamp = Date.now()
    const currentLLM = selectedLLMRef.current
    const session: ChatSession = {
      id: crypto.randomUUID(),
      title: t('chat.newSession'),
      subjectID: nextSubjectID ?? subjectID,
      llmProvider: currentLLM.provider,
      llmModel: currentLLM.model,
      createdAt: timestamp,
      updatedAt: timestamp,
      messages: [],
    }
    await createChatSession({
      id: session.id,
      subject_id: session.subjectID,
      title: session.title,
      llm_provider: session.llmProvider,
      llm_model: session.llmModel,
    })
    setSessions((current) => [session, ...current])
    setActiveSessionID(session.id)
    setSubjectID(session.subjectID)
    return session
  }

  async function ensureActiveSession() {
    return activeSession ?? createSession(subjectID)
  }

  function startRenameSession(session: ChatSession) {
    setEditingSessionID(session.id)
    setEditingTitle(session.title)
  }

  async function confirmRenameSession(sessionID: string) {
    const nextTitle = editingTitle.trim()
    await updateChatSession({ id: sessionID, title: nextTitle })
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

  async function deleteSession(sessionID: string) {
    await deleteChatSession(sessionID)
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

  function appendProcessStep(sessionID: string, messageID: string, step: string) {
    const nextStep = step.trim()
    if (!nextStep) {
      return
    }
    updateSession(sessionID, (session) => ({
      ...session,
      updatedAt: Date.now(),
      messages: session.messages.map((item) => {
        if (item.id !== messageID) {
          return item
        }
        const currentSteps = item.processSteps ?? []
        if (currentSteps[currentSteps.length - 1] === nextStep) {
          return item
        }
        return { ...item, processSteps: [...currentSteps, nextStep] }
      }),
    }))
  }

  function closeComposerPopups() {
    setSubjectSelectOpen(false)
    setModelSelectOpen(false)
    setVendorSelectOpen(false)
    setExpandedSeriesKey('')
    setExpandedSeriesIndex(-1)
  }

  async function handleAsk() {
    closeComposerPopups()

    const trimmedQuestion = question.trim()
    if (!subjectID) {
      message.warning(t('chat.selectSubjectWarning'))
      return
    }
    if (!trimmedQuestion) {
      message.warning(t('chat.inputWarning'))
      return
    }

    let session: ChatSession
    try {
      session = await ensureActiveSession()
    } catch {
      message.error(t('chat.createSessionFailed'))
      return
    }
    const currentLLM = selectedLLMRef.current
    const currentModelOption =
      CHAT_MODEL_OPTIONS.find((option) => option.provider === currentLLM.provider && option.model === currentLLM.model) ??
      DEFAULT_CHAT_MODEL
    const messageID = crypto.randomUUID()
    const nextTimestamp = Date.now()
    if (session.messages.length === 0) {
      const title = buildSessionTitle(trimmedQuestion)
      try {
        await updateChatSession({ id: session.id, title })
      } catch {
        message.error(t('chat.renameFailed'))
        return
      }
    }
    setQuestion('')
    setAsking(true)

    updateSession(session.id, (current) => ({
      ...current,
      title: current.messages.length === 0 ? buildSessionTitle(trimmedQuestion) : current.title,
      subjectID,
      llmProvider: currentLLM.provider,
      llmModel: currentLLM.model,
      updatedAt: nextTimestamp,
      messages: [
        ...current.messages,
        {
          id: messageID,
          question: trimmedQuestion,
          answer: '',
          errorReason: '',
          status: t('chat.preparing'),
          chunks: [],
          externalLinks: [],
          modelLabel: currentModelOption.label,
          modelID: currentModelOption.model,
          webSearch,
          processSteps: [
            t('chat.process.model', { model: currentModelOption.label }),
            ...(webSearch ? [t('chat.process.webSearchEnabled')] : []),
            t('chat.process.questionSubmitted'),
          ],
          agentSteps: [],
          startedAt: nextTimestamp,
          loading: true,
        },
      ],
    }))

    try {
      await askWithRetry(
        session.id,
        messageID,
        trimmedQuestion,
        subjectID,
        currentLLM.provider,
        currentLLM.model,
        webSearch,
      )
    } catch (error) {
		const failure = classifyChatFailure(error, t)
      updateMessage(session.id, messageID, {
        status: failure.status,
        errorReason: failure.detail,
        finishedAt: Date.now(),
        loading: false,
      })
      appendProcessStep(session.id, messageID, failure.detail)
      message.error(failure.toast)
    } finally {
      setAsking(false)
    }
  }

  async function askWithRetry(
    sessionID: string,
    messageID: string,
    currentQuestion: string,
    currentSubjectID: string,
    currentLLMProvider: string,
    currentLLMModel: string,
    currentWebSearch: boolean,
  ) {
    return streamAnswer({
      sessionID,
      messageID,
      question: currentQuestion,
      subjectID: currentSubjectID,
      llmProvider: currentLLMProvider,
      llmModel: currentLLMModel,
      webSearch: currentWebSearch,
      topK: CHAT_TOP_K,
      askTimeoutMS: ASK_TIMEOUT_MS,
      labels: {
        rewriteDone: (query) => t('chat.process.rewriteDone', { query }),
        rewriteSkipped: t('chat.process.rewriteSkipped'),
        retrievalDone: (returned, candidates) => t('chat.process.retrievalDone', { returned, candidates }),
        rerankDone: t('chat.process.rerankDone'),
        rerankSkipped: t('chat.process.rerankSkipped'),
        sourcesDone: (count) => t('chat.process.sourcesDone', { count }),
        webSourcesDone: (count) => t('chat.process.webSourcesDone', { count }),
        answerStreaming: t('chat.process.answerStreaming'),
        finished: t('chat.process.finished'),
        done: t('chat.done'),
        insufficient: t('chat.insufficient'),
      },
      callbacks: {
        updateMessage,
        appendProcessStep,
        appendAnswer,
        updateAgentStep: (sessionID, messageID, step) => {
          updateSession(sessionID, (session) => ({
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((item) => {
              if (item.id !== messageID) {
                return item
              }
              const current = item.agentSteps ?? []
              const exists = current.some((candidate) => candidate.id === step.id)
              return {
                ...item,
                agentSteps: exists
                  ? current.map((candidate) => candidate.id === step.id ? step : candidate)
                  : [...current, step],
              }
            }),
          }))
        },
      },
		localizeStatus: (status) => localizeChatStatus(status, t),
    })
  }

  function handleInputKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== 'Enter' || event.shiftKey) {
      return
    }
    event.preventDefault()
    void handleAsk()
  }

  function handleSubjectChange(value: string) {
    setSubjectSelectOpen(false)
    setSubjectID(value)
    if (!activeSession) {
      return
    }
    updateSession(activeSession.id, (session) => ({ ...session, subjectID: value }))
  }

  function handleModelChange(value: string) {
    if (!value) {
      return
    }
    applyModelChange(parseChatModelValue(value))
  }

  function applyModelChange(nextModel: SelectedLLM) {
    const modelChanged =
      nextModel.provider !== selectedLLMRef.current.provider || nextModel.model !== selectedLLMRef.current.model

    setModelSelectOpen(false)
    setVendorSelectOpen(false)
    setExpandedSeriesKey('')
    setExpandedSeriesIndex(-1)

    if (!modelChanged) {
      return
    }

    setSelectedLLM(nextModel)
    selectedLLMRef.current = nextModel

    if (activeSession?.messages.length) {
      message.warning(t('chat.modelSwitchWarning'))
    }

    if (!activeSession) {
      return
    }

    updateSession(activeSession.id, (session) => ({
      ...session,
      llmProvider: nextModel.provider,
      llmModel: nextModel.model,
      updatedAt: Date.now(),
    }))
  }

  function handleVendorChange(value: string) {
    const nextOption = getBestVendorModel(value)
    applyModelChange({
      provider: nextOption.provider,
      model: nextOption.model,
    })
  }

  function handleModelDropdownOpenChange(open: boolean) {
    setModelSelectOpen(open)
    setExpandedSeriesKey('')
    setExpandedSeriesIndex(-1)
  }

  function handleSeriesClick(seriesKey: string, index: number) {
    const willCollapse = expandedSeriesKey === seriesKey
    setExpandedSeriesKey(willCollapse ? '' : seriesKey)
    setExpandedSeriesIndex(willCollapse ? -1 : index)
  }

  function handleChatScroll(event: UIEvent<HTMLDivElement>) {
    const container = event.currentTarget
    const remaining = container.scrollHeight - container.scrollTop - container.clientHeight
    autoFollowRef.current = remaining < 48
    setShowScrollToBottom(remaining >= 48)
  }

  function scrollToBottom() {
    if (!chatScrollRef.current) {
      return
    }
    chatScrollRef.current.scrollTo({
      top: chatScrollRef.current.scrollHeight,
      behavior: 'smooth',
    })
    autoFollowRef.current = true
    setShowScrollToBottom(false)
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
        onCreateSession={() => void createSession(subjectID || subjects[0]?.id || '')}
        onSelectSession={setActiveSessionID}
        onRenameSession={startRenameSession}
        onConfirmRenameSession={(sessionID) => void confirmRenameSession(sessionID)}
        onCancelRenameSession={cancelRenameSession}
        onDeleteSession={(sessionID) => void deleteSession(sessionID)}
      />

      <section className="chat-main">
        {messages.length === 0 ? (
          <div className="chat-scroll" ref={chatScrollRef} onScroll={handleChatScroll}>
            <ChatEmptyState
              subjectID={subjectID}
              subjects={subjects}
              subjectOverview={subjectOverview}
              subjectOverviewLoading={subjectOverviewLoading}
              onEvaluationOpen={() => setEvaluationOpen(true)}
            />
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
        {showScrollToBottom && messages.length > 0 ? (
          <Button
            type="primary"
            shape="circle"
            className="chat-scroll-bottom-button"
            icon={<DownOutlined />}
            aria-label={t('chat.scrollToBottom')}
            onClick={scrollToBottom}
          />
        ) : null}
        <ChatComposer
          asking={asking}
          question={question}
          subjectID={subjectID}
          subjects={subjects}
          subjectOptions={subjectOptions}
          vendorOptions={vendorOptions}
          modelOptions={modelOptions}
          activeModelLabel={activeModelOption.label}
          activeModelValue={buildChatModelValue(activeModelOption.provider, activeModelOption.model)}
          activeVendor={activeModelOption.vendor}
          subjectSelectOpen={subjectSelectOpen}
          vendorSelectOpen={vendorSelectOpen}
          modelSelectOpen={modelSelectOpen}
          expandedSeriesKey={expandedSeriesKey}
          expandedSeriesIndex={expandedSeriesIndex}
          webSearch={webSearch}
          onQuestionChange={setQuestion}
          onInputKeyDown={handleInputKeyDown}
          onSubjectChange={handleSubjectChange}
          onSubjectOpenChange={setSubjectSelectOpen}
          onWebSearchChange={setWebSearch}
          onModelDropdownOpenChange={handleModelDropdownOpenChange}
          onVendorChange={handleVendorChange}
          onVendorOpenChange={setVendorSelectOpen}
          onModelChange={handleModelChange}
          onSeriesClick={handleSeriesClick}
          onAsk={() => void handleAsk()}
        />
      </section>
      <RetrievalEvaluationModal
        open={evaluationOpen}
        subjectID={subjectID}
        onClose={() => setEvaluationOpen(false)}
      />
    </div>
  )
}
