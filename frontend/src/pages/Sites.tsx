import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { get, post } from '../api/client'
import type { Site } from '../types'

const typeBadge = (t: string) => (t === 'unknown' ? 'badge-muted' : 'badge-primary')
const statusBadge = (s: string) =>
  s === 'ok' || s === 'active' ? 'badge-success' : s === 'error' || s === 'failed' ? 'badge-danger' : 'badge-warning'

export default function Sites() {
  const [sites, setSites] = useState<Site[]>([])
  const [search, setSearch] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [baseUrl, setBaseUrl] = useState('')
  const [name, setName] = useState('')
  const [proxyUrl, setProxyUrl] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [rowBusy, setRowBusy] = useState('')

  const load = () => get<Site[]>('/sites').then(setSites).catch((e) => setError(String(e.message ?? e)))

  useEffect(() => {
    load()
  }, [])

  const filtered = useMemo(
    () =>
      sites.filter(
        (s) =>
          !search ||
          s.name.toLowerCase().includes(search.toLowerCase()) ||
          s.base_url.toLowerCase().includes(search.toLowerCase()),
      ),
    [sites, search],
  )

  const openModal = () => {
    setBaseUrl('')
    setName('')
    setProxyUrl('')
    setError('')
    setShowModal(true)
  }

  const create = async () => {
    if (!baseUrl.trim()) return
    setBusy(true)
    setError('')
    try {
      await post('/sites', { base_url: baseUrl.trim(), name: name.trim(), proxy_url: proxyUrl.trim() })
      setShowModal(false)
      await load()
    } catch (e) {
      setError(String((e as Error).message ?? e))
    } finally {
      setBusy(false)
    }
  }

  const detect = async (s: Site) => {
    setRowBusy(s.id)
    try {
      await post(`/sites/${s.id}/detect`)
      await load()
    } finally {
      setRowBusy('')
    }
  }

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">站点管理</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-dark-400">添加站点并自动识别 sub2api / new-api 类型</p>
        </div>
        <button className="btn btn-primary" onClick={openModal}>
          ＋ 新增站点
        </button>
      </div>

      {error && !showModal && <p className="text-error">{error}</p>}

      {/* 搜索栏 */}
      <div className="card p-4">
        <input
          className="input"
          placeholder="搜索站点名称或地址…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* 站点表格 */}
      <section className="card">
        <div className="card-header flex items-center justify-between">
          <h3 className="font-semibold text-gray-900 dark:text-white">站点列表（{filtered.length}）</h3>
        </div>
        <div className="card-body p-0">
          <div className="table-container border-0">
            <table className="table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>地址</th>
                  <th>类型</th>
                  <th>识别分</th>
                  <th>状态</th>
                  <th className="text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((s) => (
                  <tr key={s.id}>
                    <td>
                      <Link
                        to={`/sites/${s.id}`}
                        className="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                      >
                        {s.name}
                      </Link>
                    </td>
                    <td className="text-muted max-w-xs truncate">{s.base_url}</td>
                    <td>
                      <span className={`badge ${typeBadge(s.site_type)}`}>{s.site_type}</span>
                    </td>
                    <td className="font-mono text-xs">{s.detect_score}</td>
                    <td>
                      <span className={`badge ${statusBadge(s.status)}`}>{s.status}</span>
                    </td>
                    <td className="text-right">
                      <button className="btn btn-secondary btn-sm" disabled={rowBusy === s.id} onClick={() => detect(s)}>
                        {rowBusy === s.id ? '识别中…' : '重新识别'}
                      </button>
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && (
                  <tr>
                    <td colSpan={6}>
                      <div className="empty-state">
                        <span className="text-3xl">🌐</span>
                        <span className="empty-state-title">{search ? '没有匹配的站点' : '暂无站点'}</span>
                        {!search && <span className="empty-state-desc">点击右上角「新增站点」添加第一个站点</span>}
                      </div>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* 新增站点模态框 */}
      {showModal && (
        <div className="modal-overlay" onClick={() => setShowModal(false)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <div className="card-header flex items-center justify-between">
              <h3 className="font-semibold text-gray-900 dark:text-white">新增站点</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowModal(false)} aria-label="关闭">
                ✕
              </button>
            </div>
            <div className="card-body space-y-4">
              {error && <p className="text-error">{error}</p>}
              <div>
                <label className="input-label">站点地址（必填）</label>
                <input
                  className="input"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="https://api.example.com"
                />
                <p className="input-hint">系统将自动探测并识别 sub2api / new-api 类型</p>
              </div>
              <div>
                <label className="input-label">名称（可选）</label>
                <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="默认取域名" />
              </div>
              <div>
                <label className="input-label">出站代理（可选）</label>
                <input
                  className="input"
                  value={proxyUrl}
                  onChange={(e) => setProxyUrl(e.target.value)}
                  placeholder="留空=直连/全局代理，如 socks5://127.0.0.1:1080"
                />
                <p className="input-hint">支持 http/https/socks5；留空则使用全局 proxy.url 配置</p>
              </div>
            </div>
            <div className="card-footer flex justify-end gap-2 px-6 py-4">
              <button className="btn btn-secondary" onClick={() => setShowModal(false)}>
                取消
              </button>
              <button className="btn btn-primary" disabled={busy || !baseUrl.trim()} onClick={create}>
                {busy ? '创建中…' : '创建并识别'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
