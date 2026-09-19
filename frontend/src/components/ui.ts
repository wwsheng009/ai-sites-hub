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
