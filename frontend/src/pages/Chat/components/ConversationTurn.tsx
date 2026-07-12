import { GlobalOutlined } from '@ant-design/icons'
import { Collapse, Space, Tag, Typography } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkBreaks from 'remark-breaks'
import type { ExternalLink } from '../../../api/chat'
import { useI18n } from '../../../useI18n'
import type { RetrievalChunk } from '../../../api/retrieval'
import type { ChatMessage } from '../types'

export function ConversationTurn({ message: chatMessage }: { message: ChatMessage }) {
  return (
    <article className="chat-turn">
      <div className="chat-question">{chatMessage.question}</div>
      <div className="chat-answer">
        <ReasoningProcess message={chatMessage} />
        <AnswerContent
          messageID={chatMessage.id}
          answer={chatMessage.answer}
          errorReason={chatMessage.errorReason}
          chunks={chatMessage.chunks}
          externalLinks={chatMessage.externalLinks}
        />
      </div>
    </article>
  )
}

function ReasoningProcess({ message }: { message: ChatMessage }) {
  const { t } = useI18n()
  const steps = message.processSteps ?? []
  const [now, setNow] = useState(() => Date.now())
  const externalLinks = message.externalLinks ?? []

  useEffect(() => {
    if (!message.loading || !message.startedAt) {
      return
    }

    setNow(Date.now())
    const timer = window.setInterval(() => setNow(Date.now()), 1_000)
    return () => window.clearInterval(timer)
  }, [message.loading, message.startedAt])

  const elapsedSeconds = message.startedAt
    ? Math.max(0, Math.floor(((message.finishedAt ?? now) - message.startedAt) / 1_000))
    : null

  return (
    <details className="thinking-process">
      <summary>
        <span className="chat-status-row">
          <Typography.Text type="secondary" className="chat-status">
            {message.status}
          </Typography.Text>
          {elapsedSeconds !== null && (
            <Typography.Text type="secondary" className="chat-elapsed">
              {formatElapsed(elapsedSeconds)}
            </Typography.Text>
          )}
          {message.modelLabel && (
            <Typography.Text type="secondary" className="chat-model-label">
              {message.modelLabel}
            </Typography.Text>
          )}
          {!message.loading && message.webSearch && (
            <span className="chat-external-link-icons" aria-label={t('chat.externalSources')}>
              {externalLinks.length > 0 ? (
                <a
                  className="chat-external-link-icon"
                  href={externalLinks[0].url}
                  target="_blank"
                  rel="noreferrer"
                  title={
                    externalLinks.length === 1
                      ? externalLinks[0].title
                      : t('chat.externalSourcesCount', { count: externalLinks.length })
                  }
                >
                  <GlobalOutlined />
                </a>
              ) : (
                <span className="chat-external-link-icon chat-external-link-icon-muted" title={t('chat.externalSourcesEmpty')}>
                  <GlobalOutlined />
                </span>
              )}
            </span>
          )}
        </span>
      </summary>
      <ol>
        {steps.map((step, index) => (
          <li key={`${step}-${index}`}>{step}</li>
        ))}
      </ol>
    </details>
  )
}

