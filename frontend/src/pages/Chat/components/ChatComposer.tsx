import { Button, Dropdown, Input, Select, Switch, Tooltip } from 'antd'
import { DownOutlined, GlobalOutlined, RightOutlined, SendOutlined } from '@ant-design/icons'
import type { KeyboardEvent } from 'react'
import type { SubjectInfo } from '../../../api/subjects'
import { useI18n } from '../../../useI18n'

type Option = { label: string; value: string }
type ModelChild = { label: string; value: string }
type ModelOption = { label: string; value: string; children?: ModelChild[] }

type Props = {
  asking: boolean
  question: string
  subjectID: string
  subjects: SubjectInfo[]
  subjectOptions: Option[]
  vendorOptions: Option[]
  modelOptions: ModelOption[]
  activeModelLabel: string
  activeModelValue: string
  activeVendor: string
  subjectSelectOpen: boolean
  vendorSelectOpen: boolean
  modelSelectOpen: boolean
  expandedSeriesKey: string
  expandedSeriesIndex: number
  webSearch: boolean
  onQuestionChange: (value: string) => void
  onInputKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void
  onSubjectChange: (value: string) => void
  onSubjectOpenChange: (open: boolean) => void
  onWebSearchChange: (checked: boolean) => void
  onModelDropdownOpenChange: (open: boolean) => void
  onVendorChange: (value: string) => void
  onVendorOpenChange: (open: boolean) => void
  onModelChange: (value: string) => void
  onSeriesClick: (seriesKey: string, index: number) => void
  onAsk: () => void
}

export function ChatComposer({
  asking,
  question,
  subjectID,
  subjects,
  subjectOptions,
  vendorOptions,
  modelOptions,
  activeModelLabel,
  activeModelValue,
  activeVendor,
  subjectSelectOpen,
  vendorSelectOpen,
  modelSelectOpen,
  expandedSeriesKey,
  expandedSeriesIndex,
  webSearch,
  onQuestionChange,
  onInputKeyDown,
  onSubjectChange,
  onSubjectOpenChange,
  onWebSearchChange,
  onModelDropdownOpenChange,
  onVendorChange,
  onVendorOpenChange,
  onModelChange,
  onSeriesClick,
  onAsk,
}: Props) {
  const { t } = useI18n()
  const expandedSeries = modelOptions.find((option) => option.value === expandedSeriesKey)

  return (
    <div className="chat-composer">
      {!question && !asking ? (
        <div className="chat-suggestions" aria-label={t('chat.suggestions.title')}>
          <span>{t('chat.suggestions.title')}</span>
          {[
            t('chat.suggestions.overview'),
            t('chat.suggestions.useCases'),
          ].map((suggestion) => (
            <button key={suggestion} type="button" onClick={() => onQuestionChange(suggestion)}>
              {suggestion}
            </button>
          ))}
        </div>
      ) : null}
      <Input.TextArea
        value={question}
        onChange={(event) => onQuestionChange(event.target.value)}
        onKeyDown={onInputKeyDown}
        placeholder={t('chat.inputPlaceholder')}
        autoSize={{ minRows: 1, maxRows: 10 }}
        disabled={asking}
        className="chat-input"
      />
      <div className="chat-composer-row">
        <div className="chat-composer-left">
          <Select
            className="chat-subject-select"
            placeholder={t('chat.selectSubject')}
            value={subjectID && subjects.some((subject) => subject.id === subjectID) ? subjectID : undefined}
            options={subjectOptions}
            onChange={onSubjectChange}
            disabled={asking}
            open={subjectSelectOpen}
            onOpenChange={onSubjectOpenChange}
            popupMatchSelectWidth={false}
            placement="topLeft"
            variant="borderless"
          />
          <Tooltip title={t('chat.webSearchHint')}>
            <label className="chat-web-search">
              <GlobalOutlined />
              <Switch
                size="small"
                checked={webSearch}
                disabled={asking}
                aria-label={t('chat.webSearch')}
                onChange={onWebSearchChange}
              />
            </label>
          </Tooltip>
        </div>
        <div className="chat-composer-right">
          <Dropdown
            open={modelSelectOpen}
            onOpenChange={onModelDropdownOpenChange}
            placement="topLeft"
            trigger={['click']}
            disabled={asking}
            dropdownRender={() => (
              <div className="chat-model-dropdown">
                <div className="chat-model-dropdown-menu">
                  {modelOptions.map((option, index) => (
                    <button
                      key={option.value}
                      type="button"
                      className={`chat-model-series-item${option.value === activeModelValue ? ' is-selected' : ''}${option.value === expandedSeriesKey ? ' is-expanded' : ''}`}
                      onClick={() => {
                        if (!option.children?.length) {
                          onModelChange(option.value)
                          return
                        }
                        onSeriesClick(option.value, index)
                      }}
                    >
                      <span>{option.label}</span>
                      {option.children?.length ? <RightOutlined className="chat-model-series-arrow" /> : null}
                    </button>
                  ))}
                </div>
                {expandedSeries?.children?.length ? (
                  <div
                    className="chat-model-dropdown-submenu"
                    style={{ top: `${8 + Math.max(0, expandedSeriesIndex) * 42}px` }}
                  >
                    {expandedSeries.children.map((child) => (
                      <button
                        key={child.value}
                        type="button"
                        className={`chat-model-variant-item${child.value === activeModelValue ? ' is-selected' : ''}`}
                        onClick={() => onModelChange(child.value)}
                      >
                        {child.label}
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>
            )}
          >
            <Button type="text" className="chat-model-select" disabled={asking}>
              <span>{activeModelLabel}</span>
              <DownOutlined />
            </Button>
          </Dropdown>
          <Select
            className="chat-vendor-select"
            value={activeVendor}
            options={vendorOptions}
            onChange={onVendorChange}
            disabled={asking}
            open={vendorSelectOpen}
            onOpenChange={onVendorOpenChange}
            popupMatchSelectWidth={false}
            placement="topLeft"
            popupClassName="chat-vendor-dropdown"
            virtual={false}
            variant="borderless"
          />
          <Button type="primary" icon={<SendOutlined />} loading={asking} aria-label={t('chat.send')} onClick={onAsk} />
        </div>
      </div>
    </div>
  )
}
