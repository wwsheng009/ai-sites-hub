import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGetSite, apiListUsageDaily, apiListUsageLogs } from '../api/endpoints'
import type { Site, UsageDaily, UsageLog } from '../types'
import {
  timeDisplay,
  statusBadgeCls,
  formatTokens,
  formatCacheTokens,
  formatDuration,
  formatMultiplier,
  formatDateTimeFull,
  firstTokenSeverity,
  durationSeverity,
  LATENCY_TEXT_CLS,
  LATENCY_BAR_CLS,
  LATENCY_BAR_FROM_CLS,
  LATENCY_BAR_TO_CLS,
  requestTypeLabel,
  requestTypeBadgeCls,
} from '../components/ui'
import { useToast, errMsg } from '../components/Toast'
import Modal from '../components/Modal'

const PAGE_SIZE_OPTIONS = [50, 100, 200, 500]
const DEFAULT_PAGE_SIZE = 100

// 列定义（借鉴 sub2api UsageTable 列设置机制；token/费用/延迟为复合列）
type ColumnKey =
  | 'ts'
  | 'model_name'
  | 'api_key_mask'
  | 'tokens'
  | 'cost'
  | 'latency'
  | 'request_type'
  | 'status'
  | 'err_code'
  | 'inbound_endpoint'
const ALL_COLUMNS: { key: ColumnKey; label: string; defaultVisible: boolean }[] = [
  { key: 'ts', label: '时间', defaultVisible: true },
  { key: 'model_name', label: '模型', defaultVisible: true },
  { key: 'api_key_mask', label: 'Key', defaultVisible: true },
  { key: 'tokens', label: 'Token（输入/输出/缓存）', defaultVisible: true },
  { key: 'cost', label: '费用', defaultVisible: true },
  { key: 'latency', label: '延迟', defaultVisible: true },
  { key: 'request_type', label: '类型', defaultVisible: true },
  { key: 'status', label: '状态', defaultVisible: true },
  { key: 'err_code', label: '错误码', defaultVisible: false },
  { key: 'inbound_endpoint', label: '端点', defaultVisible: false },
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

// ============ 行详情面板（点击行内字段弹出；不再表内展开）============

/** 空值判定：空串 / null / undefined（含纯空白串）都算空 */
function isEmptyValue(v: unknown): boolean {
  return v === null || v === undefined || (typeof v === 'string' && v.trim() === '')
}

/** 文本：空值统一显示 — */
function fmtText(v: string | null | undefined): string {
  return isEmptyValue(v) ? '—' : String(v).trim()
}

/** 布尔：false 是有效值（显示「否」），仅缺失时显示 — */
function fmtBool(v: boolean | null | undefined): string {
  if (v === true) return '是'
  if (v === false) return '否'
  return '—'
}

/** 数字：缺失 / NaN 显示 —；省略 digits 时按本地千分位展示 */
function fmtNum(v: number | null | undefined, digits?: number): string {
  if (typeof v !== 'number' || Number.isNaN(v)) return '—'
  return digits === undefined ? v.toLocaleString() : v.toFixed(digits)
}

/** 计费倍率：0 / 缺失视为未知（formatMultiplier(0) 会误显示 1.00） */
function fmtMultiplierValue(v: number | null | undefined): string {
  if (typeof v !== 'number' || !Number.isFinite(v) || v <= 0) return '—'
  return formatMultiplier(v)
}

/** 详情分区中的单个键值对 */
function DetailField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="shrink-0 text-muted">{label}</dt>
      <dd className="min-w-0 break-all text-right font-mono tabular-nums text-gray-700 dark:text-gray-300">
        {children}
      </dd>
    </div>
  )
}