function formatElapsed(seconds: number) {
  if (seconds < 60) {
    return `${seconds}s`
  }
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

function AnswerContent({
  messageID,
  answer,
  errorReason,
  chunks,
  externalLinks,
}: {
  messageID: string
  answer: string
  errorReason?: string
  chunks: RetrievalChunk[]
  externalLinks: ExternalLink[]
}) {
  if (!answer) {
    return errorReason ? (
      <Typography.Paragraph className="answer-text answer-error-text">{errorReason}</Typography.Paragraph>
    ) : (
      <Typography.Paragraph className="answer-text"> </Typography.Paragraph>
    )
  }

  const normalizedAnswer = normalizeAnswer(rewriteCitationReferences(messageID, stripExternalLinks(answer), chunks, externalLinks))
  const cannotAnswer = isCannotAnswer(normalizedAnswer)
  return (
    <div className="answer-text">
      <ReactMarkdown
        remarkPlugins={[remarkBreaks]}
        components={{
          a: ({ children, href, title, ...props }) => {
            if (href?.startsWith('#citation-')) {
              const [text = '', rawSource = 'keyword'] = title?.split('|||') ?? []
              return (
                <button
                  type="button"
                  className="inline-citation-button"
                  title={text}
                  onClick={() => openCitationReference(href)}
                >
                  <Tag className="inline-citation-tag" color={rawSource === 'vector' ? 'blue' : 'gold'}>
                    {children}
                  </Tag>
                </button>
              )
            }
            if (href?.startsWith('http://') || href?.startsWith('https://')) {
              return (
                <a {...props} href={href} title={title} target="_blank" rel="noreferrer" className="inline-citation-link">
                  <Tag className="inline-citation-tag" color="geekblue">
                    {children}
                  </Tag>
                </a>
              )
            }
            return <a {...props} href={href} title={title} target="_blank" rel="noreferrer">{children}</a>
          },
        }}
      >
        {normalizedAnswer}
      </ReactMarkdown>
      {!cannotAnswer && <SourceDetails messageID={messageID} chunks={chunks} answer={answer} externalLinks={externalLinks} />}
    </div>
  )
}

function SourceDetails({
  messageID,
  chunks,
  answer,
  externalLinks,
}: {
  messageID: string
  chunks: RetrievalChunk[]
  answer: string
  externalLinks: ExternalLink[]
}) {
  const { t } = useI18n()
  const referencedChunkEntries = pickReferencedChunks(answer, chunks)
  const referencedExternalLinks = pickReferencedExternalLinks(answer, externalLinks)
  const totalCount = referencedChunkEntries.length + referencedExternalLinks.length
  const [openGroups, setOpenGroups] = useState<string[]>([])
  const [openSource, setOpenSource] = useState<string[]>([])

  useEffect(() => {
    function handleOpenCitation(event: Event) {
      const detail = (event as CustomEvent<{ messageID: string; citationKey: string }>).detail
      if (!detail || detail.messageID !== messageID) {
        return
      }

      setOpenGroups(['sources'])
      setOpenSource([detail.citationKey])
      window.requestAnimationFrame(() => {
        const target = document.getElementById(detail.citationKey)
        target?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
      })
    }

    window.addEventListener('open-citation-reference', handleOpenCitation as EventListener)
    return () => window.removeEventListener('open-citation-reference', handleOpenCitation as EventListener)
  }, [messageID])

  const sourceItems = useMemo(() => {
    const chunkItems = referencedChunkEntries.map(({ chunk, index }) => {
      const key = `citation-${messageID}-${index}`
      return {
        key,
        label: (
          <Space size={[8, 8]} wrap>
            <Tag color={chunk.source === 'vector' ? 'blue' : 'gold'}>{index}</Tag>
            <Tag>{chunk.doc_name || t('common.unknownDocument')}</Tag>
            <Tag>{t('chat.section', { section: chunk.section || t('common.unnamedSection') })}</Tag>
            <Tag>{t('chat.page', { page: chunk.page > 0 ? chunk.page : t('common.none') })}</Tag>
            <Tag>{t('chat.score', { score: `${Math.round(chunk.score * 100)}%` })}</Tag>
          </Space>
        ),
        children: <Typography.Paragraph className="citation-content">{chunk.content}</Typography.Paragraph>,
      }
    })

    const externalItems = referencedExternalLinks.map((link) => {
      const key = `citation-${messageID}-external-${link.index}`
      return {
        key,
        label: (
          <Space size={[8, 8]} wrap>
            <Tag color="geekblue">{link.index}</Tag>
            <a href={link.url} target="_blank" rel="noreferrer" title={link.url} className="external-source-link">
              {link.title}
            </a>
          </Space>
        ),
        children: link.snippet ? <Typography.Paragraph className="citation-content">{link.snippet}</Typography.Paragraph> : null,
      }
    })

    return [...chunkItems, ...externalItems]
  }, [messageID, referencedChunkEntries, referencedExternalLinks, t])

  return (
    <Collapse
      className="source-collapse"
      activeKey={openGroups}
      onChange={(keys) => setOpenGroups(toStringKeys(keys))}
      items={[
        {
          key: 'sources',
          label: t('chat.sourceSummary', { count: totalCount }),
          children: (
            <Collapse
              accordion
              className="source-collapse-inner"
              activeKey={openSource}
              onChange={(keys) => setOpenSource(toStringKeys(keys))}
              items={sourceItems.map((item) => ({
                ...item,
                label: <div id={item.key}>{item.label}</div>,
              }))}
            />
          ),
        },
      ]}
    />
  )
}

function isCannotAnswer(answer: string) {
  const normalized = answer.replace(/[。.!！\s]/g, '')
  return (
    normalized === '无法确定' ||
    normalized === 'Unabletodetermine' ||
    normalized.includes('无法回答') ||
    normalized.includes('资料不足')
  )
}

function normalizeAnswer(answer: string) {
  return mergeStandaloneMarkdownCitationLines(
    answer
    .replace(/\r\n/g, '\n')
    .replace(/([：:])\s*(\d+[.）])/g, '$1\n$2')
    .replace(/([；;])\s*(\d+[.）])/g, '$1\n$2')
    .replace(/(^|\n)(\d+[.、）])\s*\n+\s*(?=\S)/g, '$1$2 ')
    .replace(/([：:])\n{2,}(?=\d+[.、）]\s)/g, '$1\n')
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .replace(/([：:])\n/g, '$1\n')
    .trim(),
  )
}

function mergeStandaloneMarkdownCitationLines(answer: string) {
  const lines = answer.split('\n')
  const merged: string[] = []
  const citationLinePattern = /^\[(?:\d+)\]\([^)]+\)(?:\s+\[(?:\d+)\]\([^)]+\))*$/

  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) {
      if (merged.length > 0 && merged[merged.length - 1] !== '') {
        merged.push('')
      }
      continue
    }

    if (citationLinePattern.test(line) && merged.length > 0) {
      let anchor = merged.length - 1
      while (anchor >= 0 && merged[anchor] === '') {
        anchor--
      }
      if (anchor >= 0) {
        merged[anchor] = `${merged[anchor]} ${line}`.trim()
        continue
      }
    }

    merged.push(line)
  }

  return merged.join('\n').replace(/\n{3,}/g, '\n\n').trim()
}

