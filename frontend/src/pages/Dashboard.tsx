import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { apiDoctor, apiListAffiliates, apiListSites, apiListSyncStates, apiListUsageDaily } from '../api/endpoints'
import type { DoctorReport, Site, SiteAffiliateOut, SyncState, UsageDaily } from '../types'
import { useToast, errMsg } from '../components/Toast'
import { timeDisplay } from '../components/ui'

const checkBadge: Record<string, string> = {
  ok: 'badge-success',
  warn: 'badge-warning',
  fail: 'badge-danger',
}

export default function Dashboard() {
  const toast = useToast()
  const navigate = useNavigate()
  const [sites, setSites] = useState<Site[]>([])
  const [affs, setAffs] = useState<SiteAffiliateOut[]>([])
  const [doctor, setDoctor] = useState<DoctorReport | null>(null)
  const [daily, setDaily] = useState<Record<string, UsageDaily[]>>({})
  const [syncStates, setSyncStates] = useState<Record<string, SyncState[]>>({})
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    Promise.all([apiListSites(), apiListAffiliates(), apiDoctor()])
      .then(([s, a, d]) => {
        if (cancelled) return
        setSites(s)
        setAffs(a)
        setDoctor(d)

        // S3：近 7 天用量趋势（跨站，currency 不合并）
        const activeSites = s.filter((st) => st.status === 'active' || st.status === 'ok')
        return Promise.all(
          activeSites.map((st) =>
            apiListUsageDaily(st.id, { start: sevenDaysAgo(), end: today(), limit: 30 }).catch(() => []),
          ),
        ).then((results) => {
          const bySite: Record<string, UsageDaily[]> = {}
          activeSites.forEach((st, i) => {
            bySite[st.id] = results[i] || []
          })
          setDaily(bySite)

          // S1：同步作业状态（用于余额卡片 freshness）
          return Promise.all(
            activeSites.map((st) => apiListSyncStates(st.id).catch(() => [])),
          )
        }).then((syncResults) => {
          if (!syncResults) return
          const byState: Record<string, SyncState[]> = {}
          activeSites.forEach((st, i) => {
            byState[st.id] = syncResults[i] || []
          })
          setSyncStates(byState)
        })
      })
      .catch((e) => {
        const m = errMsg(e)
        setError(m)
        toast.error(m)
      })
    return () => {
      cancelled = true
    }
  }, [toast])

  const byType = sites.reduce<Record<string, number>>((m, s) => {
    m[s.site_type] = (m[s.site_type] ?? 0) + 1
    return m
  }, {})

  const totalAvailable = affs.reduce((sum, a) => sum + (a.available ?? 0), 0)
  const okCount = sites.filter((s) => s.status === 'ok' || s.status === 'active').length

  // S3：余额卡片 freshness — 检查所有站点 account 域是否 stale（>2min）
  const staleSites = sites.filter((s) => {
    const states = syncStates[s.id] || []
    const accountState = states.find((st) => st.domain === 'account')
    if (!accountState || !accountState.last_run_at) return false
    return Date.now() - new Date(accountState.last_run_at).getTime() > 120000
  })
  const lastSyncAt = (() => {
    const all = Object.values(syncStates).flat()
    if (all.length === 0) return null
    const times = all.map((s) => s.last_run_at).filter(Boolean)
    if (times.length === 0) return null
    return new Date(Math.max(...times.map((t) => new Date(t!).getTime())))
  })()

  // S3 日期辅助
  const today = () => new Date().toISOString().slice(0, 10)
  const sevenDaysAgo = () => {
    const d = new Date()
    d.setDate(d.getDate() - 7)
    return d.toISOString().slice(0, 10)
  }

  // 汇总跨站 daily → 按日期聚合（不同 currency 不合并求和）
  const trendData = (() => {
    const byDate: Record<string, { date: string; sites: Record<string, { amount: number; currency: string }> }> = {}
    Object.entries(daily).forEach(([siteId, days]) => {
      days.forEach((d) => {
        if (!byDate[d.day]) byDate[d.day] = { date: d.day, sites: {} }
        byDate[d.day].sites[siteId] = { amount: d.amount, currency: d.currency }
      })
    })
    return Object.values(byDate).sort((a, b) => (a.date > b.date ? 1 : -1))
  })()

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">概览</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-dark-400">站点、凭据与返利的集中监控面板</p>
      </div>

      {error && <p className="text-error">{error}</p>}

      {/* 统计卡片网格 */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-primary">🌐</div>
          <div className="min-w-0">
            <div className="stat-value">{sites.length}</div>
            <div className="stat-label">站点总数</div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-success">✅</div>
          <div className="min-w-0">
            <div className="stat-value">{okCount}</div>
            <div className="stat-label">正常站点</div>
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-warning">💰</div>
          <div className="min-w-0">
            <div className="stat-value truncate">{totalAvailable}</div>
            <div className="stat-label">返利可用余额合计</div>
            {staleSites.length > 0 && (
              <div className="mt-1 flex items-center gap-1 text-xs text-warning">
                <span className="h-1.5 w-1.5 rounded-full bg-warning" />
                <span>{staleSites.length} 个站点余额过期</span>
              </div>
            )}
            {lastSyncAt && (
              <div className="text-xs text-muted">
                上次同步：{timeDisplay(lastSyncAt.toISOString())}
              </div>
            )}
          </div>
        </div>
        <div className="stat-card card-hover">
          <div className="stat-icon stat-icon-danger">🩺</div>
          <div className="min-w-0">
            <div className="stat-value">
              {doctor ? (
                <span className={`badge ${checkBadge[doctor.overall] ?? 'badge-muted'}`}>{doctor.overall}</span>
              ) : (
                <span className="spinner inline-block" />
              )}
            </div>
            <div className="stat-label">系统体检状态</div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        {/* 返利概览 */}
        <section className="card">
          <div className="card-header flex items-center justify-between">
            <h3 className="font-semibold text-gray-900 dark:text-white">返利概览</h3>
            <Link to="/affiliates" className="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">
              前往返利中心 →
            </Link>
          </div>
          <div className="card-body p-0">
            {affs.length === 0 ? (
              <div className="empty-state">
                <span className="text-3xl">💰</span>
                <span className="empty-state-title">暂无返利数据</span>
                <span className="empty-state-desc">同步站点后自动获取（AffiliateInfo）</span>
              </div>
            ) : (
              <div className="table-container border-0">
                <table className="table">
                  <thead>
                    <tr>
                      <th>站点</th>
                      <th>可用余额</th>
                      <th>累计</th>
                      <th>邀请人数</th>
                      <th>更新时间</th>
                    </tr>
                  </thead>
                  <tbody>
                    {affs.map((a) => (
                      <tr key={a.id}>
                        <td>
                          <Link
                            to={`/sites/${a.site_id}`}
                            className="hover:text-primary-600 dark:hover:text-primary-400"
                          >
                            {a.site_name || sites.find((s) => s.id === a.site_id)?.name || a.site_id}
                          </Link>
                        </td>
                        <td>{a.available ?? '—'}</td>
                        <td>{a.history ?? '—'}</td>
                        <td>{a.invitee_count ?? '—'}</td>
                        <td className="text-muted whitespace-nowrap">{a.last_sync_at ?? '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </section>

        {/* 系统体检 */}
        <section className="card">
          <div className="card-header">
            <h3 className="font-semibold text-gray-900 dark:text-white">系统体检（doctor）</h3>
          </div>
          <div className="card-body">
            {doctor ? (
              <ul className="space-y-3">
                {doctor.checks.map((c) => (
                  <li key={c.name} className="flex items-start gap-3">
                    <span className={`badge ${checkBadge[c.status] ?? 'badge-muted'} mt-0.5`}>{c.status}</span>
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-gray-900 dark:text-white">{c.name}</div>
                      <div className="text-xs text-gray-500 dark:text-dark-400">{c.detail}</div>
                    </div>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="flex items-center gap-3 py-8">
                <span className="spinner" />
                <span className="text-sm text-muted">体检报告加载中…</span>
              </div>
            )}
          </div>
        </section>
      </div>

      {/* S3：近 7 天用量趋势（按站点分组，currency 不合并） */}
      <section className="card">
        <div className="card-header flex items-center justify-between">
          <h3 className="font-semibold text-gray-900 dark:text-white">近 7 天用量趋势</h3>
          <span className="text-xs text-muted">不同站点/币种不合并求和</span>
        </div>
        <div className="card-body">
          {trendData.length === 0 ? (
            <div className="empty-state py-8">
              <span className="text-3xl">📊</span>
              <span className="empty-state-title">暂无用量数据</span>
              <span className="empty-state-desc">同步站点后自动获取</span>
            </div>
          ) : (
            <div className="space-y-4">
              {trendData.map((d) => (
                <div key={d.date} className="flex items-end gap-4">
                  <span className="text-xs text-muted whitespace-nowrap w-20">{d.date}</span>
                  {Object.entries(d.sites).map(([siteId, info]) => {
                    const site = sites.find((s) => s.id === siteId)
                    const maxAmount = Math.max(1, ...trendData.flatMap((t) => Object.values(t.sites).map((v) => v.amount)))
                    const barHeight = Math.max(4, (info.amount / maxAmount) * 60)
                    return (
                      <div
                        key={siteId}
                        className="flex flex-col items-center"
                        title={`${site?.name || siteId}: ${info.amount} ${info.currency}`}
                        onClick={() => navigate(`/sites/${siteId}/usage/logs?start=${d.date}&end=${d.date}`)}
                        style={{ cursor: 'pointer' }}
                      >
                        <div
                          className="w-8 rounded-t bg-primary-500 dark:bg-primary-400 transition-all hover:opacity-80"
                          style={{ height: `${barHeight}px` }}
                        />
                        <span className="mt-1 max-w-[80px] truncate text-xs text-muted" title={site?.name || siteId}>
                          {site?.name || siteId.slice(0, 6)}
                        </span>
                        <span className="text-xs font-mono">{info.amount.toFixed(2)}</span>
                        <span className="text-xs text-muted">{info.currency}</span>
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* 站点类型分布 */}
      <section className="card">
        <div className="card-header">
          <h3 className="font-semibold text-gray-900 dark:text-white">站点类型分布</h3>
        </div>
        <div className="card-body">
          {sites.length === 0 ? (
            <p className="text-sm text-muted">暂无站点，前往「站点管理」添加第一个站点。</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {Object.entries(byType).map(([t, n]) => (
                <span key={t} className={`badge ${t === 'unknown' ? 'badge-muted' : 'badge-primary'} px-3 py-1 text-sm`}>
                  {t} · {n}
                </span>
              ))}
            </div>
          )}
        </div>
      </section>
    </div>
  )
}
