/** 状态徽章：根据值映射语义色（统一各页面的状态 → 颜色逻辑） */
export function statusBadgeCls(v: string): string {
  const s = v.toLowerCase()
  if (['ok', 'active', 'success', 'enabled', 'available'].includes(s)) return 'badge-success'
  if (['error', 'failed', 'fail', 'disabled'].includes(s)) return 'badge-danger'
  if (['warn', 'warning', 'pending', 'detecting', 'expired'].includes(s)) return 'badge-warning'
  return 'badge-muted'
}

/** 站点类型徽章 */
export function siteTypeBadgeCls(t: string): string {
  return t === 'unknown' ? 'badge-muted' : 'badge-primary'
}

/** 事件级别徽章 */
export function eventLevelBadgeCls(level: string): string {
  const s = level.toLowerCase()
  if (s === 'error' || s === 'fail') return 'badge-danger'
  if (s === 'warn') return 'badge-warning'
  if (s === 'debug') return 'badge-muted'
  return 'badge-primary'
}

/** Token 数字紧凑格式化（借鉴 sub2api formatTokens） */
export function formatTokens(n: number): string {
  if (n >= 1e9) return (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toLocaleString()
}

/** 缓存 token 紧凑格式化（借鉴 sub2api formatCacheTokens，保留 1 位小数） */
export function formatCacheTokens(tokens: number): string {
  if (tokens >= 1_000_000) return (tokens / 1_000_000).toFixed(1) + 'M'
  if (tokens >= 1_000) return (tokens / 1_000).toFixed(1) + 'K'
  return tokens.toLocaleString()
}

/**
 * 延迟毫秒格式化（借鉴 sub2api formatDuration）：
 * <1s 显示 ms，<60s 显示 s（1 位小数），否则显示分秒。
 */
export function formatDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || Number.isNaN(ms)) return '—'
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60_000)
  const s = Math.round((ms % 60_000) / 1000)
  return `${m}m${String(s).padStart(2, '0')}s`
}

/** 自适应精度倍率（借鉴 sub2api formatMultiplier）：至少 2 位小数，最多 4 位并去掉尾随 0 */
export function formatMultiplier(val: number): string {
  if (!val) return '1.00'
  if (val < 0.0001) return val.toPrecision(2)
  return val.toFixed(4).replace(/(\.\d{2}\d*?)0+$/, '$1')
}

/** 绝对时间（本地时区，秒级）；解析失败时原样返回 */
export function formatDateTimeFull(s: string | null | undefined): string {
  if (!s) return '—'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 延迟健康度分档（借鉴 sub2api latencyHealth；阈值对齐其线上口径） */
export type LatencySeverity = 'good' | 'warn' | 'slow' | 'critical'

const FIRST_TOKEN_THRESHOLDS_MS = { warn: 10_000, slow: 30_000, critical: 60_000 } as const
const DURATION_THRESHOLDS_MS = { warn: 60_000, slow: 180_000, critical: 300_000 } as const

function classifyLatency(ms: number, t: { warn: number; slow: number; critical: number }): LatencySeverity {
  if (ms >= t.critical) return 'critical'
  if (ms >= t.slow) return 'slow'
  if (ms >= t.warn) return 'warn'
  return 'good'
}

export function firstTokenSeverity(ms: number): LatencySeverity {
  return classifyLatency(ms, FIRST_TOKEN_THRESHOLDS_MS)
}

export function durationSeverity(ms: number): LatencySeverity {
  return classifyLatency(ms, DURATION_THRESHOLDS_MS)
}

/** 延迟数值文本色（good → critical） */
export const LATENCY_TEXT_CLS: Record<LatencySeverity, string> = {
  good: 'text-emerald-600 dark:text-emerald-400',
  warn: 'text-amber-600 dark:text-amber-400',
  slow: 'text-orange-600 dark:text-orange-400',
  critical: 'text-red-600 dark:text-red-400',
}

/** 延迟健康度色条（无首字数据时纯色） */
export const LATENCY_BAR_CLS: Record<LatencySeverity, string> = {
  good: 'bg-emerald-500',
  warn: 'bg-amber-400',
  slow: 'bg-orange-500',
  critical: 'bg-red-500',
}

/** 顶部渐变色（首字档） */
export const LATENCY_BAR_FROM_CLS: Record<LatencySeverity, string> = {
  good: 'from-emerald-500',
  warn: 'from-amber-400',
  slow: 'from-orange-500',
  critical: 'from-red-500',
}

/** 底部渐变色（总耗时档） */
export const LATENCY_BAR_TO_CLS: Record<LatencySeverity, string> = {
  good: 'to-emerald-500',
  warn: 'to-amber-400',
  slow: 'to-orange-500',
  critical: 'to-red-500',
}

/** 请求形态标签（借鉴 sub2api resolveUsageRequestType + getRequestTypeExportText） */
export function requestTypeLabel(t: string, stream: boolean): string {
  const s = (t || '').toLowerCase()
  if (s === 'cyber') return 'Cyber'
  if (s === 'live') return 'Live'
  if (s === 'ws_v2' || s === 'ws') return 'WS'
  if (s === 'stream') return 'Stream'
  if (s === 'sync') return 'Sync'
  return stream ? 'Stream' : 'Sync'
}

/** 请求形态徽章色 */
export function requestTypeBadgeCls(t: string, stream: boolean): string {
  const label = requestTypeLabel(t, stream)
  if (label === 'WS') return 'badge-primary'
  if (label === 'Stream') return 'badge-success'
  if (label === 'Live') return 'badge-warning'
  if (label === 'Cyber') return 'badge-danger'
  return 'badge-muted'
}

/** 相对时间显示（如 "3 分钟前"），超出一律显示绝对日期 */
export function timeDisplay(s: string | null | undefined): string {
  if (!s) return '—'
  const t = new Date(s).getTime()
  if (Number.isNaN(t)) return s
  const diff = Date.now() - t
  if (diff < 0) return s
  const min = Math.floor(diff / 60_000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h} 小时前`
  const d = new Date(s)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
