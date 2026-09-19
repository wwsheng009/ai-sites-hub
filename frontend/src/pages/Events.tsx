import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiListEvents, apiListSites } from '../api/endpoints'
import type { Event, Site } from '../types'
import { useToast, errMsg } from '../components/Toast'
import { eventLevelBadgeCls, timeDisplay } from '../components/ui'

const LEVELS = ['error', 'warn', 'info', 'debug'] as const

export default function Events() {
  const toast = useToast()
  const [events, setEvents] = useState<Event[]>([])
  const [sites, setSites] = useState<Site[]>([])
  const [loading, setLoading] = useState(true)
  const [levelFilter, setLevelFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [siteFilter, setSiteFilter] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(false)
  const [refreshing, setRefreshing] = useState(false)

  const load = useCallback(async () => {
    setRefreshing(true)
    try {
      const [evs, sts] = await Promise.all([apiListEvents({ limit: 200 }), apiListSites()])
      setEvents(evs)
      setSites(sts)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [toast])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (!autoRefresh) return
    const t = setInterval(load, 10_000)
    return () => clearInterval(t)
  }, [autoRefresh, load])

  const types = useMemo(() => Array.from(new Set(events.map((e) => e.type))).sort(), [events])
  const siteMap = useMemo(() => new Map(sites.map((s) => [s.id, s])), [sites])

  const filtered = useMemo(
    () =>
      events.filter((e) => {
        if (levelFilter && e.level.toLowerCase() !== levelFilter) return false
        if (typeFilter && e.type !== typeFilter) return false
        if (siteFilter === 'none' && e.site_id !== null) return false
        if (siteFilter && siteFilter !== 'none' && e.site_id !== siteFilter) return false
        return true
      }),
    [events, levelFilter, typeFilter, siteFilter],
  )

  const levelCounts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const e of events) {
      const l = e.level.toLowerCase()
      c[l] = (c[l] ?? 0) + 1
    }
    return c
  }, [events])

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">事件中心</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-dark-400">站点探测、同步与返利相关的运行事件流</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex cursor-pointer items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            10s 自动刷新
          </label>
          <button className="btn btn-secondary" disabled={refreshing} onClick={load}>
            {refreshing ? (
              <>
                <span className="spinner !h-4 !w-4 !border-2" /> 刷新中…
              </>
            ) : (
              '⟳ 刷新'
            )}
          </button>
        </div>
      </div>

      {/* 级别统计卡片 */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {LEVELS.map((lv) => (
          <button
            key={lv}
            onClick={() => setLevelFilter(levelFilter === lv ? '' : lv)}
            className={`stat-card text-left transition-transform hover:-translate-y-0.5 ${
              levelFilter === lv ? 'ring-2 ring-primary-500' : ''
            }`}
          >
            <div className="stat-label uppercase">{lv}</div>
            <div className="stat-value">{levelCounts[lv] ?? 0}</div>
          </button>
        ))}
      </div>

      {/* 过滤器 */}
      <div className="card grid grid-cols-1 gap-3 p-4 sm:grid-cols-3">
        <select className="input" value={levelFilter} onChange={(e) => setLevelFilter(e.target.value)}>
          <option value="">全部级别</option>
          {LEVELS.map((l) => (
            <option key={l} value={l}>
              {l}
            </option>
          ))}
        </select>
        <select className="input" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
          <option value="">全部类型</option>
          {types.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        <select className="input" value={siteFilter} onChange={(e) => setSiteFilter(e.target.value)}>
          <option value="">全部站点</option>
          <option value="none">未关联站点</option>
          {sites.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </div>

      {/* 事件列表 */}
      <section className="card">
        <div className="card-header flex items-center justify-between">
          <h3 className="font-semibold text-gray-900 dark:text-white">事件列表（{filtered.length}）</h3>
          {autoRefresh && (
            <span className="badge badge-success">
              <span className="mr-1 inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-current" />
              自动刷新中
            </span>
          )}
        </div>
        <div className="card-body p-0">
          {loading ? (
            <div className="flex items-center justify-center gap-3 py-16">
              <span className="spinner" />
              <span className="text-sm text-muted">加载中…</span>
            </div>
          ) : (
            <div className="table-container border-0">
              <table className="table">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>级别</th>
                    <th>类型</th>
                    <th>消息</th>
                    <th>站点</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((e) => (
                    <tr key={e.id}>
                      <td className="text-muted whitespace-nowrap text-xs" title={e.created_at}>
                        {timeDisplay(e.created_at)}
                      </td>
                      <td>
                        <span className={`badge ${eventLevelBadgeCls(e.level)}`}>{e.level}</span>
                      </td>
                      <td className="font-mono text-xs">{e.type}</td>
                      <td className="max-w-[380px]">
                        <span className="block truncate text-sm" title={e.message}>
                          {e.message}
                        </span>
                      </td>
                      <td>
                        {e.site_id && siteMap.has(e.site_id) ? (
                          <Link
                            to={`/sites/${e.site_id}`}
                            className="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                          >
                            {siteMap.get(e.site_id)!.name}
                          </Link>
                        ) : (
                          <span className="text-xs text-muted">—</span>
                        )}
                      </td>
                    </tr>
                  ))}
                  {filtered.length === 0 && (
                    <tr>
                      <td colSpan={5}>
                        <div className="empty-state">
                          <span className="text-3xl">📭</span>
                          <span className="empty-state-title">
                            {levelFilter || typeFilter || siteFilter ? '没有匹配的事件' : '暂无事件'}
                          </span>
                          {!levelFilter && !typeFilter && !siteFilter && (
                            <span className="empty-state-desc">添加站点并执行探测/同步后，事件会出现在这里</span>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </section>
    </div>
  )
}
