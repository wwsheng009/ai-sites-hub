/**
 * 站点状态监控页
 * 借鉴 sub2api ChannelStatusView 的模式：统一展示所有站点的健康状态、
 * 同步状态、认证状态与余额。多站点聚合，单站点不放大。
 */
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiDoctor, apiListAffiliates, apiListSites, apiListSyncStates } from '../api/endpoints'
import type { DoctorReport, Site, SiteAffiliateOut, SyncState } from '../types'
import { useToast, errMsg } from '../components/Toast'
import { timeDisplay } from '../components/ui'

const statusBadgeCls: Record<string, string> = {
  ok: 'badge-success',
  active: 'badge-success',
  warn: 'badge-warning',
  fail: 'badge-danger',
  error: 'badge-danger',
  offline: 'badge-danger',
}

const syncStateBadgeCls: Record<string, string> = {
  ok: 'badge-success',
  success: 'badge-success',
  error: 'badge-danger',
  warn: 'badge-warning',
}

const domainLabels: Record<string, string> = {
  keys: '密钥',
  groups: '分组',
  account: '账号余额',
  affiliate: '返利',
  usage_log: '用量日志',
  usage_daily: '日聚合',
  announcements: '公告',
  checkin: '签到',
}

export default function SiteStatus() {
  const toast = useToast()
  const [sites, setSites] = useState<Site[]>([])
  const [affs, setAffs] = useState<SiteAffiliateOut[]>([])
  const [doctor, setDoctor] = useState<DoctorReport | null>(null)
  const [syncStates, setSyncStates] = useState<Record<string, SyncState[]>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    Promise.all([apiListSites(), apiListAffiliates(), apiDoctor()])
      .then(([s, a, d]) => {
        if (cancelled) return
        setSites(s)
        setAffs(a)
        setDoctor(d)

        // 并行加载每个站点的同步状态
        return Promise.all(
          s.map((st) =>
            apiListSyncStates(st.id).catch(() => []),
          ),
        )
      })
      .then((results) => {
        if (cancelled) return
        if (results) {
          const byState: Record<string, SyncState[]> = {}
          sites.forEach((st, i) => {
            byState[st.id] = results[i] || []
          })
          setSyncStates(byState)
        }
        setLoading(false)
      })
      .catch((e) => {
        if (cancelled) return
        const m = errMsg(e)
        setError(m)
        toast.error(m)
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [toast])

  // 获取站点认证状态
  const getAuthState = (siteId: string) => {
    const states = syncStates[siteId] || []
    const accountState = states.find((s) => s.domain === 'account')
    if (!accountState) return 'unknown'
    if (accountState.consecutive_failures > 0) return 'error'
    if (accountState.last_error_class) return 'warn'
    return 'ok'
  }

  // 获取站点余额
  const getBalance = (siteId: string) => {
    return affs.find((a) => a.site_id === siteId)
  }

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">站点状态</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-dark-400">
          跨站健康监控 · 借鉴自 sub2api ChannelStatusView
        </p>
      </div>

      {error && <p className="text-danger">{error}</p>}

      {/* 系统体检摘要 */}
      {doctor && (
        <div className="card">
          <div className="card-header">
            <h3 className="font-semibold text-gray-900 dark:text-white">系统体检</h3>
          </div>
          <div className="card-body">
            <div className="flex items-center gap-3">
              <span className={`badge ${statusBadgeCls[doctor.overall] ?? 'badge-muted'}`}>{doctor.overall}</span>
              <span className="text-xs text-gray-500 dark:text-dark-400">
                检查时间：{timeDisplay(doctor.checked_at)}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* 站点状态表格 */}
      {loading ? (
        <div className="flex items-center gap-3 py-12">
          <span className="spinner" />
          <span className="text-sm text-muted">加载中…</span>
        </div>
      ) : sites.length === 0 ? (
        <div className="card">
          <div className="card-body empty-state">
            <span className="text-3xl">🌐</span>
            <span className="empty-state-title">暂无站点</span>
            <span className="empty-state-desc">前往「站点管理」添加站点</span>
          </div>
        </div>
      ) : (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                <th>站点</th>
                <th>类型</th>
                <th>状态</th>
                <th>认证</th>
                <th>余额</th>
                <th>同步域</th>
                <th>上次同步</th>
                <th>最近错误</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {sites.map((s) => {
                const balance = getBalance(s.id)
                const authState = getAuthState(s.id)
                const states = syncStates[s.id] || []
                const lastSync = states.reduce((latest, st) => {
                  if (!st.last_run_at) return latest
                  return !latest || st.last_run_at > latest ? st.last_run_at : latest
                }, '' as string | null)
                const lastError = states.find((st) => st.last_error_class)
                const failedCount = states.filter((st) => st.consecutive_failures > 0).length

                return (
                  <tr key={s.id}>
                    <td className="font-medium">
                      <Link
                        to={`/sites/${s.id}`}
                        className="text-primary-600 hover:text-primary-700 dark:text-primary-400"
                      >
                        {s.name}
                      </Link>
                    </td>
                    <td>
                      <span className={`badge ${s.site_type === 'unknown' ? 'badge-muted' : 'badge-primary'} px-2 py-0.5 text-xs`}>
                        {s.site_type}
                      </span>
                    </td>
                    <td>
                      <span className={`badge ${statusBadgeCls[s.status] ?? 'badge-muted'}`}>{s.status}</span>
                    </td>
                    <td>
                      <span
                        className={`badge ${
                          authState === 'ok'
                            ? 'badge-success'
                            : authState === 'warn'
                            ? 'badge-warning'
                            : authState === 'error'
                            ? 'badge-danger'
                            : 'badge-muted'
                        }`}
                      >
                        {authState === 'ok' ? '正常' : authState === 'warn' ? '警告' : authState === 'error' ? '失败' : '未知'}
                      </span>
                    </td>
                    <td className="font-mono text-xs">
                      {balance ? (
                        <>
                          <span className="text-gray-900 dark:text-white">{balance.available ?? '—'}</span>
                          <span className="text-gray-500 dark:text-dark-400"> {balance.currency ?? ''}</span>
                        </>
                      ) : (
                        '—'
                      )}
                    </td>
                    <td>
                      <div className="flex flex-wrap gap-1">
                        {states.map((st) => (
                          <span
                            key={st.id}
                            className={`badge badge-sm ${
                              st.consecutive_failures > 0 ? 'badge-danger' : st.last_error_class ? 'badge-warning' : 'badge-success'
                            }`}
                            title={domainLabels[st.domain] || st.domain}
                          >
                            {domainLabels[st.domain] || st.domain}
                          </span>
                        ))}
                        {states.length === 0 && <span className="text-xs text-gray-400">无</span>}
                      </div>
                    </td>
                    <td className="text-muted whitespace-nowrap text-sm">
                      {lastSync ? timeDisplay(lastSync) : '—'}
                    </td>
                    <td className="max-w-[150px] truncate text-xs text-muted" title={lastError?.last_error_class || ''}>
                      {lastError?.last_error_class || '—'}
                      {failedCount > 0 && (
                        <span className="ml-1 badge badge-danger">{failedCount} 失败</span>
                      )}
                    </td>
                    <td>
                      <Link
                        to={`/sites/${s.id}/jobs`}
                        className="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400"
                      >
                        作业详情
                      </Link>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="text-right">
        <Link to="/" className="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400">
          ← 返回概览
        </Link>
      </div>
    </div>
  )
}
