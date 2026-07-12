import { useEffect, useState } from 'react'
import { listDocuments, type DocumentInfo } from '../../../api/documents'

export type SubjectOverview = {
  total: number
  indexed: number
  failed: number
  processing: number
  recentDocuments: DocumentInfo[]
  fileTypes: { type: string; count: number }[]
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

        const typeMap = new Map<string, number>()
        for (const item of data.list) {
          const type = (item.file_type || 'unknown').toLowerCase()
          typeMap.set(type, (typeMap.get(type) ?? 0) + 1)
        }

        setSubjectOverview({
          total: data.total,
          indexed: data.list.filter((item) => item.status === 'indexed').length,
          failed: data.list.filter((item) => item.status === 'failed' || item.status === 'delete_failed').length,
          processing: data.list.filter((item) =>
            ['uploaded', 'parsing', 'parsed', 'chunking', 'chunked', 'embedding', 'deleting'].includes(item.status),
          ).length,
          recentDocuments: [...data.list]
            .sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime())
            .slice(0, 5),
          fileTypes: Array.from(typeMap.entries())
            .map(([type, count]) => ({ type, count }))
            .sort((left, right) => right.count - left.count)
            .slice(0, 4),
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
