import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiListAnnouncements } from '../api/endpoints'
import type { SiteAnnouncement } from '../types'
import { timeDisplay } from '../components/ui'

export default function Announcements() {
  const [items, setItems] = useState<SiteAnnouncement[]>([])
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    apiListAnnouncements(100)
      .then((data) => {
        setItems(data)
        setLoading(false)
      })
      .catch((e) => {
        setErr(e?.message || '加载失败')
        setLoading(false)
      })
  }, [])

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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">公告</h1>
        <p className="mt-1 text-sm text-muted">跨站公告聚合（hash 去重，仅显示站点元数据）</p>
      </div>

      {items.length === 0 ? (
        <div className="card">
          <div className="card-body empty-state">
            <span className="text-3xl">📭</span>
            <span className="empty-state-title">暂无公告</span>
            <span className="empty-state-desc">站点公告同步后自动聚合展示</span>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {items.map((a, i) => (
            <div key={`${a.site_id}-${a.title}-${i}`} className="card">
              <div className="card-header">
                <div className="flex flex-col gap-1">
                  <div className="flex items-baseline gap-2">
                    <span className="font-medium text-gray-900 dark:text-white">{a.title || '（无标题）'}</span>
                    {a.site_name && <span className="text-xs text-muted">来自 {a.site_name}</span>}
                  </div>
                  {a.published_at && (
                    <span className="text-xs text-muted">{timeDisplay(a.published_at)}</span>
                  )}
                </div>
              </div>
              <div className="card-body">
                <p className="text-sm text-gray-700 dark:text-dark-300 whitespace-pre-wrap break-words">{a.content}</p>
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
