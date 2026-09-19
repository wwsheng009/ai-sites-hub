import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  apiCreateSite,
  apiDeleteSite,
  apiDetectSite,
  apiListSites,
  apiSyncAll,
  apiUpdateSite,
} from '../api/endpoints'
import type { Site } from '../types'
import { useToast, errMsg } from '../components/Toast'
import Modal from '../components/Modal'
import ConfirmDialog from '../components/ConfirmDialog'
import { siteTypeBadgeCls, statusBadgeCls, timeDisplay } from '../components/ui'

type EditState = { id: string; name: string; proxy_url: string; site_type: string }

export default function Sites() {
  const toast = useToast()
  const [sites, setSites] = useState<Site[]>([])
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [edit, setEdit] = useState<EditState | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Site | null>(null)
  const [busy, setBusy] = useState('')

  // 创建表单
  const [baseUrl, setBaseUrl] = useState('')
  const [name, setName] = useState('')
  const [proxyUrl, setProxyUrl] = useState('')

  const load = useCallback(async () => {
    try {
      setSites(await apiListSites())
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    load()
  }, [load])

  const filtered = useMemo(
    () =>
      sites.filter((s) => {
        if (statusFilter && s.status !== statusFilter) return false
        if (!search) return true
        const q = search.toLowerCase()
        return s.name.toLowerCase().includes(q) || s.base_url.toLowerCase().includes(q)
      }),
    [sites, search, statusFilter],
  )

  const statuses = useMemo(() => Array.from(new Set(sites.map((s) => s.status))).sort(), [sites])

  const openCreate = () => {
    setBaseUrl('')
    setName('')
    setProxyUrl('')
    setShowCreate(true)
  }

  const create = async () => {
    setBusy('create')
    try {
      await apiCreateSite({ base_url: baseUrl.trim(), name: name.trim(), proxy_url: proxyUrl.trim() })
      toast.success('站点已创建并开始识别')
      setShowCreate(false)
      await load()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }

  const detect = async (s: Site) => {
    setBusy(`detect-${s.id}`)
    try {
      await apiDetectSite(s.id)
      toast.success(`「${s.name}」识别完成`)
      await load()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }

  const syncAll = async () => {
    setBusy('sync-all')
    try {
      const results = await apiSyncAll()
      const failed = results.filter((r) => r.error).length
      if (failed > 0) {
        toast.warning(`全站同步完成，${results.length - failed} 成功 / ${failed} 失败`)
      } else {
        toast.success(`全站同步完成（${results.length} 个站点）`)
      }
      await load()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }

  const openEdit = (s: Site) => {
    setEdit({ id: s.id, name: s.name, proxy_url: s.proxy_url ?? '', site_type: s.site_type })
  }

  const saveEdit = async () => {
    if (!edit) return
    setBusy('edit')
    try {
      await apiUpdateSite(edit.id, {
        name: edit.name.trim() || undefined,
        proxy_url: edit.proxy_url.trim(),
        site_type: edit.site_type,
      })
      toast.success('站点信息已更新')
      setEdit(null)
      await load()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }

  const doDelete = async () => {
    if (!deleteTarget) return
    setBusy('delete')
    try {
      await apiDeleteSite(deleteTarget.id)
      toast.success(`站点「${deleteTarget.name}」已删除`)
      setDeleteTarget(null)
      await load()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
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
        <div className="flex flex-wrap gap-2">
          <button className="btn btn-secondary" disabled={busy === 'sync-all' || sites.length === 0} onClick={syncAll}>
            {busy === 'sync-all' ? (
              <>
                <span className="spinner !h-4 !w-4 !border-2" /> 同步中…
              </>
            ) : (
              '⟳ 全站同步'
            )}
          </button>
          <button className="btn btn-primary" onClick={openCreate}>
            ＋ 新增站点
          </button>
        </div>
      </div>

      {/* 搜索 + 状态过滤 */}
      <div className="card flex flex-col gap-3 p-4 sm:flex-row">
        <input
          className="input flex-1"
          placeholder="搜索站点名称或地址…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select className="input sm:w-44" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="">全部状态</option>
          {statuses.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      {/* 站点表格 */}
      <section className="card">
        <div className="card-header flex items-center justify-between">
          <h3 className="font-semibold text-gray-900 dark:text-white">站点列表（{filtered.length}）</h3>
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
                    <th>名称</th>
                    <th>地址</th>
                    <th>类型</th>
                    <th>识别分</th>
                    <th>状态</th>
                    <th>最近探测</th>
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
                      <td className="text-muted max-w-[220px] truncate font-mono text-xs">{s.base_url}</td>
                      <td>
                        <span className={`badge ${siteTypeBadgeCls(s.site_type)}`}>{s.site_type}</span>
                      </td>
                      <td className="font-mono text-xs">{s.detect_score}</td>
                      <td>
                        <span className={`badge ${statusBadgeCls(s.status)}`}>{s.status}</span>
                      </td>
                      <td className="text-muted whitespace-nowrap text-xs">{timeDisplay(s.last_probe_at)}</td>
                      <td>
                        <div className="flex justify-end gap-1.5">
                          <button
                            className="btn btn-ghost btn-sm"
                            title="编辑站点"
                            onClick={() => openEdit(s)}
                          >
                            编辑
                          </button>
                          <button
                            className="btn btn-secondary btn-sm"
                            disabled={busy === `detect-${s.id}`}
                            onClick={() => detect(s)}
                          >
                            {busy === `detect-${s.id}` ? '识别中…' : '识别'}
                          </button>
                          <button
                            className="btn btn-ghost btn-sm !text-red-500 hover:!bg-red-50 dark:hover:!bg-red-900/20"
                            title="删除站点"
                            onClick={() => setDeleteTarget(s)}
                          >
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                  {filtered.length === 0 && (
                    <tr>
                      <td colSpan={7}>
                        <div className="empty-state">
                          <span className="text-3xl">🌐</span>
                          <span className="empty-state-title">
                            {search || statusFilter ? '没有匹配的站点' : '暂无站点'}
                          </span>
                          {!search && !statusFilter && (
                            <span className="empty-state-desc">点击右上角「新增站点」添加第一个站点</span>
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

      {/* 新增站点 */}
      <Modal
        open={showCreate}
        title="新增站点"
        onClose={() => setShowCreate(false)}
        footer={
          <>
            <button className="btn btn-secondary" onClick={() => setShowCreate(false)} disabled={busy === 'create'}>
              取消
            </button>
            <button className="btn btn-primary" disabled={busy === 'create' || !baseUrl.trim()} onClick={create}>
              {busy === 'create' ? '创建中…' : '创建并识别'}
            </button>
          </>
        }
      >
        <div className="space-y-4">
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
              placeholder="http://… 或 socks5://…（留空使用全局配置）"
            />
            <p className="input-hint">需代理才能访问的站点在此填写；支持 http/https/socks5，识别与后续请求都会走该代理</p>
          </div>
        </div>
      </Modal>

      {/* 编辑站点 */}
      <Modal
        open={edit !== null}
        title="编辑站点"
        onClose={() => setEdit(null)}
        footer={
          <>
            <button className="btn btn-secondary" onClick={() => setEdit(null)} disabled={busy === 'edit'}>
              取消
            </button>
            <button className="btn btn-primary" disabled={busy === 'edit'} onClick={saveEdit}>
              {busy === 'edit' ? '保存中…' : '保存'}
            </button>
          </>
        }
      >
        {edit && (
          <div className="space-y-4">
            <div>
              <label className="input-label">名称</label>
              <input className="input" value={edit.name} onChange={(e) => setEdit({ ...edit, name: e.target.value })} />
            </div>
            <div>
              <label className="input-label">站点类型</label>
              <select
                className="input"
                value={edit.site_type}
                onChange={(e) => setEdit({ ...edit, site_type: e.target.value })}
              >
                <option value="unknown">自动（unknown）</option>
                <option value="sub2api">sub2api</option>
                <option value="new-api">new-api</option>
              </select>
              <p className="input-hint">手动指定后可绕过自动识别</p>
            </div>
            <div>
              <label className="input-label">出站代理（可选）</label>
              <input
                className="input"
                value={edit.proxy_url}
                onChange={(e) => setEdit({ ...edit, proxy_url: e.target.value })}
                placeholder="http://… 或 socks5://…（留空使用全局配置）"
              />
              <p className="input-hint">支持 http/https/socks5；清空则回落全局 proxy.url</p>
            </div>
          </div>
        )}
      </Modal>

      {/* 删除确认 */}
      <ConfirmDialog
        open={deleteTarget !== null}
        title="删除站点"
        message={
          <>
            确定删除站点 <strong className="text-gray-900 dark:text-white">「{deleteTarget?.name}」</strong>
            吗？该站点的凭据、Keys、分组与返利数据将一并清除，此操作不可恢复。
          </>
        }
        busy={busy === 'delete'}
        onConfirm={doDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
