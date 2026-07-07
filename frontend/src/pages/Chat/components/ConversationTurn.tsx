import { Space, Tag, Typography } from 'antd'
import { useI18n } from '../../../useI18n'
import type { RetrievalChunk } from '../../../api/retrieval'
import type { ChatMessage } from '../types'

export function ConversationTurn({ message: chatMessage }: { message: ChatMessage }) {
  return (
    <article className="chat-turn">
      <div className="chat-question">{chatMessage.question}</div>
      <div className="chat-answer">
        {chatMessage.status && (
          <Typography.Text type="secondary" className="chat-status">
            {chatMessage.status}
          </Typography.Text>
        )}
        <AnswerContent answer={chatMessage.answer} chunks={chatMessage.chunks} />
      </div>
    </article>
  )
}

function AnswerContent({ answer, chunks }: { answer: string; chunks: RetrievalChunk[] }) {
  if (!answer) {
    return <Typography.Paragraph className="answer-text"> </Typography.Paragraph>
  }

  const cannotAnswer = isCannotAnswer(answer)
  const paragraphs = answer.split(/\n{2,}/).filter((paragraph) => paragraph.trim() !== '')

  return (
    <div className="answer-text">
      {paragraphs.map((paragraph, paragraphIndex) => (
        <p key={`${paragraph}-${paragraphIndex}`}>{paragraph}</p>
      ))}
      {!cannotAnswer && chunks.length > 0 && <SourceDetails chunks={chunks} />}
    </div>
  )
}

function SourceDetails({ chunks }: { chunks: RetrievalChunk[] }) {
  const { t } = useI18n()

  return (
    <details className="source-summary">
      <summary>{t('chat.sourceSummary', { count: chunks.length })}</summary>
      <div className="source-list">
        {chunks.map((chunk, index) => (
          <details key={chunk.id} className="source-item">
            <summary>
              <Space size={[8, 8]} wrap>
                <Tag color={chunk.source === 'vector' ? 'blue' : 'gold'}>{t('chat.source', { index: index + 1 })}</Tag>
                <Tag>{chunk.doc_name || t('common.unknownDocument')}</Tag>
                <Tag>{t('chat.section', { section: chunk.section || t('common.unnamedSection') })}</Tag>
                <Tag>{t('chat.page', { page: chunk.page > 0 ? chunk.page : t('common.none') })}</Tag>
                <Tag>{t('chat.score', { score: chunk.score.toFixed(2) })}</Tag>
              </Space>
            </summary>
            <Typography.Paragraph className="citation-content">{chunk.content}</Typography.Paragraph>
          </details>
        ))}
      </div>
    </details>
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
