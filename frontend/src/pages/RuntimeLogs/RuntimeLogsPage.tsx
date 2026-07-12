import {
  CaretDownOutlined,
  CaretUpOutlined,
  CheckCircleOutlined,
  ClearOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  ExclamationCircleFilled,
  FileTextOutlined,
  LoadingOutlined,
  MessageOutlined,
  RetweetOutlined,
  ScissorOutlined,
} from '@ant-design/icons'
import {
  Button,
  Card,
  Modal,
  Pagination,
  Segmented,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import type { FilterValue, SortOrder, SorterResult } from 'antd/es/table/interface'
import { useCallback, useEffect, useState } from 'react'
import {
  clearAdminLogs,
  clearAdminTasks,
  getAdminSummary,
  listAdminLogs,
  listAdminTasks,
  type AdminSummary,
} from '../../api/admin'
import { retryIndexTask, type IndexTaskInfo, type ParseLogInfo } from '../../api/documents'
import { listSubjects, type SubjectInfo } from '../../api/subjects'
import { useI18n } from '../../useI18n'
import { translateErrorMessage, translateTaskMessage } from '../../utils/errorMessage'

const refreshInterval = 5000
const taskLimit = 500
const logPageSize = 10

type LogSorterKey = 'filename' | 'duration' | 'updated_at'
type ActiveSorter = { key: LogSorterKey; order: Exclude<SortOrder, null> }
type RuntimeViewMode = 'tasks' | 'logs'
type LogEntrySorterKey = 'filename' | 'created_at'

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}

