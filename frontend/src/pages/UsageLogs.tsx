import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { apiGetSite, apiListUsageLogs } from '../api/endpoints'
import type { Site, UsageLog } from '../types'
import { timeDisplay } from '../components/ui'

const PAGE_SIZE = 100

export default function UsageLogs() {
  const { id } = useParams<{ id: string }>()
  const [site, setSite] = useState<Site | null>(null)
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  // 过滤器
  const [start, setStart] = useState('')
  const [end, setEnd] = useState('')
  const [model, setModel] = useState('')

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    setErr(null)
    apiListUsageLogs(id, { start: start || undefined, end: end || undefined, model: model || undefined, limit: PAGE_SIZE })
      .then((data) => {
        setLogs(data)
        setLoading(false)
      })
      .catch((e) => {
        setErr(e?.message || '加载失败')
        setLoading(false)
      })
  }, [id, start, end, model])

  useEffect(() => {
    if (!id) return
    apiGetSite(id).then(setSite).catch(() => setSite(null))
    load()
  }, [id, load])

  const apply = () => load()

  const reset = () => {
    setStart('')
    setEnd('')
    setModel('')
  }

  if (!id) return null

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div>
        <Link to="/sites" className="text-sm text-gray-500 hover:text-primary-600 dark:text-gray-400">
          ← 返回站点列表
        </Link>
        <h1 className="mt-2 text-2xl font-bold text-gray-900 dark:text-white">
          调用明细{site ? ` — ${site.name}` : ''}
        </h1>
        <p className="mt-1 text-sm text-muted">站点用量日志（服务端分页，limit=100；currency 仅标明不折算）</p>
      </div>

      {/* 过滤器 */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="flex-1 min-w-[140px]">
          <label className="input-label">开始日期</label>
          <input className="input" type="date" value={start} onChange={(e) => setStart(e.target.value)} />
        </div>
        <div className="flex-1 min-w-[140px]">
          <label className="input-label">结束日期</label>
          <input className="input" type="date" value={end} onChange={(e) => setEnd(e.target.value)} />
        </div>
        <div className="flex-1 min-w-[140px]">
          <label className="input-label">模型名</label>
          <input className="input" placeholder="gpt-4o" value={model} onChange={(e) => setModel(e.target.value)} />
        </div>
        <div className="flex gap-2">
          <button className="btn btn-primary btn-sm" onClick={apply} disabled={loading}>
            {loading ? '加载中…' : '筛选'}
          </button>
          <button className="btn btn-secondary btn-sm" onClick={reset} disabled={loading}>
            重置
          </button>
        </div>
      </div>

      {/* 表格 */}
      {err ? (
        <p className="text-danger">{err}</p>
      ) : (
        <div className="table-container">
          <table className="table">
            <thead>
              <tr>
                <th>时间</th>
                <th>模型</th>
                <th>Key</th>
                <th>提示</th>
                <th>补全</th>
                <th>总计</th>
                <th>金额</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id}>
                  <td className="text-muted whitespace-nowrap text-xs">{timeDisplay(l.ts)}</td>
                  <td className="font-mono text-xs">{l.model_name || '—'}</td>
                  <td className="font-mono text-xs">{l.api_key_mask || '—'}</td>
                  <td className="font-mono text-xs">{l.prompt_tokens.toLocaleString()}</td>
                  <td className="font-mono text-xs">{l.completion_tokens.toLocaleString()}</td>
                  <td className="font-mono text-xs">{l.total_tokens.toLocaleString()}</td>
                  <td className="font-mono text-xs">
                    {l.amount} <span className="text-xs text-muted">({l.currency})</span>
                  </td>
                  <td>
                    <span className={`badge ${l.status === 'ok' ? 'badge-success' : l.status === 'err' ? 'badge-danger' : 'badge-muted'}`}>
                      {l.status}
                    </span>
                    {l.err_code && <span className="ml-1 text-xs text-muted">{l.err_code}</span>}
                  </td>
                </tr>
              ))}
              {logs.length === 0 && !loading && (
                <tr>
                  <td colSpan={8}>
                    <div className="empty-state">
                      <span className="text-3xl">📊</span>
                      <span className="empty-state-title">暂无用量日志</span>
                      <span className="empty-state-desc">尚未同步或筛选条件不匹配</span>
                    </div>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
