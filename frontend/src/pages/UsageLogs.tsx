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
const ALL_COLUMNS: { key: ColumnKey; label: string; defaultVisible: boolean }[] = [
  { key: 'ts', label: '时间', defaultVisible: true },
  { key: 'model_name', label: '模型', defaultVisible: true },
  { key: 'api_key_mask', label: 'Key', defaultVisible: true },
  { key: 'prompt_tokens', label: '提示', defaultVisible: true },
  { key: 'completion_tokens', label: '补全', defaultVisible: true },
  { key: 'total_tokens', label: '总计', defaultVisible: true },
  { key: 'amount', label: '金额', defaultVisible: true },
  { key: 'status', label: '状态', defaultVisible: true },
  { key: 'err_code', label: '错误码', defaultVisible: false },
]

// 日期预设（借鉴 sub2api DateRangePicker presets）
const DATE_PRESETS = [
  { label: '今天', value: 'today' },
  { label: '昨天', value: 'yesterday' },
  { label: '最近 24 小时', value: 'last24h' },
  { label: '最近 7 天', value: 'last7d' },
  { label: '最近 14 天', value: 'last14d' },
  { label: '最近 30 天', value: 'last30d' },
]

// 格式化 Token 数字（借鉴 sub2api formatTokens）
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

// 格式化日期为 YYYY-MM-DD（本地时区）
function formatDateLocal(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function today(): string {
  return formatDateLocal(new Date())
}

function sevenDaysAgo(): string {
  const d = new Date()
  d.setDate(d.getDate() - 7)
  return formatDateLocal(d)
}

// 应用日期预设
function applyDatePreset(preset: string): { start: string; end: string } {
  const now = new Date()
  const t = today()
  switch (preset) {
    case 'today':
      return { start: t, end: t }
    case 'yesterday': {
      const d = new Date()
      d.setDate(d.getDate() - 1)
      const y = formatDateLocal(d)
      return { start: y, end: y }
    }
    case 'last24h': {
      const end = now
      const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
      return { start: formatDateLocal(start), end: formatDateLocal(end) }
    }
    case 'last7d':
      return { start: sevenDaysAgo(), end: t }
    case 'last14d': {
      const d = new Date()
      d.setDate(d.getDate() - 13)
      return { start: formatDateLocal(d), end: t }
    }
    case 'last30d': {
      const d = new Date()
      d.setDate(d.getDate() - 29)
      return { start: formatDateLocal(d), end: t }
    }
    default:
      return { start: sevenDaysAgo(), end: t }
  }
}

// 计算可见页码（借鉴 sub2api visiblePages）
function getVisiblePages(current: number, total: number): (number | string)[] {
  const pages: (number | string)[] = []
  const maxVisible = 7
  if (total <= maxVisible) {
    for (let i = 1; i <= total; i++) pages.push(i)
  } else {
    pages.push(1)
    const start = Math.max(2, current - 2)
    const end = Math.min(total - 1, current + 2)
    if (start > 2) pages.push('...')
    for (let i = start; i <= end; i++) pages.push(i)
    if (end < total - 1) pages.push('...')
    pages.push(total)
  }
  return pages
}

export default function UsageLogs() {
  const { id } = useParams<{ id: string }>()
  const toast = useToast()

  const [site, setSite] = useState<Site | null>(null)
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [daily, setDaily] = useState<UsageDaily[]>([])
  const [statsDaily, setStatsDaily] = useState<UsageDaily[]>([])
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

  // 列可见性（借鉴 sub2api column settings + localStorage 持久化）
  const [columnVisibility, setColumnVisibility] = useState<Record<ColumnKey, boolean>>(() => {
    const defaults: Record<ColumnKey, boolean> = {} as Record<ColumnKey, boolean>
    ALL_COLUMNS.forEach((c) => {
      defaults[c.key] = c.defaultVisible
    })
    // 尝试从 localStorage 恢复
    try {
      const saved = localStorage.getItem('usage-logs-hidden-columns')
      if (saved) {
        const hidden = JSON.parse(saved) as string[]
        hidden.forEach((k) => {
          defaults[k as ColumnKey] = false
        })
      }
    } catch {
      // ignore
    }
    return defaults
  })

  const visibleColumns = useMemo(
    () => ALL_COLUMNS.filter((c) => columnVisibility[c.key]),
    [columnVisibility],
  )

  // 切换列可见性
  const toggleColumn = (key: ColumnKey) => {
    const newVis = { ...columnVisibility, [key]: !columnVisibility[key] }
    setColumnVisibility(newVis)
    // 持久化到 localStorage
    const hidden = ALL_COLUMNS.filter((c) => !newVis[c.key]).map((c) => c.key)
    try {
      localStorage.setItem('usage-logs-hidden-columns', JSON.stringify(hidden))
    } catch {
      // ignore
    }
  }

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
  // 统计卡片使用全量数据（最近 30 天），趋势图使用过滤后的日期范围
  const loadDaily = useCallback(() => {
    if (!id) return
    setDailyLoading(true)
    const s = start || sevenDaysAgo()
    const e = end || today()
    // 趋势图：使用过滤后的日期范围
    apiListUsageDaily(id, { start: s, end: e, limit: 30 })
      .then((data) => {
        setDaily(data)
        setDailyLoading(false)
      })
      .catch(() => {
        setDailyLoading(false)
      })
    // 统计卡片：使用最近 30 天的全量数据
    const statsStart = new Date()
    statsStart.setDate(statsStart.getDate() - 30)
    apiListUsageDaily(id, {
      start: formatDateLocal(statsStart),
      end: today(),
      limit: 30,
    })
      .then((data) => {
        setStatsDaily(data)
      })
      .catch(() => {})
  }, [id, start, end])

  useEffect(() => {
    loadLogs()
  }, [loadLogs])

  useEffect(() => {
    loadDaily()
  }, [loadDaily])

  // 应用日期预设
  const applyPreset = (preset: string) => {
    const range = applyDatePreset(preset)
    setStart(range.start)
    setEnd(range.end)
  }

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
    loadLogs()
    loadDaily()
  }

  // 统计卡片数据（从 statsDaily 全量数据聚合，借鉴 sub2api UsageStatsCards）
  // sub2api 页面展示「今日 vs 累计」两行网格，这里也采用相同模式
  const today = useMemo(() => {
    const t = today()
    return statsDaily.filter((d) => d.day.slice(0, 10) === t)
  }, [statsDaily])

  const stats = useMemo(() => {
    const totalRequests = statsDaily.reduce((sum, d) => sum + d.request_count, 0)
    const totalTokens = statsDaily.reduce((sum, d) => sum + d.total_tokens, 0)
    const totalPrompt = statsDaily.reduce((sum, d) => sum + d.prompt_tokens, 0)
    const totalCompletion = statsDaily.reduce((sum, d) => sum + d.completion_tokens, 0)
    const totalAmount = statsDaily.reduce((sum, d) => sum + d.amount, 0)
    const todayRequests = today.reduce((sum, d) => sum + d.request_count, 0)
    const todayTokens = today.reduce((sum, d) => sum + d.total_tokens, 0)
    const todayAmount = today.reduce((sum, d) => sum + d.amount, 0)
    return {
      totalRequests, totalTokens, totalPrompt, totalCompletion, totalAmount,
      todayRequests, todayTokens, todayAmount,
    }
  }, [statsDaily, today])

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

  // 模型统计（按 model_name 汇总，借鉴 sub2api ModelStatsTable）
  const modelStats = useMemo(() => {
    const byModel = new Map<string, UsageDaily>()
    for (const d of statsDaily) {
      const key = d.model_name || ''
      const existing = byModel.get(key)
      if (existing) {
        byModel.set(key, {
          ...existing,
          prompt_tokens: existing.prompt_tokens + d.prompt_tokens,
          completion_tokens: existing.completion_tokens + d.completion_tokens,
          total_tokens: existing.total_tokens + d.total_tokens,
          amount: existing.amount + d.amount,
          request_count: existing.request_count + d.request_count,
        })
      } else {
        byModel.set(key, { ...d })
      }
    }
    return Array.from(byModel.values()).sort((a, b) => b.total_tokens - a.total_tokens)
  }, [statsDaily])

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
    a.download = `usage_logs_${formatDateLocal(new Date())}.csv`
    a.click()
    URL.revokeObjectURL(url)
    toast.success(`导出 ${logs.length} 条记录`)
  }

  // 复制 Key
  const copyKey = async (mask: string) => {
    await copyToClipboard(mask)
    toast.success('已复制到剪贴板')
  }

  // 分页导航
  const totalPages = Math.ceil(total / pageSize)
  const goToPage = (newPage: number) => {
    if (newPage >= 1 && newPage <= totalPages && newPage !== page) {
      setPage(newPage)
    }
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

      {/* 统计卡片网格（借鉴 sub2api UsageStatsCards — 今日 vs 累计两行） */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-primary">📋</div>
          <div className="min-w-0">
            <div className="stat-value">{stats.todayRequests.toLocaleString()}</div>
            <div className="stat-label">今日请求</div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-success">📋</div>
          <div className="min-w-0">
            <div className="stat-value">{stats.totalRequests.toLocaleString()}</div>
            <div className="stat-label">累计请求（最近 30 天）</div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-info">🔢</div>
          <div className="min-w-0">
            <div className="stat-value">{formatTokens(stats.todayTokens)}</div>
            <div className="stat-label">今日 Token</div>
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
      </div>

      {/* 金额卡片 */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-warning">💰</div>
          <div className="min-w-0">
            <div className="stat-value text-xl">{stats.todayAmount.toFixed(4)}</div>
            <div className="stat-label">今日金额</div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-danger">💰</div>
          <div className="min-w-0">
            <div className="stat-value text-xl">{stats.totalAmount.toFixed(4)}</div>
            <div className="stat-label">
              累计金额{' '}
              <span className="text-xs text-muted">
                ({daily[0]?.currency || 'quota'})
              </span>
            </div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-secondary">📊</div>
          <div className="min-w-0">
            <div className="stat-value">{statsDaily.length}</div>
            <div className="stat-label">聚合天数</div>
          </div>
        </div>
      </div>

      {/* 趋势图（借鉴 sub2api TokenUsageTrend） */}
      <div className="card p-4">
        <h2 className="mb-3 text-sm font-medium text-gray-600 dark:text-dark-300">
          Token 使用趋势
        </h2>
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
                const step =
                  trendData.length > 1 ? chartWidth / (trendData.length - 1) : chartWidth / 2
                const points = trendData
                  .map((d, i) => {
                    const x = padding + i * step
                    const y =
                      padding +
                      chartHeight -
                      (d.tokens / maxTokens) * chartHeight
                    return `${x},${y}`
                  })
                  .join(' ')
                return (
                  <g fill="none" stroke="currentColor" strokeWidth="1.5">
                    <polyline
                      points={points}
                      className="stroke-primary-500/60"
                    />
                    {trendData.map((d, i) => {
                      const x = padding + i * step
                      const y =
                        padding +
                        chartHeight -
                        (d.tokens / maxTokens) * chartHeight
                      return (
                        <circle
                          key={i}
                          cx={x}
                          cy={y}
                          r="3"
                          className="fill-primary-500"
                        />
                      )
                    })}
                  </g>
                )
              })()}
            </svg>
            <div className="mt-2 flex justify-between text-xs text-muted">
              {trendData.map((d) => (
                <span key={d.day}>
                  {d.day.slice(5).replace('-', '/')}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* 每日用量表（借鉴 sub2api DailyUsageTable） */}
      <div className="card p-4">
        <h2 className="mb-3 text-sm font-medium text-gray-600 dark:text-dark-300">
          每日用量汇总
        </h2>
        {daily.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted">暂无每日汇总数据</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="table">
              <thead>
                <tr>
                  <th className="whitespace-nowrap">日期</th>
                  <th className="whitespace-nowrap text-right">请求数</th>
                  <th className="whitespace-nowrap text-right">提示词</th>
                  <th className="whitespace-nowrap text-right">补全词</th>
                  <th className="whitespace-nowrap text-right">总计</th>
                  <th className="whitespace-nowrap text-right">金额</th>
                </tr>
              </thead>
              <tbody>
                {daily
                  .slice()
                  .sort((a, b) => (a.day < b.day ? -1 : 1))
                  .map((d) => (
                    <tr key={`${d.day}-${d.model_name}`}>
                      <td className="text-sm">{d.day.slice(5).replace('-', '/')}</td>
                      <td className="text-right font-mono text-sm tabular-nums">{d.request_count.toLocaleString()}</td>
                      <td className="text-right font-mono text-sm tabular-nums">{d.prompt_tokens.toLocaleString()}</td>
                      <td className="text-right font-mono text-sm tabular-nums">{d.completion_tokens.toLocaleString()}</td>
                      <td className="text-right font-mono text-sm tabular-nums">{d.total_tokens.toLocaleString()}</td>
                      <td className="text-right font-mono text-sm tabular-nums">{d.amount.toFixed(4)}</td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* 过滤器 + 操作栏（借鉴 sub2api UsageView 过滤器区域） */}
      <div className="card p-4">
        <div className="flex flex-wrap items-end gap-4">
          {/* 日期预设按钮组 */}
          <div className="flex flex-wrap items-end gap-1.5">
            {DATE_PRESETS.map((p) => (
              <button
                key={p.value}
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => applyPreset(p.value)}
              >
                {p.label}
              </button>
            ))}
          </div>

          {/* 自定义日期范围 */}
          <div className="flex items-end gap-2">
            <div className="min-w-[120px]">
              <label className="input-label">开始日期</label>
              <input
                className="input"
                type="date"
                value={start}
                onChange={(e) => setStart(e.target.value)}
              />
            </div>
            <div className="min-w-[120px]">
              <label className="input-label">结束日期</label>
              <input
                className="input"
                type="date"
                value={end}
                onChange={(e) => setEnd(e.target.value)}
              />
            </div>
          </div>

          {/* 模型筛选 */}
          <div className="min-w-[140px]">
            <label className="input-label">模型名</label>
            <input
              className="input"
              placeholder="gpt-4o"
              value={model}
              onChange={(e) => setModel(e.target.value)}
            />
          </div>

          {/* 操作按钮 */}
          <div className="ml-auto flex flex-wrap items-center gap-2">
            {/* 列设置下拉（借鉴 sub2api column settings） */}
            <div className="relative">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => {
                  const dropdown = document.getElementById('column-dropdown')
                  if (dropdown) dropdown.classList.toggle('hidden')
                }}
                title="列设置"
              >
                <span className="hidden sm:inline">列设置</span>
                <span className="sm:hidden">⚙</span>
              </button>
              <div
                id="column-dropdown"
                className="absolute right-0 top-full z-50 mt-1 hidden w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
              >
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
            <button
              className="btn btn-secondary btn-sm"
              onClick={apply}
              disabled={loading}
            >
              {loading ? '加载中…' : '筛选'}
            </button>
            <button
              className="btn btn-secondary btn-sm"
              onClick={reset}
              disabled={loading}
            >
              重置
            </button>
          </div>
        </div>
      </div>

      {/* 模型统计表（借鉴 sub2api ModelStatsTable） */}
      <div className="card p-4">
        <h2 className="mb-3 text-sm font-medium text-gray-600 dark:text-dark-300">
          模型用量统计
        </h2>
        {statsDaily.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted">暂无模型统计数据</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="table">
              <thead>
                <tr>
                  <th className="whitespace-nowrap">模型</th>
                  <th className="whitespace-nowrap text-right">请求数</th>
                  <th className="whitespace-nowrap text-right">提示词</th>
                  <th className="whitespace-nowrap text-right">补全词</th>
                  <th className="whitespace-nowrap text-right">总计</th>
                  <th className="whitespace-nowrap text-right">金额</th>
                </tr>
              </thead>
              <tbody>
                {modelStats.map((m) => (
                  <tr key={m.model_name}>
                    <td className="font-mono text-xs">{m.model_name || '—'}</td>
                    <td className="text-right font-mono text-sm tabular-nums">{m.request_count.toLocaleString()}</td>
                    <td className="text-right font-mono text-sm tabular-nums">{m.prompt_tokens.toLocaleString()}</td>
                    <td className="text-right font-mono text-sm tabular-nums">{m.completion_tokens.toLocaleString()}</td>
                    <td className="text-right font-mono text-sm tabular-nums">{m.total_tokens.toLocaleString()}</td>
                    <td className="text-right font-mono text-sm tabular-nums">{m.amount.toFixed(4)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* 表格（借鉴 sub2api UsageTable） */}
      {err ? (
        <p className="text-error">{err}</p>
      ) : (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                {visibleColumns.map((col) => (
                  <th
                    key={col.key}
                    className="whitespace-nowrap"
                  >
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
                        content = (
                          <span className="text-xs text-muted">
                            {timeDisplay(v as string)}
                          </span>
                        )
                        break
                      case 'model_name':
                        content = (
                          <span
                            className="font-mono text-xs"
                            title={v ? String(v) : undefined}
                          >
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
                            {v !== undefined && v !== null
                              ? formatTokens(v as number)
                              : '—'}
                          </span>
                        )
                        break
                      case 'amount':
                        content = (
                          <span className="font-mono text-xs">
                            {v !== undefined && v !== null
                              ? (v as number).toFixed(4)
                              : '—'}
                            <span className="text-xs text-muted">
                              {' '}
                              ({l.currency})
                            </span>
                          </span>
                        )
                        break
                      case 'status':
                        content = (
                          <span
                            className={`badge ${statusBadgeCls(v as string)}`}
                          >
                            {v || '—'}
                          </span>
                        )
                        break
                      case 'err_code':
                        content = (
                          <span
                            className="font-mono text-xs text-gray-500"
                            title={v ? String(v) : undefined}
                          >
                            {v || '—'}
                          </span>
                        )
                        break
                      default:
                        content =
                          v !== undefined && v !== null ? String(v) : '—'
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
                      <span className="empty-state-title">
                        暂无用量日志
                      </span>
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

      {/* 分页控件（借鉴 sub2api Pagination） */}
      {!loading && total > 0 && (
        <div className="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-dark-700 sm:px-6">
          <div className="flex items-center gap-4">
            <p className="text-sm text-gray-700 dark:text-gray-300">
              显示{' '}
              <span className="font-medium">
                {(page - 1) * pageSize + 1}
              </span>
              -
              <span className="font-medium">
                {Math.min(page * pageSize, total)}
              </span>
              个，共{' '}
              <span className="font-medium">{total}</span> 条记录
            </p>

            {/* 页大小选择器 */}
            <div className="flex items-center gap-2">
              <span className="text-sm text-gray-500 dark:text-dark-400">
                每页
              </span>
              <select
                className="input w-auto"
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value))
                  setPage(1)
                }}
              >
                {PAGE_SIZE_OPTIONS.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* 页码导航 */}
          <nav
            className="relative z-0 inline-flex -space-x-px rounded-md shadow-sm"
            aria-label="分页"
          >
            <button
              onClick={() => goToPage(page - 1)}
              disabled={page === 1}
              className="relative inline-flex items-center rounded-l-md border border-gray-300 bg-white px-2.5 py-2 text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400"
              aria-label="上一页"
            >
              ‹
            </button>
            {getVisiblePages(page, totalPages).map((pageNum, i) =>
              typeof pageNum === 'number' ? (
                <button
                  key={`${pageNum}-${i}`}
                  onClick={() => goToPage(pageNum)}
                  className={`relative inline-flex items-center border px-4 py-2 text-sm font-medium ${
                    pageNum === page
                      ? 'z-10 border-primary-500 bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400'
                      : 'border-gray-300 bg-white text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
                  }`}
                  aria-label={`第 ${pageNum} 页`}
                  aria-current={pageNum === page ? 'page' : undefined}
                >
                  {pageNum}
                </button>
              ) : (
                <span
                  key={`ellipsis-${i}`}
                  className="relative inline-flex items-center border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-500 dark:border-dark-600 dark:bg-dark-800"
                >
                  …
                </span>
              ),
            )}
            <button
              onClick={() => goToPage(page + 1)}
              disabled={page === totalPages}
              className="relative inline-flex items-center rounded-r-md border border-gray-300 bg-white px-2.5 py-2 text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400"
              aria-label="下一页"
            >
              ›
            </button>
          </nav>
        </div>
      )}
    </div>
  )
}