function formatDuration(start: string, end: string) {
  const duration = Math.max(0, new Date(end).getTime() - new Date(start).getTime())
  if (!Number.isFinite(duration)) {
    return '-'
  }
  if (duration < 1000) {
    return '< 1s'
  }
  const seconds = Math.floor(duration / 1000)
  if (seconds < 60) {
    return `${seconds}s`
  }
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

export function RuntimeLogsPage() {
  const { t } = useI18n()
  const [summary, setSummary] = useState<AdminSummary>()
  const [subjects, setSubjects] = useState<SubjectInfo[]>([])
  const [tasks, setTasks] = useState<IndexTaskInfo[]>([])
  const [logs, setLogs] = useState<ParseLogInfo[]>([])
  const [logsTotal, setLogsTotal] = useState(0)
  const [live, setLive] = useState(true)
  const [loading, setLoading] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date>()
  const [page, setPage] = useState(1)
  const [viewMode, setViewMode] = useState<RuntimeViewMode>('tasks')
  const [subjectFilter, setSubjectFilter] = useState<string>()
  const [taskTypeFilter, setTaskTypeFilter] = useState<string>()
  const [statusFilter, setStatusFilter] = useState<string>()
  const [retryingTaskIDs, setRetryingTaskIDs] = useState<string[]>([])
  const [activeSorters, setActiveSorters] = useState<ActiveSorter[]>([
    { key: 'updated_at', order: 'descend' },
  ])
  const [logSorter, setLogSorter] = useState<{ key: LogEntrySorterKey; order: Exclude<SortOrder, null> }>({
    key: 'created_at',
    order: 'descend',
  })

  const loadData = useCallback(async () => {
    if (!tasks.length && !logs.length) {
      setLoading(true)
    }
    try {
      const [summaryData, subjectData] = await Promise.all([
        getAdminSummary(),
        listSubjects({ page: 1, page_size: 100 }),
      ])
      if (viewMode === 'tasks') {
        const taskData = await listAdminTasks({
          subject_id: subjectFilter,
          status: statusFilter,
          task_type: taskTypeFilter,
          page: 1,
          page_size: taskLimit,
        })
        setTasks(taskData.list)
      } else {
        const logData = await listAdminLogs({
          subject_id: subjectFilter,
          status: statusFilter,
          page,
          page_size: logPageSize,
        })
        setLogs(logData.list)
        setLogsTotal(logData.total)
      }
      setSummary(summaryData)
      setSubjects(subjectData.list)
      setLastUpdatedAt(new Date())
    } catch {
      message.error(t('logs.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [logs.length, page, statusFilter, subjectFilter, t, taskTypeFilter, tasks.length, viewMode])

  useEffect(() => {
    void loadData()
  }, [loadData])

  useEffect(() => {
    if (!live) {
      return
    }
    const timer = window.setInterval(() => void loadData(), refreshInterval)
    return () => window.clearInterval(timer)
  }, [live, loadData])

  async function handleRetry(id: string) {
    if (retryingTaskIDs.includes(id)) {
      return
    }
    setRetryingTaskIDs((current) => [...current, id])
    setTasks((current) =>
      current.map((task) =>
        task.id === id ? { ...task, status: 'pending', error_message: '', updated_at: new Date().toISOString() } : task,
      ),
    )
    try {
      await retryIndexTask(id)
      message.success(t('documents.retryStarted'))
      await loadData()
    } catch {
      message.error(t('documents.retryFailed'))
      await loadData()
    } finally {
      setRetryingTaskIDs((current) => current.filter((taskID) => taskID !== id))
    }
  }

  function handleClear() {
    Modal.confirm({
      title: viewMode === 'tasks' ? t('logs.clearConfirmTitle') : t('logs.clearLogsConfirmTitle'),
      content: viewMode === 'tasks' ? t('logs.clearConfirmContent') : t('logs.clearLogsConfirmContent'),
      okButtonProps: { danger: true },
      onOk: async () => {
        setClearing(true)
        try {
          const cleared =
            viewMode === 'tasks'
              ? await clearAdminTasks()
              : await clearAdminLogs({ subject_id: subjectFilter })
          message.success(t('logs.clearSuccess', { count: cleared }))
          await loadData()
        } catch {
          message.error(t('logs.clearFailed'))
        } finally {
          setClearing(false)
        }
      },
    })
  }

  function statusTag(status: string) {
    if (status === 'running') {
      return <Tag icon={<LoadingOutlined spin />} color="processing">{t('tasks.status.running')}</Tag>
    }
    if (status === 'success') {
      return <Tag icon={<CheckCircleOutlined />} color="success">{t('tasks.status.success')}</Tag>
    }
    if (status === 'failed') {
      return <Tag icon={<CloseCircleOutlined />} color="error">{t('tasks.status.failed')}</Tag>
    }
    return <Tag icon={<ClockCircleOutlined />}>{t('tasks.status.pending')}</Tag>
  }

  function renderSortIcon({ sortOrder }: { sortOrder?: SortOrder }) {
    if (sortOrder === 'ascend') {
      return <CaretUpOutlined />
    }
    return <CaretDownOutlined />
  }

  function translateTaskErrorMessage(errorMessage: string) {
		return translateErrorMessage(errorMessage, t)
  }

  function translateLogMessage(rawMessage: string) {
		return translateTaskMessage(rawMessage, t)
  }

  const runningTaskTypes = new Set(
    tasks.filter((task) => task.status === 'running').map((task) => task.task_type),
  )
  const topology = [
    {
      key: 'subjects',
      label: t('logs.topology.subjects'),
      value: summary?.subject_total ?? 0,
      icon: <DatabaseOutlined />,
      active: runningTaskTypes.has('document.parse'),
      flow: runningTaskTypes.has('document.parse'),
    },
    {
      key: 'documents',
      label: t('logs.topology.documents'),
      value: summary?.document_total ?? 0,
      icon: <FileTextOutlined />,
      active:
        runningTaskTypes.has('document.parse') || runningTaskTypes.has('document.chunk'),
      flow: runningTaskTypes.has('document.chunk'),
    },
    {
      key: 'chunks',
      label: t('logs.topology.chunks'),
      value: summary?.chunk_total ?? 0,
      icon: <ScissorOutlined />,
      active:
        runningTaskTypes.has('document.chunk') || runningTaskTypes.has('document.embedding'),
      flow: runningTaskTypes.has('document.embedding'),
    },
    {
      key: 'vectors',
      label: t('logs.topology.vectors'),
      value: summary?.indexed_total ?? 0,
      icon: <DeploymentUnitOutlined />,
      active: runningTaskTypes.has('document.embedding'),
      flow: false,
    },
    {
      key: 'sessions',
      label: t('logs.topology.sessions'),
      value: summary?.session_total ?? 0,
      icon: <MessageOutlined />,
      active: false,
      flow: false,
    },
  ]

  const indexRate =
    (summary?.document_total ?? 0) > 0
      ? Math.round(((summary?.indexed_total ?? 0) / (summary?.document_total ?? 1)) * 100)
      : 0

  const filteredTasks = tasks

  const sortedTasks = [...filteredTasks].sort((left, right) => {
    for (const sorter of activeSorters) {
      let result = 0
      if (sorter.key === 'filename') {
        result = left.filename.localeCompare(right.filename)
      } else if (sorter.key === 'duration') {
        result =
          (new Date(left.updated_at).getTime() - new Date(left.created_at).getTime()) -
          (new Date(right.updated_at).getTime() - new Date(right.created_at).getTime())
      } else if (sorter.key === 'updated_at') {
        result = new Date(left.updated_at).getTime() - new Date(right.updated_at).getTime()
      }
      if (result !== 0) {
        return sorter.order === 'ascend' ? result : -result
      }
    }
    return 0
  })

  function handleTableChange(
    _: unknown,
    filters: Record<string, FilterValue | null>,
    sorter: SorterResult<IndexTaskInfo> | SorterResult<IndexTaskInfo>[],
  ) {
    const nextTaskType = filters.task_type?.[0]
    const nextStatus = filters.status?.[0]
    setTaskTypeFilter(typeof nextTaskType === 'string' ? nextTaskType : undefined)
    setStatusFilter(typeof nextStatus === 'string' ? nextStatus : undefined)
    setPage(1)

    const priority: Record<LogSorterKey, number> = {
      filename: 1,
      duration: 2,
      updated_at: 3,
    }
    const sorterList = (Array.isArray(sorter) ? sorter : [sorter])
      .filter(
        (item): item is SorterResult<IndexTaskInfo> & { order: Exclude<SortOrder, null>; columnKey: LogSorterKey } =>
          Boolean(
            item.order &&
              typeof item.columnKey === 'string' &&
              item.columnKey in priority,
          ),
      )
      .sort((left, right) => priority[left.columnKey] - priority[right.columnKey])
      .map((item) => ({ key: item.columnKey, order: item.order }))

    setActiveSorters(sorterList.length > 0 ? sorterList : [{ key: 'updated_at', order: 'descend' }])
  }

  const pagedTasks = sortedTasks.slice((page - 1) * logPageSize, page * logPageSize)
  const sortedLogs = [...logs].sort((left, right) => {
    let result = 0
    if (logSorter.key === 'filename') {
      result = left.filename.localeCompare(right.filename)
    } else {
      result = new Date(left.created_at).getTime() - new Date(right.created_at).getTime()
    }
    return logSorter.order === 'ascend' ? result : -result
  })
  const taskQueue = {
    pending: tasks.filter((task) => task.status === 'pending').length,
    running: tasks.filter((task) => task.status === 'running').length,
    success: tasks.filter((task) => task.status === 'success').length,
    failed: tasks.filter((task) => task.status === 'failed').length,
  }
  const failureReasons = Array.from(
    [...tasks, ...logs]
      .reduce((map, item) => {
        const raw = item.error_message?.trim()
        if (!raw) {
          return map
        }
        const reason = translateTaskErrorMessage(raw)
        map.set(reason, (map.get(reason) ?? 0) + 1)
        return map
      }, new Map<string, number>())
      .entries(),
  )
    .sort((left, right) => right[1] - left[1])
    .slice(0, 5)
  useEffect(() => {
    const nextTotal = Math.max(1, Math.ceil(sortedTasks.length / logPageSize))
    if (page > nextTotal) {
      setPage(nextTotal)
    }
  }, [page, sortedTasks.length])

  return (
    <div className="logs-page">
      <Card className="page-card" styles={{ body: { paddingTop: 16 } }}>
        <div className="knowledge-topology-heading">
          <div className="logs-live-status">
          <Space size={8}>
            <span className={live ? 'live-indicator is-active' : 'live-indicator'} />
            <Typography.Text strong>{t('logs.live')}</Typography.Text>
            <Switch
              size="small"
              checked={live}
              aria-label={t('logs.live')}
              onChange={setLive}
            />
          </Space>
          <Typography.Text type="secondary">
            {lastUpdatedAt
              ? t('logs.lastUpdated', { time: lastUpdatedAt.toLocaleTimeString() })
              : t('logs.waiting')}
          </Typography.Text>
          </div>
          <Space size={20} wrap>
          <div className="logs-health">
            <span>{t('logs.processing')}</span>
            <strong>{summary?.processing_total ?? 0}</strong>
          </div>
          <div className="logs-health">
            <span>{t('logs.failedDocuments')}</span>
            <strong className={(summary?.failed_total ?? 0) > 0 ? 'has-error' : ''}>
              {summary?.failed_total ?? 0}
            </strong>
          </div>
          <div className="logs-health">
            <span>{t('logs.indexRate')}</span>
            <strong>{indexRate}%</strong>
          </div>
        </Space>
      </div>
        <div className="knowledge-topology">
          {topology.map((node, index) => (
            <div className="topology-unit" key={node.key}>
              <div className={`topology-node ${node.active ? 'is-active' : ''}`}>
                <span className="topology-icon">{node.icon}</span>
                <span>{node.label}</span>
                <strong>{node.value}</strong>
              </div>
              {index < topology.length - 1 ? (
                <div className={`topology-connector ${node.flow ? 'is-active' : ''}`}>
                  <span />
                </div>
              ) : null}
            </div>
          ))}
        </div>
      </Card>

      <div className="logs-insights">
        <div className="logs-insight-panel logs-task-panel">
          <Typography.Text strong>{t('logs.taskQueue')}</Typography.Text>
          <div className="logs-insight-list">
            <div className="logs-insight-row">
              <Typography.Text>{t('tasks.status.pending')}</Typography.Text>
              <Tag>{taskQueue.pending}</Tag>
            </div>
            <div className="logs-insight-row">
              <Typography.Text>{t('tasks.status.running')}</Typography.Text>
              <Tag color="processing">{taskQueue.running}</Tag>
            </div>
            <div className="logs-insight-row">
              <Typography.Text>{t('logs.completed')}</Typography.Text>
              <Tag color="success">{taskQueue.success}</Tag>
            </div>
            <div className="logs-insight-row">
              <Typography.Text>{t('tasks.status.failed')}</Typography.Text>
              <Tag color="error">{taskQueue.failed}</Tag>
            </div>
          </div>
        </div>
        <div className="logs-insight-panel logs-failure-panel">
          <Typography.Text strong>{t('logs.topReasons')}</Typography.Text>
          <div className="logs-insight-list">
            {failureReasons.length > 0 ? failureReasons.map(([reason, count]) => (
              <div className="logs-insight-row" key={reason}>
                <Typography.Text className="logs-failure-reason" ellipsis={{ tooltip: reason }}>
                  {reason}
                </Typography.Text>
                <Tag>{count}</Tag>
              </div>
            )) : <Typography.Text type="secondary">{t('logs.emptyReasons')}</Typography.Text>}
          </div>
        </div>
      </div>

      <Card className="page-card logs-stream" styles={{ body: { paddingTop: 16 } }}>
        <div className="logs-table-actions">
          <Space size={8} wrap>
            <Segmented
              value={viewMode}
              options={[
                { label: t('documents.tasks'), value: 'tasks' },
                { label: t('documents.logs'), value: 'logs' },
              ]}
              onChange={(value) => {
                setViewMode(value as RuntimeViewMode)
                setPage(1)
              }}
            />
            <Select
              value={subjectFilter ?? '__all__'}
              style={{ minWidth: 240 }}
              options={[
                { label: t('logs.allSubjects'), value: '__all__' },
                ...subjects.map((subject) => ({ label: subject.name, value: subject.id })),
              ]}
              onChange={(value) => {
                setSubjectFilter(value === '__all__' ? undefined : value)
                setPage(1)
              }}
            />
          </Space>
          <Space size={8}>
            <Tooltip title={t('logs.clearAction')}>
              <Button
                danger
                icon={<ClearOutlined />}
                aria-label={t('logs.clearAction')}
                className="table-action-button"
                loading={clearing}
                disabled={viewMode === 'tasks' ? filteredTasks.length === 0 : logsTotal === 0}
                onClick={handleClear}
              />
            </Tooltip>
          </Space>
        </div>
        <div className="fixed-table-shell logs-table-shell">
          <div className="fixed-table-body">
            {viewMode === 'tasks' ? (
              <Table
                className="logs-table"
                rowKey="id"
                loading={loading}
                dataSource={pagedTasks}
                showSorterTooltip={false}
                pagination={false}
                tableLayout="fixed"
                onChange={handleTableChange}
                columns={[
                {
                  title: t('documents.fileName'),
                  key: 'filename',
                  dataIndex: 'filename',
                  width: 280,
                  ellipsis: true,
                  sortDirections: ['ascend', 'descend', 'ascend'],
                  sortIcon: renderSortIcon,
                  sortOrder:
                    activeSorters.find((sorter) => sorter.key === 'filename')?.order ?? null,
                  sorter: {
                    compare: (left, right) => left.filename.localeCompare(right.filename),
                    multiple: 1,
                  },
                  render: (filename) => (
                    <div className="log-event">
                      <Typography.Text strong ellipsis className="log-event-filename">
                        {filename || t('common.unknownDocument')}
                      </Typography.Text>
                    </div>
                  ),
                },
                {
                  title: t('documents.taskType'),
                  key: 'task_type',
                  dataIndex: 'task_type',
                  width: 116,
                  filteredValue: taskTypeFilter ? [taskTypeFilter] : null,
                  filters: [
                    'document.parse',
                    'document.chunk',
                    'document.embedding',
                    'document.delete',
                  ].map((value) => ({ value, text: t(`tasks.type.${value}`) })),
                  filterMultiple: false,
                  render: (value) => t(`tasks.type.${value}`),
                },
                {
                  title: t('documents.taskStatus'),
                  key: 'status',
                  dataIndex: 'status',
                  width: 116,
                  filteredValue: statusFilter ? [statusFilter] : null,
                  filters: ['pending', 'running', 'failed', 'success'].map((value) => ({
                    value,
                    text: t(`tasks.status.${value}`),
                  })),
                  filterMultiple: false,
                  render: (status, task) => (
                    <Space size={4}>
                      {statusTag(status)}
                      {status === 'failed' && task.error_message ? (
                        <Tooltip title={translateTaskErrorMessage(task.error_message)}>
                          <ExclamationCircleFilled className="log-event-error-icon" />
                        </Tooltip>
                      ) : null}
                      {status === 'failed' ? (
                        <Button
                          type="text"
                          size="small"
                          icon={<RetweetOutlined />}
                          aria-label={t('documents.retry')}
                          loading={retryingTaskIDs.includes(task.id)}
                          disabled={retryingTaskIDs.includes(task.id)}
                          onClick={() => void handleRetry(task.id)}
                        />
                      ) : null}
                    </Space>
                  ),
                },
                {
                  title: t('logs.duration'),
                  key: 'duration',
                  width: 104,
                  sortDirections: ['ascend', 'descend', 'ascend'],
                  sortIcon: renderSortIcon,
                  sortOrder:
                    activeSorters.find((sorter) => sorter.key === 'duration')?.order ?? null,
                  sorter: {
                    compare: (left, right) =>
                      new Date(left.updated_at).getTime() -
                      new Date(left.created_at).getTime() -
                      (new Date(right.updated_at).getTime() -
                        new Date(right.created_at).getTime()),
                    multiple: 2,
                  },
                  render: (_, task) =>
                    formatDuration(
                      task.created_at,
                      task.status === 'running' ? new Date().toISOString() : task.updated_at,
                    ),
                },
                {
                  title: t('logs.updatedAt'),
                  key: 'updated_at',
                  dataIndex: 'updated_at',
                  width: 152,
                  sortDirections: ['ascend', 'descend', 'ascend'],
                  sortIcon: renderSortIcon,
                  sortOrder:
                    activeSorters.find((sorter) => sorter.key === 'updated_at')?.order ?? null,
                  sorter: {
                    compare: (left, right) =>
                      new Date(left.updated_at).getTime() - new Date(right.updated_at).getTime(),
                    multiple: 3,
                  },
                  render: formatDateTime,
                },
                ]}
              />
            ) : (
              <Table
                className="logs-table"
                rowKey="id"
                loading={loading}
                dataSource={sortedLogs}
                pagination={false}
                showSorterTooltip={false}
                tableLayout="fixed"
                onChange={(_, __, sorter) => {
                  const nextSorter = Array.isArray(sorter) ? sorter[0] : sorter
                  if (
                    nextSorter &&
                    nextSorter.order &&
                    (nextSorter.columnKey === 'filename' || nextSorter.columnKey === 'created_at')
                  ) {
                    setLogSorter({
                      key: nextSorter.columnKey,
                      order: nextSorter.order,
                    })
                    return
                  }
                  setLogSorter({ key: 'created_at', order: 'descend' })
                }}
                columns={[
                  {
                    title: t('documents.fileName'),
                    key: 'filename',
                    dataIndex: 'filename',
                    width: 220,
                    ellipsis: true,
                    sortDirections: ['ascend', 'descend', 'ascend'],
                    sortIcon: renderSortIcon,
                    sortOrder: logSorter.key === 'filename' ? logSorter.order : null,
                    sorter: {
                      compare: (left, right) => left.filename.localeCompare(right.filename),
                    },
                    render: (filename) => (
                      <Typography.Text strong ellipsis className="log-event-filename">
                        {filename || t('common.unknownDocument')}
                      </Typography.Text>
                    ),
                  },
                  {
                    title: t('documents.taskStatus'),
                    dataIndex: 'status',
                    width: 116,
                    render: statusTag,
                  },
                  {
                    title: t('documents.logMessage'),
                    dataIndex: 'message',
                    ellipsis: true,
                    render: (value: string) => translateLogMessage(value),
                  },
                  {
                    title: t('documents.errorMessage'),
                    dataIndex: 'error_message',
                    ellipsis: true,
                    render: (value: string) =>
                      value ? (
                        <Typography.Text type="danger">{translateTaskErrorMessage(value)}</Typography.Text>
                      ) : (
                        '-'
                      ),
                  },
                  {
                    title: t('documents.createdAt'),
                    key: 'created_at',
                    dataIndex: 'created_at',
                    width: 152,
                    sortDirections: ['ascend', 'descend', 'ascend'],
                    sortIcon: renderSortIcon,
                    sortOrder: logSorter.key === 'created_at' ? logSorter.order : null,
                    sorter: {
                      compare: (left, right) =>
                        new Date(left.created_at).getTime() - new Date(right.created_at).getTime(),
                    },
                    render: formatDateTime,
                  },
                ]}
              />
            )}
          </div>
          <Pagination
            className="fixed-table-pagination"
            current={page}
            total={viewMode === 'tasks' ? sortedTasks.length : logsTotal}
            pageSize={logPageSize}
            showSizeChanger={false}
            showTotal={(total) => t('logs.resultCount', { count: total })}
            onChange={setPage}
          />
        </div>
      </Card>
    </div>
  )
}