function stripExternalLinks(answer: string) {
  return answer.replace(/\n{0,2}网络来源：[\s\S]*$/u, '').trim()
}

function rewriteCitationReferences(messageID: string, answer: string, chunks: RetrievalChunk[], externalLinks: ExternalLink[]) {
  let rewritten = answer.replace(/\[引用(\d+)\]/gu, (_, rawIndex: string) => {
    const index = Number(rawIndex)
    const chunk = chunks[index - 1]
    const title = chunk?.doc_name?.trim() || `引用来源${index}`
    const source = chunk?.source === 'vector' ? 'vector' : 'keyword'
    return `[${index}](#citation-${messageID}-${index} "${title}|||${source}")`
  })

  rewritten = rewritten.replace(/\[外链(\d+)\]/gu, (_, rawIndex: string) => {
    const index = Number(rawIndex)
    const link = externalLinks[index - 1]
    if (!link?.url) {
      return `${index}`
    }
    const title = link.title?.trim() || link.url
    return `[${index}](${link.url} "${title}")`
  })

  return rewritten
}

function openCitationReference(href: string) {
  const citationKey = href.replace(/^#/, '')
  const match = citationKey.match(/^citation-(.+)-(\d+)$/)
  if (!match) {
    return
  }
  const [, messageID] = match
  window.dispatchEvent(new CustomEvent('open-citation-reference', { detail: { messageID, citationKey } }))
}

function toStringKeys(keys: string | number | Array<string | number>) {
  const values = Array.isArray(keys) ? keys : keys ? [keys] : []
  return values.map(String)
}

function pickReferencedExternalLinks(answer: string, externalLinks: ExternalLink[]) {
  const matches = answer.matchAll(/\[外链(\d+)\]/gu)
  const seen = new Set<number>()
  const result: Array<ExternalLink & { index: number }> = []
  for (const match of matches) {
    const index = Number(match[1])
    if (!Number.isFinite(index) || index <= 0 || seen.has(index)) {
      continue
    }
    const link = externalLinks[index - 1]
    if (!link?.url) {
      continue
    }
    seen.add(index)
    result.push({ ...link, index })
  }
  return result
}

function pickReferencedChunks(answer: string, chunks: RetrievalChunk[]) {
  const matches = answer.matchAll(/\[引用(\d+)\]/gu)
  const seen = new Set<number>()
  const result: Array<{ chunk: RetrievalChunk; index: number }> = []
  for (const match of matches) {
    const index = Number(match[1])
    if (!Number.isFinite(index) || index <= 0 || seen.has(index)) {
      continue
    }
    const chunk = chunks[index - 1]
    if (!chunk?.id) {
      continue
    }
    seen.add(index)
    result.push({ chunk, index })
  }
  return result
}
