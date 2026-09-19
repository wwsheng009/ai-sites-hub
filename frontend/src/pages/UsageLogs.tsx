import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGetSite, apiListUsageDaily, apiListUsageLogs } from '../api/endpoints'
import type { Site, UsageDaily, UsageLog } from '../types'
import { timeDisplay, statusBadgeCls } from '../components/ui'
import { useToast, errMsg } from '../components/Toast'

const PAGE_SIZE_OPTIONS = [50, 100, 200, 500]
const DEFAULT_PAGE_SIZE = 100

// 列定义（借鉴 sub2api UsageTable 列设置机制）
type ColumnKey = 'ts' | 'model_name' | 'api_key_mask' | 'prompt_tokens' | 'completion_tokens' | 'total_tokens' | 'amount' | 'status' | 'err_code'
const ALL_COLUMNS: { key: ColumnKey; label: string; visible: boolean }[] = [
  { key: 'ts', label: '时间', visible: true },
  { key: 'model_name', label: '模型', visible: true },
  { key: 'api_key_mask', label: 'Key', visible: true },
  { key: 'prompt_tokens', label: '提示', visible: true },
  { key: 'completion_tokens', label: '补全', visible: true },
  { key: 'total_tokens', label: '总计', visible: true },
  { key: 'amount', label: '金额', visible: true },
  { key: 'status', label: '状态', visible: true },
  { key: 'err_code', label: '错误码', visible: false },
]

// 格式化 Token 数字
function formatTokens(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toLocaleString()
}

// 复制到剪贴板
async function copyToClipboard(text: string): Promise<void> {
  await navigator.clipboard.writeText(text)
}

