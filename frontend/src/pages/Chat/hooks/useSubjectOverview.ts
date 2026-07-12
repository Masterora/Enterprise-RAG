import { useEffect, useState } from 'react'
import { listDocuments } from '../../../api/documents'

export type SubjectOverview = {
  total: number
  indexed: number
  failed: number
  processing: number
}

export function useSubjectOverview(subjectID: string) {
  const [subjectOverview, setSubjectOverview] = useState<SubjectOverview | null>(null)
  const [subjectOverviewLoading, setSubjectOverviewLoading] = useState(false)

  useEffect(() => {
    if (!subjectID) {
      setSubjectOverview(null)
      return
    }

    let cancelled = false

    async function loadSubjectOverview() {
      setSubjectOverviewLoading(true)
      try {
        const data = await listDocuments({ subject_id: subjectID, page: 1, page_size: 1000 })
        if (cancelled) {
          return
        }

        setSubjectOverview({
          total: data.total,
          indexed: data.list.filter((item) => item.status === 'indexed').length,
          failed: data.list.filter((item) => item.status === 'failed' || item.status === 'delete_failed').length,
          processing: data.list.filter((item) =>
            ['uploaded', 'parsing', 'parsed', 'chunking', 'chunked', 'embedding', 'deleting'].includes(item.status),
          ).length,
        })
      } catch {
        if (!cancelled) {
          setSubjectOverview(null)
        }
      } finally {
        if (!cancelled) {
          setSubjectOverviewLoading(false)
        }
      }
    }

    void loadSubjectOverview()
    return () => {
      cancelled = true
    }
  }, [subjectID])

  return { subjectOverview, subjectOverviewLoading }
}
