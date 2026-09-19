/**
 * 公告通知铃铛（借鉴 sub2api AnnouncementBell）
 * 多站点公告聚合，点击展开最近公告列表。
 */
import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiListAnnouncements } from '../api/endpoints'
import type { SiteAnnouncement } from '../types'
import { timeDisplay } from './ui'

export default function AnnouncementBell() {
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<SiteAnnouncement[]>([])
  const [loading, setLoading] = useState(false)
  const [unread, setUnread] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)

  const load = () => {
    setLoading(true)
    apiListAnnouncements(30)
      .then((data) => {
        setItems(data)
        // 简单认为最近 24 小时的公告为未读
        const cutoff = Date.now() - 86400000
        setUnread(data.filter((a) => a.published_at && new Date(a.published_at).getTime() > cutoff).length)
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [])

  // 点击外部关闭
  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [])

  const markAllRead = () => {
    setUnread(0)
  }

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => {
          setOpen(!open)
          if (!open && items.length === 0) load()
        }}
        className="relative flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition-all hover:bg-gray-100 hover:scale-105 dark:text-gray-400 dark:hover:bg-dark-800"
        aria-label="公告"
        title="公告"
      >
        <BellIcon />
        {unread > 0 && (
          <span className="absolute right-1 top-1 flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-500 opacity-75"></span>
            <span className="relative inline-flex h-2 w-2 rounded-full bg-red-500"></span>
          </span>
        )}
      </button>

      {open && (
        <div className="animate-fade-in fixed inset-y-4 right-4 z-[100] w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10">
          <div className="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">公告</h3>
              {unread > 0 && (
                <button
                  onClick={markAllRead}
                  className="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                >
                  全部已读
                </button>
              )}
            </div>
          </div>
          <div className="max-h-[60vh] overflow-y-auto">
            {loading ? (
              <div className="flex items-center justify-center py-12">
                <span className="spinner" />
              </div>
            ) : items.length === 0 ? (
              <div className="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                <span className="text-2xl">📭</span>
                <p className="mt-2">暂无公告</p>
              </div>
            ) : (
              <div className="divide-y divide-gray-100 dark:divide-dark-700">
                {items.map((a, i) => (
                  <Link
                    key={`${a.site_id}-${a.title}-${i}`}
                    to="/announcements"
                    onClick={() => setOpen(false)}
                    className="block p-4 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/30"
                  >
                    <div className="flex items-start gap-3">
                      <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-blue-100 dark:bg-blue-900/30">
                        <BellSmallIcon />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium text-gray-900 dark:text-white">
                          {a.title || '（无标题）'}
                        </p>
                        {a.site_name && (
                          <p className="text-xs text-gray-500 dark:text-dark-400">来自 {a.site_name}</p>
                        )}
                        {a.published_at && (
                          <p className="mt-1 text-xs text-gray-400 dark:text-dark-500">
                            {timeDisplay(a.published_at)}
                          </p>
                        )}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function BellIcon() {
  return (
    <svg className="h-5 w-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75v-.7V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0A3 3 0 11-5.714 0a3 3 0 015.714 0z" />
    </svg>
  )
}

function BellSmallIcon() {
  return (
    <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" strokeWidth={2.5} stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  )
}
