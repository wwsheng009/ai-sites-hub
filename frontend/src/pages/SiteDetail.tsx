import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { get, post, put } from '../api/client'
import type { Site, SiteAffiliate, SiteCredentialState, SiteGroup, SiteKey } from '../types'

const typeBadge = (t: string) => (t === 'unknown' ? 'badge-muted' : 'badge-primary')
const stateBadge = (s: string) => (s === 'ok' ? 'badge-success' : s === 'none' ? 'badge-muted' : 'badge-warning')

type Tab = 'keys' | 'groups' | 'affiliate'

export default function SiteDetail() {
  const { id } = useParams()
  const [site, setSite] = useState<Site | null>(null)
  const [auth, setAuth] = useState<SiteCredentialState | null>(null)
  const [keys, setKeys] = useState<SiteKey[]>([])
  const [groups, setGroups] = useState<SiteGroup[]>([])
  const [aff, setAff] = useState<SiteAffiliate | null>(null)
  const [tab, setTab] = useState<Tab>('keys')
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')

  // 凭据表单（明文仅提交，不落盘前端状态之外）
  const [mode, setMode] = useState('username_password')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')

  const load = useCallback(() => {
    if (!id) return
    Promise.allSettled([
      get<Site>(`/sites/${id}`),
      get<SiteCredentialState>(`/sites/${id}/auth-state`),
      get<SiteKey[]>(`/keys?site_id=${id}`),
      get<SiteGroup[]>(`/groups?site_id=${id}`),
      get<SiteAffiliate>(`/affiliates/site/${id}`),
    ]).then(([s, a, k, g, af]) => {
      if (s.status === 'fulfilled') setSite(s.value)
      if (a.status === 'fulfilled') setAuth(a.value)
      if (k.status === 'fulfilled') setKeys(k.value)
      if (g.status === 'fulfilled') setGroups(g.value)
      if (af.status === 'fulfilled') setAff(af.value)
    })
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const run = async (fn: () => Promise<unknown>, okMsg: string, tag: string) => {
    setError('')
    setMsg('')
    setBusy(tag)
    try {
      await fn()
      setMsg(okMsg)
      load()
    } catch (e) {
      setError(String((e as Error).message ?? e))
    } finally {
      setBusy('')
    }
  }

  if (!site) {
    return (
      <div className="flex items-center justify-center gap-3 py-24">
        <span className="spinner" />
        <span className="text-sm text-muted">加载中…</span>
      </div>
    )
  }

  const tabs: { key: Tab; label: string; count?: number }[] = [
    { key: 'keys', label: 'Keys', count: keys.length },
    { key: 'groups', label: '分组', count: groups.length },
    { key: 'affiliate', label: '返利' },
  ]

  return (
    <div className="space-y-6">
      {/* 返回 + 页头 */}
      <div>
        <Link to="/sites" className="text-sm text-gray-500 hover:text-primary-600 dark:text-dark-400">
          ← 返回站点列表
        </Link>
        <div className="mt-2 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-bold text-gray-900 dark:text-white">
              {site.name}
              <span className={`badge ${typeBadge(site.site_type)}`}>{site.site_type}</span>
              <span className={`badge ${stateBadge(site.status === 'active' ? 'ok' : site.status)}`}>{site.status}</span>
            </h1>
            <p className="mt-1 font-mono text-sm text-gray-500 dark:text-dark-400">{site.base_url}</p>
          </div>
          {/* 操作区 */}
          <div className="flex flex-wrap gap-2">
            <button className="btn btn-secondary btn-sm" disabled={busy === 'detect'} onClick={() => run(() => post(`/sites/${id}/detect`), '识别完成', 'detect')}>
              {busy === 'detect' ? '识别中…' : '重新识别'}
            </button>
            <button className="btn btn-secondary btn-sm" disabled={busy === 'sync'} onClick={() => run(() => post(`/sites/${id}/sync`), '同步完成', 'sync')}>
              {busy === 'sync' ? '同步中…' : '同步数据'}
            </button>
            <button className="btn btn-secondary btn-sm" disabled={busy === 'authtest'} onClick={() => run(() => post(`/sites/${id}/auth-test`), '登录测试完成', 'authtest')}>
              {busy === 'authtest' ? '测试中…' : '登录测试'}
            </button>
          </div>
        </div>
      </div>

      {msg && <p className="text-sm text-emerald-600 dark:text-emerald-400">{msg}</p>}
      {error && <p className="text-error">{error}</p>}

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        {/* 左侧：数据 Tab */}
        <div className="xl:col-span-2">
          <div className="card">
            {/* Tab 栏 */}
            <div className="flex gap-1 border-b border-gray-100 px-4 pt-3 dark:border-dark-700">
              {tabs.map((t) => (
                <button
                  key={t.key}
                  onClick={() => setTab(t.key)}
                  className={`rounded-t-xl px-4 py-2.5 text-sm font-medium transition-colors ${
                    tab === t.key
                      ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-400'
                      : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'
                  }`}
                >
                  {t.label}
                  {t.count !== undefined && <span className="badge badge-muted ml-1.5">{t.count}</span>}
                </button>
              ))}
            </div>

            {/* Keys */}
            {tab === 'keys' && (
              <div className="table-container border-0 rounded-t-none">
                <table className="table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>分组</th>
                      <th>状态</th>
                      <th>已用 / 上限</th>
                      <th>来源</th>
                    </tr>
                  </thead>
                  <tbody>
                    {keys.map((k) => (
                      <tr key={k.id}>
                        <td className="font-medium text-gray-900 dark:text-white">{k.name}</td>
                        <td>{k.group}</td>
                        <td>
                          <span className={`badge ${k.status === 'active' ? 'badge-success' : 'badge-muted'}`}>{k.status}</span>
                        </td>
                        <td>
                          {k.unlimited ? (
                            <span className="badge badge-primary">无限</span>
                          ) : (
                            <span className="font-mono text-xs">
                              {k.quota_used ?? '—'} / {k.quota_limit ?? '—'}
                            </span>
                          )}
                        </td>
                        <td className="text-muted">{k.key_source}</td>
                      </tr>
                    ))}
                    {keys.length === 0 && (
                      <tr>
                        <td colSpan={5}>
                          <div className="empty-state">
                            <span className="text-3xl">🔑</span>
                            <span className="empty-state-title">暂无 Keys</span>
                            <span className="empty-state-desc">先配置凭据并同步站点数据</span>
                          </div>
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            )}

            {/* 分组 */}
            {tab === 'groups' && (
              <div className="table-container border-0 rounded-t-none">
                <table className="table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>倍率</th>
                      <th>可用</th>
                    </tr>
                  </thead>
                  <tbody>
                    {groups.map((g) => (
                      <tr key={g.id}>
                        <td className="font-medium text-gray-900 dark:text-white">{g.name}</td>
                        <td className="font-mono text-xs">{g.ratio}</td>
                        <td>
                          <span className={`badge ${g.available ? 'badge-success' : 'badge-muted'}`}>{g.available ? '是' : '否'}</span>
                        </td>
                      </tr>
                    ))}
                    {groups.length === 0 && (
                      <tr>
                        <td colSpan={3}>
                          <div className="empty-state">
                            <span className="text-3xl">📦</span>
                            <span className="empty-state-title">暂无分组数据</span>
                          </div>
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            )}

            {/* 返利 */}
            {tab === 'affiliate' && (
              <div className="card-body">
                {aff ? (
                  <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                    <div>
                      <div className="stat-label">邀请码</div>
                      <div className="mt-1 font-mono text-sm font-semibold text-gray-900 dark:text-white">{aff.aff_code || '—'}</div>
                    </div>
                    <div>
                      <div className="stat-label">可用余额</div>
                      <div className="stat-value text-xl">{aff.available ?? '—'}</div>
                    </div>
                    <div>
                      <div className="stat-label">累计</div>
                      <div className="stat-value text-xl">{aff.history ?? '—'}</div>
                    </div>
                    <div>
                      <div className="stat-label">邀请人数</div>
                      <div className="stat-value text-xl">{aff.invitee_count ?? '—'}</div>
                    </div>
                  </div>
                ) : (
                  <div className="empty-state">
                    <span className="text-3xl">💰</span>
                    <span className="empty-state-title">暂无返利数据</span>
                    <span className="empty-state-desc">配置凭据并同步后自动获取（FR-10）</span>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* 右侧：凭据卡片 */}
        <div className="card self-start">
          <div className="card-header">
            <h3 className="font-semibold text-gray-900 dark:text-white">凭据</h3>
            <p className="mt-0.5 text-xs text-gray-400 dark:text-dark-400">AES-256-GCM 加密存储，前端不留存</p>
          </div>
          <div className="card-body space-y-4">
            {auth && (
              <div className="flex items-center gap-2 rounded-xl bg-gray-50 p-3 dark:bg-dark-800/50">
                <span className={`badge ${stateBadge(auth.auth_state)}`}>{auth.auth_state}</span>
                {auth.auth_state_msg && <span className="text-xs text-muted">{auth.auth_state_msg}</span>}
              </div>
            )}
            <div>
              <label className="input-label">认证方式</label>
              <select className="input" value={mode} onChange={(e) => setMode(e.target.value)}>
                <option value="username_password">账号密码</option>
                <option value="token">Token / PAT</option>
                <option value="hybrid">混合</option>
              </select>
            </div>
            {(mode === 'username_password' || mode === 'hybrid') && (
              <>
                <div>
                  <label className="input-label">用户名</label>
                  <input className="input" value={username} onChange={(e) => setUsername(e.target.value)} />
                </div>
                <div>
                  <label className="input-label">密码</label>
                  <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
                </div>
              </>
            )}
            {(mode === 'token' || mode === 'hybrid') && (
              <div>
                <label className="input-label">Token / PAT</label>
                <input className="input" type="password" value={token} onChange={(e) => setToken(e.target.value)} />
              </div>
            )}
            <button
              className="btn btn-primary w-full"
              disabled={busy === 'cred'}
              onClick={() => run(
                () =>
                  put(`/sites/${id}/credentials`, {
                    auth_mode: mode,
                    username,
                    password: mode === 'token' ? '' : password,
                    access_token: token,
                  }),
                '凭据已保存',
                'cred',
              )}
            >
              {busy === 'cred' ? '保存中…' : '保存凭据'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
