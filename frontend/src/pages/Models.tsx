import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiListSiteModels } from '../api/endpoints'
import type { SiteModel } from '../types'
import { timeDisplay } from '../components/ui'

export default function Models() {
  const [items, setItems] = useState<SiteModel[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    apiListSiteModels(200)
      .then((data) => {
        setItems(data)
        setLoading(false)
      })
      .catch((e) => {
        setErr(e?.message || '加载失败')
        setLoading(false)
      })
  }, [])

  // 按 model_name 聚合跨站
  const byModel = items.reduce<Record<string, SiteModel[]>>((m, item) => {
    ;(m[item.model_name] ||= []).push(item)
    return m
  }, {})

  if (loading) {
    return (
      <div className="flex items-center justify-center gap-3 py-24">
        <span className="spinner" />
        <span className="text-sm text-muted">加载中…</span>
      </div>
    )
  }

  if (err) {
    return (
      <div className="card">
        <div className="card-body">
          <p className="text-danger">{err}</p>
        </div>
      </div>
    )
  }

  const models = Object.keys(byModel)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">模型广场</h1>
        <p className="mt-1 text-sm text-muted">跨站模型/价格对照（currency 不折算，按站点分别展示）</p>
      </div>

      {models.length === 0 ? (
        <div className="card">
          <div className="card-body empty-state">
            <span className="text-3xl">🤖</span>
            <span className="empty-state-title">暂无模型数据</span>
            <span className="empty-state-desc">同步站点后自动获取模型列表</span>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {models.map((model) => {
            const sites = byModel[model]
            return (
              <div key={model} className="card">
                <div className="card-header">
                  <h3 className="font-medium text-gray-900 dark:text-white">{model}</h3>
                </div>
                <div className="card-body p-0">
                  <div className="table-container border-0">
                    <table className="table">
                      <thead>
                        <tr>
                          <th>站点</th>
                          <th>总用量（token）</th>
                          <th>金额</th>
                          <th>余额</th>
                          <th>更新时间</th>
                        </tr>
                      </thead>
                      <tbody>
                        {sites.map((s) => (
                          <tr key={s.site_id}>
                            <td>
                              <Link
                                to={`/sites/${s.site_id}`}
                                className="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                              >
                                {s.site_name || s.site_id}
                              </Link>
                            </td>
                            <td className="font-mono text-xs">{s.total_tokens.toLocaleString()}</td>
                            <td className="font-mono text-xs">
                              {s.amount} <span className="text-xs text-muted">({s.currency})</span>
                            </td>
                            <td className="font-mono text-xs">{s.balance ?? '—'}</td>
                            <td className="text-muted whitespace-nowrap text-xs">{s.freshness}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )
          })}
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
