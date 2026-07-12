import { ExperimentOutlined } from '@ant-design/icons'
import { Button } from 'antd'
import type { SubjectInfo } from '../../../api/subjects'
import { useI18n } from '../../../useI18n'
import type { SubjectOverview } from '../hooks/useSubjectOverview'

type Props = {
  subjectID: string
  subjects: SubjectInfo[]
  subjectOverview: SubjectOverview | null
  subjectOverviewLoading: boolean
  onEvaluationOpen: () => void
}

export function ChatEmptyState({
  subjectID,
  subjects,
  subjectOverview,
  subjectOverviewLoading,
  onEvaluationOpen,
}: Props) {
  const { t } = useI18n()
  const readiness = getReadinessText(subjectOverview, subjectOverviewLoading, t)

  return (
    <div className="chat-empty-state">
      <h1>{t('chat.title')}</h1>
      <p>{t('chat.subtitle')}</p>
      {subjectID ? (
        <div className="chat-subject-overview">
          <div className="chat-subject-overview-header">
            <strong>
              {t('chat.overview.title', {
                name: subjects.find((subject) => subject.id === subjectID)?.name ?? t('chat.selectSubject'),
              })}
            </strong>
            <div className="chat-subject-overview-actions">
              <span className="chat-subject-overview-readiness">{readiness}</span>
              <Button
                type="text"
                size="small"
                icon={<ExperimentOutlined />}
                disabled={!subjectOverview?.indexed}
                onClick={onEvaluationOpen}
              >
                {t('chat.evaluation.title')}
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function getReadinessText(
  overview: SubjectOverview | null,
  loading: boolean,
  t: (key: string, values?: Record<string, string | number>) => string,
) {
  if (loading) {
    return t('chat.overview.loading')
  }
  if (!overview || overview.total === 0) {
    return t('chat.overview.noDocuments')
  }
  if (overview.indexed === 0 && overview.processing > 0) {
    return t('chat.overview.processingStatus', { count: overview.processing })
  }
  if (overview.indexed === 0) {
    return t('chat.overview.noAnswerable')
  }
  if (overview.processing > 0) {
    return t('chat.overview.partiallyReady', { indexed: overview.indexed, processing: overview.processing })
  }
  if (overview.failed > 0) {
    return t('chat.overview.readyWithFailures', { indexed: overview.indexed, failed: overview.failed })
  }
  return t('chat.overview.ready', { count: overview.indexed })
}
