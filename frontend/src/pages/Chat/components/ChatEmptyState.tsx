import type { SubjectInfo } from '../../../api/subjects'
import { useI18n } from '../../../useI18n'
import type { SubjectOverview } from '../hooks/useSubjectOverview'

type Props = {
  subjectID: string
  subjects: SubjectInfo[]
  subjectOverview: SubjectOverview | null
  subjectOverviewLoading: boolean
}

export function ChatEmptyState({ subjectID, subjects, subjectOverview, subjectOverviewLoading }: Props) {
  const { t } = useI18n()

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
            <span>{subjectOverviewLoading ? t('chat.overview.loading') : t('chat.overview.ready')}</span>
          </div>
          {subjectOverview ? (
            <>
              <div className="chat-subject-overview-metrics">
                <div>
                  <span>{t('chat.overview.totalDocuments')}</span>
                  <strong>{subjectOverview.total}</strong>
                </div>
                <div>
                  <span>{t('chat.overview.indexedDocuments')}</span>
                  <strong>{subjectOverview.indexed}</strong>
                </div>
                <div>
                  <span>{t('chat.overview.processingDocuments')}</span>
                  <strong>{subjectOverview.processing}</strong>
                </div>
                <div>
                  <span>{t('chat.overview.failedDocuments')}</span>
                  <strong>{subjectOverview.failed}</strong>
                </div>
              </div>
              <div className="chat-subject-overview-sections">
                <div className="chat-subject-overview-block">
                  <span>{t('chat.overview.fileTypes')}</span>
                  <div className="chat-subject-overview-tags">
                    {subjectOverview.fileTypes.length > 0 ? (
                      subjectOverview.fileTypes.map((item) => (
                        <span className="chat-subject-overview-tag" key={item.type}>
                          {item.type} · {item.count}
                        </span>
                      ))
                    ) : (
                      <span className="chat-subject-overview-empty">{t('chat.overview.empty')}</span>
                    )}
                  </div>
                </div>
                <div className="chat-subject-overview-block">
                  <span>{t('chat.overview.recentDocuments')}</span>
                  <ul>
                    {subjectOverview.recentDocuments.length > 0 ? (
                      subjectOverview.recentDocuments.map((item) => <li key={item.id}>{item.filename}</li>)
                    ) : (
                      <li className="chat-subject-overview-empty">{t('chat.overview.empty')}</li>
                    )}
                  </ul>
                </div>
              </div>
            </>
          ) : (
            <div className="chat-subject-overview-empty">{t('chat.overview.empty')}</div>
          )}
        </div>
      ) : null}
    </div>
  )
}