/** 详情分区（标题 + 键值对列表） */
function DetailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0 rounded-lg border border-gray-100 bg-white/70 p-3 dark:border-dark-700 dark:bg-dark-800/40">
      <div className="mb-2 text-[11px] font-semibold tracking-wide text-gray-500 dark:text-dark-300">
        {title}
      </div>
      <dl className="space-y-1 text-xs">{children}</dl>
    </div>
  )
}

/** 行内详情主体：账号 / 请求 / 计费 / 缓存与延迟 / 图片 / 客户端 */
function UsageLogDetail({ log }: { log: UsageLog }) {
  // image_size_breakdown：键值对形式（如 1024x1024: 3）；null / 空对象显示 —
  const sizeBreakdown =
    log.image_size_breakdown && typeof log.image_size_breakdown === 'object'
      ? Object.entries(log.image_size_breakdown)
          .filter(([, n]) => typeof n === 'number' && !Number.isNaN(n))
          .map(([k, n]) => `${k}: ${n.toLocaleString()}`)
          .join('、')
      : ''

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <DetailSection title="账号">
        <DetailField label="用户 ID">{fmtText(log.user_id)}</DetailField>
        <DetailField label="账号 ID">{fmtText(log.account_id)}</DetailField>
        <DetailField label="API Key ID">{fmtText(log.api_key_id)}</DetailField>
        <DetailField label="订阅 ID">{fmtText(log.subscription_id)}</DetailField>
      </DetailSection>

      <DetailSection title="请求">
        <DetailField label="请求类型">{fmtText(log.request_type)}</DetailField>
        <DetailField label="流式">{fmtBool(log.stream)}</DetailField>
        <DetailField label="计费模式">{fmtText(log.billing_mode)}</DetailField>
        <DetailField label="服务层级">{fmtText(log.service_tier)}</DetailField>
        <DetailField label="推理强度">{fmtText(log.reasoning_effort)}</DetailField>
        <DetailField label="入站端点">{fmtText(log.inbound_endpoint)}</DetailField>
        <DetailField label="上游端点">{fmtText(log.upstream_endpoint)}</DetailField>
        <DetailField label="分组 ID">{fmtText(log.group_id)}</DetailField>
        <DetailField label="会话 ID">{fmtText(log.session_id)}</DetailField>
        <DetailField label="OpenAI WS 模式">{fmtBool(log.openai_ws_mode)}</DetailField>
        <DetailField label="原生压缩 V2">{fmtBool(log.native_compaction_v2)}</DetailField>
      </DetailSection>

      <DetailSection title="计费">
        <DetailField label="实付金额">{fmtNum(log.amount, 6)}</DetailField>
        <DetailField label="币种">{fmtText(log.currency)}</DetailField>
        <DetailField label="计费倍率">{fmtMultiplierValue(log.rate_multiplier)}</DetailField>
        <DetailField label="输入费用">{fmtNum(log.input_cost, 6)}</DetailField>
        <DetailField label="输出费用">{fmtNum(log.output_cost, 6)}</DetailField>
        <DetailField label="缓存写入费用">{fmtNum(log.cache_creation_cost, 6)}</DetailField>
        <DetailField label="缓存读取费用">{fmtNum(log.cache_read_cost, 6)}</DetailField>
        <DetailField label="原价">{fmtNum(log.total_cost, 6)}</DetailField>
        <DetailField label="计费类型">
          {log.billing_type === 0 ? '0（未知）' : fmtNum(log.billing_type, 0)}
        </DetailField>
        <DetailField label="长上下文计费">{fmtBool(log.long_context_billing_applied)}</DetailField>
        <DetailField label="缓存 TTL 覆盖">{fmtBool(log.cache_ttl_overridden)}</DetailField>
      </DetailSection>

      <DetailSection title="缓存与延迟">
        <DetailField label="缓存读取 Token">{fmtNum(log.cache_read_tokens)}</DetailField>
        <DetailField label="缓存写入 Token">{fmtNum(log.cache_creation_tokens)}</DetailField>
        <DetailField label="缓存写入 5m">{fmtNum(log.cache_creation_5m_tokens)}</DetailField>
        <DetailField label="缓存写入 1h">{fmtNum(log.cache_creation_1h_tokens)}</DetailField>
        <DetailField label="首字延迟">{formatDuration(log.first_token_ms)}</DetailField>
        <DetailField label="总耗时">{formatDuration(log.duration_ms)}</DetailField>
      </DetailSection>

      <DetailSection title="图片">
        <DetailField label="图片数量">{fmtNum(log.image_count)}</DetailField>
        <DetailField label="图片尺寸">{fmtText(log.image_size)}</DetailField>
        <DetailField label="输入尺寸">{fmtText(log.image_input_size)}</DetailField>
        <DetailField label="输出尺寸">{fmtText(log.image_output_size)}</DetailField>
        <DetailField label="输入图片 Token">{fmtNum(log.image_input_tokens)}</DetailField>
        <DetailField label="输入图片费用">{fmtNum(log.image_input_cost, 6)}</DetailField>
        <DetailField label="输出图片 Token">{fmtNum(log.image_output_tokens)}</DetailField>
        <DetailField label="输出图片费用">{fmtNum(log.image_output_cost, 6)}</DetailField>
        <DetailField label="尺寸来源">{fmtText(log.image_size_source)}</DetailField>
        <DetailField label="媒体类型">{fmtText(log.media_type)}</DetailField>
        <DetailField label="尺寸明细">{sizeBreakdown || '—'}</DetailField>
      </DetailSection>

      <DetailSection title="客户端">
        <DetailField label="User-Agent">{fmtText(log.user_agent)}</DetailField>
        <DetailField label="IP 地址">{fmtText(log.ip_address)}</DetailField>
        <DetailField label="状态">
          {isEmptyValue(log.status) ? (
            '—'
          ) : (
            <span className={`badge ${statusBadgeCls(log.status)}`}>{log.status}</span>
          )}
        </DetailField>
        <DetailField label="错误码">{fmtText(log.err_code)}</DetailField>
        <DetailField label="时间">{formatDateTimeFull(log.ts)}</DetailField>
      </DetailSection>
    </div>
  )
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

  // 行详情面板：直接保存被点击的行对象（不依赖行 key，避免 id 缺失时所有行联动展开）
  const [detailLog, setDetailLog] = useState<UsageLog | null>(null)
  const closeDetail = useCallback(() => setDetailLog(null), [])

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
  const todayData = useMemo(() => {
    const t = today()
    return statsDaily.filter((d) => d.day.slice(0, 10) === t)
  }, [statsDaily])

  const stats = useMemo(() => {
    const totalRequests = statsDaily.reduce((sum, d) => sum + d.request_count, 0)
    const totalTokens = statsDaily.reduce((sum, d) => sum + d.total_tokens, 0)
    const totalPrompt = statsDaily.reduce((sum, d) => sum + d.prompt_tokens, 0)
    const totalCompletion = statsDaily.reduce((sum, d) => sum + d.completion_tokens, 0)
    const totalCacheRead = statsDaily.reduce((sum, d) => sum + (d.cache_read_tokens || 0), 0)
    const totalCacheCreation = statsDaily.reduce((sum, d) => sum + (d.cache_creation_tokens || 0), 0)
    const totalAmount = statsDaily.reduce((sum, d) => sum + d.amount, 0)
    const todayRequests = todayData.reduce((sum, d) => sum + d.request_count, 0)
    const todayTokens = todayData.reduce((sum, d) => sum + d.total_tokens, 0)
    const todayCacheRead = todayData.reduce((sum, d) => sum + (d.cache_read_tokens || 0), 0)
    const todayAmount = todayData.reduce((sum, d) => sum + d.amount, 0)
    // 缓存命中率：命中读取 / (输入 + 命中读取)，与 sub2api Dashboard 口径一致
    const cacheHitBase = totalPrompt + totalCacheRead
    const cacheHitRate = cacheHitBase > 0 ? totalCacheRead / cacheHitBase : 0
    return {
      totalRequests, totalTokens, totalPrompt, totalCompletion, totalAmount,
      todayRequests, todayTokens, todayAmount,
      totalCacheRead, totalCacheCreation, todayCacheRead, cacheHitRate,
    }
  }, [statsDaily, todayData])

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
          cache_read_tokens: (existing.cache_read_tokens || 0) + (d.cache_read_tokens || 0),
          cache_creation_tokens: (existing.cache_creation_tokens || 0) + (d.cache_creation_tokens || 0),
          amount: existing.amount + d.amount,
          request_count: existing.request_count + d.request_count,
        })
      } else {
        byModel.set(key, { ...d })
      }
    }
    return Array.from(byModel.values()).sort((a, b) => b.total_tokens - a.total_tokens)
  }, [statsDaily])

  // 单元格导出文本：复合列展开为可读文本（借鉴 sub2api UsageView exportToCSV 列集）
  const csvCell = (l: UsageLog, key: ColumnKey): string => {
    switch (key) {
      case 'ts':
        return formatDateTimeFull(l.ts)
      case 'tokens':
        return `in=${l.prompt_tokens} out=${l.completion_tokens} cache_read=${l.cache_read_tokens} cache_creation=${l.cache_creation_tokens} total=${l.total_tokens}`
      case 'cost':
        return `${l.amount} ${l.currency} (rate=${formatMultiplier(l.rate_multiplier)}, orig=${l.total_cost})`
      case 'latency':
        return `first=${formatDuration(l.first_token_ms)} total=${formatDuration(l.duration_ms)}`
      case 'request_type':
        return requestTypeLabel(l.request_type, l.stream)
      default: {
        const v = l[key] as string | number | null | undefined
        return v === undefined || v === null || v === '' ? '' : String(v)
      }
    }
  }

  // 导出 CSV
  const exportCSV = () => {
    if (logs.length === 0) {
      toast.warning('暂无数据可导出')
      return
    }
    const headers = visibleColumns.map((c) => c.label)
    const rows = logs.map((l) => visibleColumns.map((c) => csvCell(l, c.key)))
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
          站点用量日志（Token 明细含缓存读/写，延迟含首字与总耗时；currency 仅标明不折算）；
          点击任意行的任意字段可弹出该行完整详情面板
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
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-4">
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
        {/* 缓存卡片：命中率 + 读取/写入 token（借鉴 sub2api 缓存健康度口径） */}
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-info">🗄</div>
          <div className="min-w-0">
            <div className="stat-value">{(stats.cacheHitRate * 100).toFixed(1)}%</div>
            <div className="stat-label">
              缓存命中率 · 读 {formatCacheTokens(stats.totalCacheRead)} / 写{' '}
              {formatCacheTokens(stats.totalCacheCreation)}
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
                  <th className="whitespace-nowrap text-right">输入</th>
                  <th className="whitespace-nowrap text-right">输出</th>
                  <th className="whitespace-nowrap text-right">缓存读</th>
                  <th className="whitespace-nowrap text-right">缓存写</th>
                  <th className="whitespace-nowrap text-right">总 Token</th>
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
                      <td className="text-right font-mono text-sm tabular-nums text-sky-600 dark:text-sky-400">
                        {(d.cache_read_tokens || 0).toLocaleString()}
                      </td>
                      <td className="text-right font-mono text-sm tabular-nums text-amber-600 dark:text-amber-400">
                        {(d.cache_creation_tokens || 0).toLocaleString()}
                      </td>
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
                  <th className="whitespace-nowrap text-right">输入</th>
                  <th className="whitespace-nowrap text-right">输出</th>
                  <th className="whitespace-nowrap text-right">缓存读</th>
                  <th className="whitespace-nowrap text-right">缓存写</th>
                  <th className="whitespace-nowrap text-right">总 Token</th>
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
                    <td className="text-right font-mono text-sm tabular-nums text-sky-600 dark:text-sky-400">
                      {(m.cache_read_tokens || 0).toLocaleString()}
                    </td>
                    <td className="text-right font-mono text-sm tabular-nums text-amber-600 dark:text-amber-400">
                      {(m.cache_creation_tokens || 0).toLocaleString()}
                    </td>
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
              {logs.map((l, rowIdx) => (
                <Fragment key={l.id || `row-${rowIdx}`}>
                  <tr
                    className="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/40"
                    title="点击查看该行详情"
                    onClick={() => setDetailLog(l)}
                  >
                  {visibleColumns.map((col, colIdx) => {
                    const ft = l.first_token_ms ?? null
                    const dur = l.duration_ms ?? null
                    let content: React.ReactNode
                    switch (col.key) {
                      case 'ts':
                        content = (
                          <div className="flex flex-col gap-0.5">
                            <span
                              className="whitespace-nowrap text-xs tabular-nums text-gray-700 dark:text-gray-300"
                              title={l.ts}
                            >
                              {formatDateTimeFull(l.ts)}
                            </span>
                            <span className="text-[11px] text-gray-400 dark:text-gray-500">
                              {timeDisplay(l.ts)}
                            </span>
                          </div>
                        )
                        break
                      case 'model_name':
                        content = (
                          <div className="flex flex-col gap-0.5">
                            <span
                              className="font-mono text-xs"
                              title={l.model_name || undefined}
                            >
                              {l.model_name || '—'}
                            </span>
                            {(l.reasoning_effort || l.service_tier) && (
                              <div className="flex flex-wrap items-center gap-1">
                                {l.reasoning_effort && (
                                  <span
                                    className="badge badge-muted px-1 py-px text-[10px]"
                                    title="推理强度"
                                  >
                                    {l.reasoning_effort}
                                  </span>
                                )}
                                {l.service_tier && (
                                  <span
                                    className="badge badge-primary px-1 py-px text-[10px]"
                                    title="服务层级"
                                  >
                                    {l.service_tier}
                                  </span>
                                )}
                              </div>
                            )}
                          </div>
                        )
                        break
                      case 'api_key_mask':
                        content = (
                          <div className="flex items-center gap-1">
                            <span
                              className="font-mono text-xs"
                              title={l.api_key_mask ? `${l.api_key_mask}（双击复制）` : undefined}
                              onClick={(e) => e.stopPropagation()}
                              onDoubleClick={() => l.api_key_mask && copyKey(l.api_key_mask)}
                            >
                              {l.api_key_mask || '—'}
                            </span>
                            {l.api_key_mask && (
                              <button
                                className="text-xs text-gray-400 hover:text-gray-600"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  copyKey(l.api_key_mask)
                                }}
                                title="复制"
                              >
                                📋
                              </button>
                            )}
                          </div>
                        )
                        break
                      case 'tokens':
                        content = (
                          <div className="flex flex-col gap-0.5">
                            <div className="flex items-center gap-2 text-xs tabular-nums">
                              <span
                                className="text-emerald-600 dark:text-emerald-400"
                                title={`输入 ${l.prompt_tokens.toLocaleString()}`}
                              >
                                ↓ {formatTokens(l.prompt_tokens)}
                              </span>
                              <span
                                className="text-violet-600 dark:text-violet-400"
                                title={`输出 ${l.completion_tokens.toLocaleString()}`}
                              >
                                ↑ {formatTokens(l.completion_tokens)}
                              </span>
                            </div>
                            {(l.cache_read_tokens > 0 || l.cache_creation_tokens > 0) && (
                              <div className="flex items-center gap-2 text-[11px] tabular-nums">
                                {l.cache_read_tokens > 0 && (
                                  <span
                                    className="text-sky-600 dark:text-sky-400"
                                    title={`缓存读取 ${l.cache_read_tokens.toLocaleString()}`}
                                  >
                                    🗄 {formatCacheTokens(l.cache_read_tokens)}
                                  </span>
                                )}
                                {l.cache_creation_tokens > 0 && (
                                  <span
                                    className="inline-flex items-center gap-1 text-amber-600 dark:text-amber-400"
                                    title={`缓存写入 ${l.cache_creation_tokens.toLocaleString()}（5m ${(l.cache_creation_5m_tokens || 0).toLocaleString()} / 1h ${(l.cache_creation_1h_tokens || 0).toLocaleString()}）`}
                                  >
                                    ✎ {formatCacheTokens(l.cache_creation_tokens)}
                                    {l.cache_creation_1h_tokens > 0 && (
                                      <span className="rounded bg-orange-100 px-1 py-px text-[10px] font-medium leading-tight text-orange-600 ring-1 ring-inset ring-orange-200 dark:bg-orange-500/20 dark:text-orange-400 dark:ring-orange-500/30">
                                        1h
                                      </span>
                                    )}
                                  </span>
                                )}
                              </div>
                            )}
                            <span className="text-[11px] tabular-nums text-gray-400 dark:text-gray-500">
                              合计 {formatTokens(l.total_tokens)}
                            </span>
                          </div>
                        )
                        break
                      case 'cost':
                        content = (
                          <div className="flex flex-col items-start gap-0.5">
                            <span
                              className="whitespace-nowrap font-mono text-xs tabular-nums"
                              title="实付金额"
                            >
                              {l.amount.toFixed(6)}
                              <span className="ml-1 text-[10px] text-gray-400">
                                {l.currency}
                              </span>
                            </span>
                            {l.total_cost > l.amount + 1e-9 && (
                              <span
                                className="text-[11px] tabular-nums text-gray-400 line-through"
                                title="原价（未计倍率）"
                              >
                                {l.total_cost.toFixed(6)}
                              </span>
                            )}
                            {l.rate_multiplier > 0 &&
                              Math.abs(l.rate_multiplier - 1) > 1e-9 && (
                                <span
                                  className="badge badge-warning px-1 py-px text-[10px]"
                                  title="计费倍率"
                                >
                                  ×{formatMultiplier(l.rate_multiplier)}
                                </span>
                              )}
                          </div>
                        )
                        break
                      case 'latency':
                        content = (
                          <div className="flex items-stretch gap-2">
                            <span
                              className={`w-1 shrink-0 rounded-full ${
                                ft !== null
                                  ? `bg-gradient-to-b ${LATENCY_BAR_FROM_CLS[firstTokenSeverity(ft)]} ${LATENCY_BAR_TO_CLS[durationSeverity(dur ?? 0)]}`
                                  : LATENCY_BAR_CLS[durationSeverity(dur ?? 0)]
                              }`}
                              aria-hidden="true"
                            />
                            <div className="grid grid-cols-[max-content_max-content] items-baseline gap-x-2 gap-y-0.5 text-[11px]">
                              <span className="text-gray-400 dark:text-gray-500">首字</span>
                              {ft !== null ? (
                                <span
                                  className={`font-medium tabular-nums ${LATENCY_TEXT_CLS[firstTokenSeverity(ft)]}`}
                                >
                                  {formatDuration(ft)}
                                </span>
                              ) : (
                                <span className="text-gray-400 dark:text-gray-500">—</span>
                              )}
                              <span className="text-gray-400 dark:text-gray-500">耗时</span>
                              {dur !== null ? (
                                <span
                                  className={`font-medium tabular-nums ${LATENCY_TEXT_CLS[durationSeverity(dur)]}`}
                                >
                                  {formatDuration(dur)}
                                </span>
                              ) : (
                                <span className="text-gray-400 dark:text-gray-500">—</span>
                              )}
                            </div>
                          </div>
                        )
                        break
                      case 'request_type':
                        content = (
                          <div className="flex flex-col items-start gap-0.5">
                            <span
                              className={`badge ${requestTypeBadgeCls(l.request_type, l.stream)}`}
                            >
                              {requestTypeLabel(l.request_type, l.stream)}
                            </span>
                            {l.billing_mode && (
                              <span className="text-[10px] text-gray-400 dark:text-gray-500">
                                {l.billing_mode}
                              </span>
                            )}
                          </div>
                        )
                        break
                      case 'status':
                        content = (
                          <span className={`badge ${statusBadgeCls(l.status)}`}>
                            {l.status || '—'}
                          </span>
                        )
                        break
                      case 'err_code':
                        content = (
                          <span
                            className="font-mono text-xs text-gray-500"
                            title={l.err_code || undefined}
                          >
                            {l.err_code || '—'}
                          </span>
                        )
                        break
                      case 'inbound_endpoint':
                        content = (
                          <span
                            className="block max-w-[180px] truncate font-mono text-[11px] text-gray-500"
                            title={l.inbound_endpoint || undefined}
                          >
                            {l.inbound_endpoint || '—'}
                          </span>
                        )
                        break
                      default:
                        content = '—'
                    }
                    return (
                      <td key={col.key} className="align-top">
                        {colIdx === 0 ? (
                          <div className="flex items-start gap-1.5">
                            <button
                              type="button"
                              className="mt-0.5 shrink-0 text-[10px] leading-none text-gray-400 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-gray-200"
                              aria-haspopup="dialog"
                              aria-label="查看该行详情"
                              title="查看该行详情"
                              onClick={(e) => {
                                e.stopPropagation()
                                setDetailLog(l)
                              }}
                            >
                              ▸
                            </button>
                            <div className="min-w-0 flex-1">{content}</div>
                          </div>
                        ) : (
                          content
                        )}
                      </td>
                    )
                  })}
                </tr>
                </Fragment>
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

      {/* 行详情面板：点击某行的任意字段弹出（替代原行内展开，避免 id 缺失时所有行联动） */}
      <Modal
        open={detailLog !== null}
        onClose={closeDetail}
        width="max-w-5xl"
        title={
          <span className="flex flex-wrap items-center gap-2">
            <span>调用详情</span>
            {detailLog && (
              <>
                <span className="font-mono text-xs font-normal text-gray-500 dark:text-dark-300">
                  {fmtText(detailLog.model_name)}
                </span>
                <span className="text-xs font-normal tabular-nums text-gray-400 dark:text-dark-400">
                  {formatDateTimeFull(detailLog.ts)}
                </span>
              </>
            )}
          </span>
        }
        footer={
          <button type="button" className="btn btn-secondary" onClick={closeDetail}>
            关闭
          </button>
        }
      >
        {detailLog && (
          <div className="max-h-[70vh] space-y-3 overflow-y-auto pr-1">
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className={`badge ${statusBadgeCls(detailLog.status || '')}`}>
                {fmtText(detailLog.status)}
              </span>
              <span className="badge badge-muted">
                Token {formatTokens(detailLog.total_tokens ?? 0)}
              </span>
              <span className="badge badge-muted">
                首字 {formatDuration(detailLog.first_token_ms)} · 耗时{' '}
                {formatDuration(detailLog.duration_ms)}
              </span>
              <span className="badge badge-muted">
                实付 {fmtNum(detailLog.amount, 6)} {fmtText(detailLog.currency)}
              </span>
              <span className="badge badge-muted">Key {fmtText(detailLog.api_key_mask)}</span>
              <span className="text-[11px] text-gray-400 dark:text-dark-400">
                Esc / 点击遮罩关闭
              </span>
            </div>
            <UsageLogDetail log={detailLog} />
          </div>
        )}
      </Modal>
    </div>
  )
}
