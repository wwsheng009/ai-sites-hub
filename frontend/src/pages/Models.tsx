import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiListSiteModels, apiListSites } from '../api/endpoints'
import type { Site, SiteModel } from '../types'
import { timeDisplay } from '../components/ui'

const SearchIcon = ({ className = 'h-4 w-4' }: { className?: string }) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35m0 0A7.5 7.5 0 1110.5 3a7.5 7.5 0 017.5 7.5z" />
  </svg>
)

export default function Models() {
  const [items, setItems] = useState<SiteModel[]>([])
  const [sites, setSites] = useState<Site[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  // 借鉴 sub2api ModelPlazaContent 的筛选栏
  const [search, setSearch] = useState('')
  const [siteFilter, setSiteFilter] = useState<string>('all')
  const [sortKey, setSortKey] = useState<'total_tokens' | 'amount'>('total_tokens')

  useEffect(() => {
    Promise.all([apiListSites(), apiListSiteModels(200)])
      .then(([s, m]) => {
        setSites(s)
        setItems(m)
        setLoading(false)
      })
      .catch((e) => {
        setErr(e?.message || '加载失败')
        setLoading(false)
      })
  }, [])

  // 按 model_name 聚合跨站，应用筛选
  const filtered = useMemo(() => {
    let result = items
    if (search) {
      const q = search.toLowerCase()
      result = result.filter((s) => s.model_name.toLowerCase().includes(q))
    }
    if (siteFilter !== 'all') {
      result = result.filter((s) => s.site_id === siteFilter)
    }
    return result
  }, [items, search, siteFilter])

  // 按 model_name 聚合
  const byModel = filtered.reduce<Record<string, SiteModel[]>>((m, item) => {
    ;(m[item.model_name] ||= []).push(item)
    return m
  }, {})

  // 按排序字段聚合
  const sortedModels = useMemo(() => {
    return Object.entries(byModel).sort((a, b) => {
      const aTotal = a[1].reduce((s, x) => s + x.total_tokens, 0)
      const bTotal = b[1].reduce((s, x) => s + x.total_tokens, 0)
      const aAmount = a[1].reduce((s, x) => s + x.amount, 0)
      const bAmount = b[1].reduce((s, x) => s + x.amount, 0)
      if (sortKey === 'total_tokens') return bTotal - aTotal
      return bAmount - aAmount
    })
  }, [byModel, sortKey])

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

  const models = sortedModels.map(([name]) => name)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">模型广场</h1>
        <p className="mt-1 text-sm text-muted">跨站模型/价格对照（currency 不折算，按站点分别展示）</p>
      </div>

      {/* 借鉴 sub2api ModelPlazaContent 的筛选栏 */}
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-gray-100 bg-white p-4 dark:border-dark-700 dark:bg-dark-800/50">
        <div className="relative w-full max-w-xs">
          <input
            type="text"
            className="input pl-8"
            placeholder="搜索模型名称..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <SearchIcon className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-dark-500" />
        </div>

        <select
          className="input w-40"
          value={siteFilter}
          onChange={(e) => setSiteFilter(e.target.value)}
        >
          <option value="all">全部站点</option>
          {sites.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>

        <select
          className="input w-36"
          value={sortKey}
          onChange={(e) => setSortKey(e.target.value as 'total_tokens' | 'amount')}
        >
          <option value="total_tokens">按用量排序</option>
          <option value="amount">按金额排序</option>
        </select>

        {(search || siteFilter !== 'all') && (
          <button
            className="btn btn-ghost btn-sm"
            onClick={() => {
              setSearch('')
              setSiteFilter('all')
            }}
          >
            重置
          </button>
        )}
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
          {sortedModels.map(([model, modelSites]) => (
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
                      {modelSites.map((s) => (
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
                          <td className="text-muted whitespace-nowrap text-xs">{timeDisplay(s.freshness)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          ))}
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
