import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGetSite, apiListSyncStates, apiSyncSite } from '../api/endpoints'
import type { Site, SyncState } from '../types'
import { useToast, errMsg } from '../components/Toast'
import { timeDisplay } from '../components/ui'

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

const stateBadge: Record<string, string> = {
  '': 'badge-muted',
  ok: 'badge-success',
  success: 'badge-success',
  error: 'badge-danger',
  warn: 'badge-warning',
}

export default function Jobs() {
  const { id } = useParams<{ id: string }>()
  const toast = useToast()
  const [site, setSite] = useState<Site | null>(null)
  const [states, setStates] = useState<SyncState[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)

  useEffect(() => {
    if (!id) return
    apiGetSite(id).then(setSite).catch(() => setSite(null))
    apiListSyncStates(id)
      .then((data) => {
        setStates(data)
        setLoading(false)
      })
      .catch((e) => {
        toast.error(errMsg(e))
        setLoading(false)
      })
  }, [id, toast])

  const handleSync = async () => {
    if (!id) return
    setSyncing(true)
    try {
      await apiSyncSite(id)
      toast.success('同步已触发')
      // 重新加载状态
      apiListSyncStates(id).then(setStates)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setSyncing(false)
    }
  }

  if (!id) return null

  return (
    <div className="space-y-6">
      <div>
        <Link to="/sites" className="text-sm text-gray-500 hover:text-primary-600 dark:text-gray-400">
          ← 返回站点列表
        </Link>
        <h1 className="mt-2 text-2xl font-bold text-gray-900 dark:text-white">
          同步作业{site ? ` — ${site.name}` : ''}
        </h1>
        <p className="mt-1 text-sm text-muted">站点下所有域的调度状态与最近同步记录</p>
      </div>

      <div className="flex gap-3">
        <button className="btn btn-primary btn-sm" onClick={handleSync} disabled={syncing}>
          {syncing ? '同步中…' : '立即同步全部'}
        </button>
      </div>

      {loading ? (
        <div className="flex items-center gap-3 py-12">
          <span className="spinner" />
          <span className="text-sm text-muted">加载中…</span>
        </div>
      ) : states.length === 0 ? (
        <div className="card">
          <div className="card-body empty-state">
            <span className="text-3xl">📋</span>
            <span className="empty-state-title">暂无同步作业</span>
            <span className="empty-state-desc">站点尚未初始化同步域</span>
          </div>
        </div>
      ) : (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                <th>域</th>
                <th>状态</th>
                <th>上次运行</th>
                <th>下次运行</th>
                <th>间隔</th>
                <th>连续失败</th>
                <th>最近错误</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {states.map((s) => (
                <tr key={s.id}>
                  <td>
                    <span className="font-medium">{domainLabels[s.domain] || s.domain}</span>
                  </td>
                  <td>
                    <span
                      className={`badge ${
                        stateBadge[
                          s.consecutive_failures > 0 ? 'error' : s.last_error_class ? 'warn' : 'ok'
                        ]
                      }`}
                    >
                      {s.consecutive_failures > 0 ? '失败' : s.last_error_class ? '警告' : '正常'}
                    </span>
                  </td>
                  <td className="text-muted whitespace-nowrap text-sm">
                    {s.last_run_at ? timeDisplay(s.last_run_at) : '—'}
                  </td>
                  <td className="text-muted whitespace-nowrap text-sm">
                    {s.next_run_at ? timeDisplay(s.next_run_at) : '—'}
                  </td>
                  <td className="text-sm">{s.interval_s}s</td>
                  <td className="text-sm">{s.consecutive_failures}</td>
                  <td className="max-w-[200px] truncate text-xs text-muted" title={s.last_error_class}>
                    {s.last_error_class || '—'}
                  </td>
                  <td>
                    <Link
                      to={`/sites/${id}/usage/logs`}
                      className="text-xs text-primary-600 hover:text-primary-700 dark:text-primary-400"
                    >
                      明细
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