export default function UsageLogs() {
  const { id } = useParams<{ id: string }>()
  const toast = useToast()

  const [site, setSite] = useState<Site | null>(null)
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [daily, setDaily] = useState<UsageDaily[]>([])
  const [loading, setLoading] = useState(true)
  const [dailyLoading, setDailyLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  // 过滤器
  const [start, setStart] = useState('')
  const [end, setEnd] = useState('')
  const [model, setModel] = useState('')

  // 分页
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE)
  const [total, setTotal] = useState(0)

  // 列可见性
  const [columnVisibility, setColumnVisibility] = useState<Record<ColumnKey, boolean>>(() => {
    const defaults: Record<ColumnKey, boolean> = {} as Record<ColumnKey, boolean>
    ALL_COLUMNS.forEach((c) => {
      defaults[c.key] = c.visible
    })
    return defaults
  })

  const visibleColumns = useMemo(() => ALL_COLUMNS.filter((c) => columnVisibility[c.key]), [columnVisibility])

  // 加载站点信息
  useEffect(() => {
    if (!id) return
    apiGetSite(id).then(setSite).catch(() => setSite(null))
  }, [id])

  // 加载用量日志（服务端分页）
  const loadLogs = useCallback(() => {
    if (!id) return
    setLoading(true)
    setErr(null)
    apiListUsageLogs(id, {
      start: start || undefined,
      end: end || undefined,
      model: model || undefined,
      limit: pageSize,
    })
      .then((data) => {
        setLogs(data)
        setTotal(data.length) // 后端暂不返回总数，显示当前页数量
        setLoading(false)
      })
      .catch((e) => {
        setErr(errMsg(e))
        setLoading(false)
      })
  }, [id, start, end, model, pageSize])

  // 加载日聚合数据（用于统计卡片 + 趋势）
  const loadDaily = useCallback(() => {
    if (!id) return
    setDailyLoading(true)
    const s = start || sevenDaysAgo()
    const e = end || today()
    apiListUsageDaily(id, { start: s, end: e, limit: 30 })
      .then((data) => {
        setDaily(data)
        setDailyLoading(false)
      })
      .catch(() => {
        setDailyLoading(false)
      })
  }, [id, start, end])

  useEffect(() => {
    loadLogs()
  }, [loadLogs])

  useEffect(() => {
    loadDaily()
  }, [loadDaily])

  const apply = () => {
    setPage(1)
    loadLogs()
    loadDaily()
  }

  const reset = () => {
    setStart('')
    setEnd('')
    setModel('')
    setPage(1)
  }

  // 统计卡片数据（从 daily 数据聚合）
  const stats = useMemo(() => {
    const totalRequests = daily.reduce((sum, d) => sum + (d.total_tokens > 0 ? 1 : 0), 0)
    const totalTokens = daily.reduce((sum, d) => sum + d.total_tokens, 0)
    const totalPrompt = daily.reduce((sum, d) => sum + d.prompt_tokens, 0)
    const totalCompletion = daily.reduce((sum, d) => sum + d.completion_tokens, 0)
    const totalAmount = daily.reduce((sum, d) => sum + d.amount, 0)
    return { totalRequests, totalTokens, totalPrompt, totalCompletion, totalAmount }
  }, [daily])

  // 趋势图数据（按日期排序）
  const trendData = useMemo(() => {
    return daily
      .slice()
      .sort((a, b) => (a.day < b.day ? -1 : 1))
      .map((d) => ({
        day: d.day,
        tokens: d.total_tokens,
        amount: d.amount,
      }))
  }, [daily])

  // 导出 CSV
  const exportCSV = () => {
    if (logs.length === 0) {
      toast.warning('暂无数据可导出')
      return
    }
    const headers = visibleColumns.map((c) => c.label)
    const rows = logs.map((l) =>
      visibleColumns.map((c) => {
        const v = l[c.key]
        if (c.key === 'amount') return `${v} ${l.currency}`
        if (c.key === 'ts') return timeDisplay(v as string)
        return v !== undefined && v !== null ? String(v) : '—'
      }),
    )
    const csvContent = [headers, ...rows]
      .map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(','))
      .join('\n')
    const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `usage_logs_${formatDate()}.csv`
    a.click()
    URL.revokeObjectURL(url)
    toast.success(`导出 ${logs.length} 条记录`)
  }

  // 复制 Key
  const copyKey = async (mask: string) => {
    await copyToClipboard(mask)
    toast.success('已复制到剪贴板')
  }

  // 切换列可见性
  const toggleColumn = (key: ColumnKey) => {
    setColumnVisibility((v) => ({ ...v, [key]: !v[key] }))
  }

  if (!id) return null

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div>
        <Link
          to="/sites"
          className="text-sm text-gray-500 hover:text-primary-600 dark:text-gray-400"
        >
          ← 返回站点列表
        </Link>
        <div className="mt-2 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            调用明细{site ? ` — ${site.name}` : ''}
          </h1>
        </div>
        <p className="mt-1 text-sm text-muted">
          站点用量日志（服务端分页，pageSize={pageSize}；currency 仅标明不折算）
        </p>
      </div>

      {/* 统计卡片网格（借鉴 sub2api UsageStatsCards） */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-primary">📋</div>
          <div className="min-w-0">
            <div className="stat-value">{stats.totalRequests.toLocaleString()}</div>
            <div className="stat-label">请求总数（{start ? start : '7天'} 内）</div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-success">🔢</div>
          <div className="min-w-0">
            <div className="stat-value">{formatTokens(stats.totalTokens)}</div>
            <div className="stat-label">
              输入 {formatTokens(stats.totalPrompt)} / 输出 {formatTokens(stats.totalCompletion)}
            </div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-warning">💰</div>
          <div className="min-w-0">
            <div className="stat-value text-xl">{stats.totalAmount.toFixed(4)}</div>
            <div className="stat-label">
              累计金额 <span className="text-xs text-muted">({daily[0]?.currency || 'quota'})</span>
            </div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-danger">📊</div>
          <div className="min-w-0">
            <div className="stat-value">{daily.length}</div>
            <div className="stat-label">聚合天数</div>
          </div>
        </div>
      </div>

      {/* 趋势图（借鉴 sub2api TokenUsageTrend） */}
      <div className="card p-4">
        <h2 className="mb-3 text-sm font-medium text-gray-600 dark:text-dark-300">Token 使用趋势</h2>
        {dailyLoading ? (
          <div className="py-8 text-center text-muted">加载中…</div>
        ) : trendData.length === 0 ? (
          <div className="py-8 text-center text-muted">暂无趋势数据</div>
        ) : (
          <div className="h-48">
            <svg viewBox="0 0 720 160" className="h-full w-full">
              {(() => {
                const maxTokens = Math.max(...trendData.map((d) => d.tokens), 1)
                const padding = 20
                const chartWidth = 680
                const chartHeight = 120
                const step = trendData.length > 1 ? chartWidth / (trendData.length - 1) : 0
                const points = trendData
                  .map((d, i) => {
                    const x = padding + i * step
                    const y = padding + chartHeight - (d.tokens / maxTokens) * chartHeight
                    return `${x},${y}`
                  })
                  .join(' ')
                return (
                  <g fill="none" stroke="currentColor" strokeWidth="1.5">
                    <polyline points={points} className="stroke-primary-500/60" />
                    {trendData.map((d, i) => {
                      const x = padding + i * step
                      const y = padding + chartHeight - (d.tokens / maxTokens) * chartHeight
                      return (
                        <circle key={i} cx={x} cy={y} r="3" className="fill-primary-500" />
                      )
                    })}
                  </g>
                )
              })()}
            </svg>
            <div className="mt-2 flex justify-between text-xs text-muted">
              {trendData.map((d) => (
                <span key={d.day}>{formatDayLabel(d.day)}</span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* 过滤器 + 操作栏 */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-[140px]">
            <label className="input-label">开始日期</label>
            <input
              className="input"
              type="date"
              value={start}
              onChange={(e) => setStart(e.target.value)}
            />
          </div>
          <div className="min-w-[140px]">
            <label className="input-label">结束日期</label>
            <input
              className="input"
              type="date"
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </div>
          <div className="min-w-[140px]">
            <label className="input-label">模型名</label>
            <input
              className="input"
              placeholder="gpt-4o"
              value={model}
              onChange={(e) => setModel(e.target.value)}
            />
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {/* 列设置下拉（借鉴 sub2api column settings） */}
          <div className="relative" data-col-dropdown>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={() => {
                const el = (event?.currentTarget as HTMLElement)?.nextElementSibling
                if (el) el.classList.toggle('hidden')
              }}
              title="列设置"
            >
              <span className="hidden sm:inline">列设置</span>
              <span className="sm:hidden">⚙</span>
            </button>
            <div className="absolute right-0 top-full z-50 mt-1 hidden w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800">
              {ALL_COLUMNS.filter((c) => c.key !== 'ts' && c.key !== 'model_name' && c.key !== 'status')
                .map((c) => (
                  <label
                    key={c.key}
                    className="flex items-center justify-between px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                  >
                    <span>{c.label}</span>
                    <input
                      type="checkbox"
                      checked={columnVisibility[c.key]}
                      onChange={() => toggleColumn(c.key)}
                      className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    />
                  </label>
                ))}
            </div>
          </div>

          <button
            className="btn btn-secondary btn-sm"
            onClick={exportCSV}
            disabled={loading || logs.length === 0}
          >
            导出 CSV
          </button>
          <button className="btn btn-secondary btn-sm" onClick={apply} disabled={loading}>
            {loading ? '加载中…' : '筛选'}
          </button>
          <button className="btn btn-secondary btn-sm" onClick={reset} disabled={loading}>
            重置
          </button>
        </div>
      </div>

      {/* 表格 */}
      {err ? (
        <p className="text-error">{err}</p>
      ) : (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                {visibleColumns.map((col) => (
                  <th key={col.key} className="whitespace-nowrap">
                    {col.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id}>
                  {visibleColumns.map((col) => {
                    const v = l[col.key] as string | number | undefined
                    let content: React.ReactNode
                    switch (col.key) {
                      case 'ts':
                        content = <span className="text-xs text-muted">{timeDisplay(v as string)}</span>
                        break
                      case 'model_name':
                        content = (
                          <span className="font-mono text-xs" title={v ? String(v) : undefined}>
                            {v || '—'}
                          </span>
                        )
                        break
                      case 'api_key_mask':
                        content = (
                          <div className="flex items-center gap-1">
                            <span
                              className="font-mono text-xs"
                              title={v ? String(v) : undefined}
                              onDoubleClick={() => v && copyKey(String(v))}
                            >
                              {v || '—'}
                            </span>
                            {v && (
                              <button
                                className="text-xs text-gray-400 hover:text-gray-600"
                                onClick={() => copyKey(String(v))}
                                title="复制"
                              >
                                📋
                              </button>
                            )}
                          </div>
                        )
                        break
                      case 'prompt_tokens':
                      case 'completion_tokens':
                      case 'total_tokens':
                        content = (
                          <span className="font-mono text-xs">
                            {v !== undefined && v !== null ? formatTokens(v as number) : '—'}
                          </span>
                        )
                        break
                      case 'amount':
                        content = (
                          <span className="font-mono text-xs">
                            {v !== undefined && v !== null ? (v as number).toFixed(4) : '—'}
                            <span className="text-xs text-muted"> ({l.currency})</span>
                          </span>
                        )
                        break
                      case 'status':
                        content = (
                          <span className={`badge ${statusBadgeCls(v as string)}`}>
                            {v || '—'}
                          </span>
                        )
                        break
                      case 'err_code':
                        content = (
                          <span className="font-mono text-xs text-gray-500" title={v ? String(v) : undefined}>
                            {v || '—'}
                          </span>
                        )
                        break
                      default:
                        content = v !== undefined && v !== null ? String(v) : '—'
                    }
                    return (
                      <td key={col.key} className="font-mono text-xs">
                        {content}
                      </td>
                    )
                  })}
                </tr>
              ))}
              {logs.length === 0 && !loading && (
                <tr>
                  <td colSpan={visibleColumns.length}>
                    <div className="empty-state">
                      <span className="text-3xl">📊</span>
                      <span className="empty-state-title">暂无用量日志</span>
                      <span className="empty-state-desc">
                        尚未同步或筛选条件不匹配
                      </span>
                    </div>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* 分页控件 */}
      {!loading && logs.length > 0 && (
        <div className="flex items-center justify-between">
          <div className="text-sm text-muted">
            显示 {Math.min((page - 1) * pageSize + 1, total)}-{Math.min(page * pageSize, total)} 条，
            共 {total} 条
          </div>
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted">每页</span>
            <select
              className="input w-auto"
              value={pageSize}
              onChange={(e) => setPageSize(Number(e.target.value))}
            >
              {PAGE_SIZE_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>
        </div>
      )}
    </div>
  )
}

// 辅助函数
function sevenDaysAgo(): string {
  const d = new Date()
  d.setDate(d.getDate() - 7)
  return d.toISOString().slice(0, 10)
}

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

function formatDate(): string {
  return new Date().toISOString().slice(0, 10)
}

function formatDayLabel(day: string): string {
  const d = new Date(day)
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${m}-${dd}`
}
