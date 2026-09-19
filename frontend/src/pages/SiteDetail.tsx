import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { get, post, put } from '../api/client'
import type { Site, SiteAffiliate, SiteCredentialState, SiteGroup, SiteKey } from '../types'

export default function SiteDetail() {
  const { id } = useParams()
  const [site, setSite] = useState<Site | null>(null)
  const [auth, setAuth] = useState<SiteCredentialState | null>(null)
  const [keys, setKeys] = useState<SiteKey[]>([])
  const [groups, setGroups] = useState<SiteGroup[]>([])
  const [aff, setAff] = useState<SiteAffiliate | null>(null)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

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

  const run = async (fn: () => Promise<unknown>, okMsg: string) => {
    setError('')
    setMsg('')
    try {
      await fn()
      setMsg(okMsg)
      load()
    } catch (e) {
      setError(String((e as Error).message ?? e))
    }
  }

  if (!site) return <p className="muted">加载中…</p>

  return (
    <div>
      <h1 className="page-title">
        {site.name} <span className={`badge ${site.site_type === 'unknown' ? 'muted' : 'ok'}`}>{site.site_type}</span>
      </h1>
      <p className="muted">{site.base_url}</p>
      {msg && <p className="muted">{msg}</p>}
      {error && <p className="error-text">{error}</p>}

      <div className="panel">
        <h3>操作</h3>
        <button onClick={() => run(() => post(`/sites/${id}/detect`), '识别完成')}>重新识别</button>{' '}
        <button onClick={() => run(() => post(`/sites/${id}/sync`), '同步完成')}>同步数据</button>{' '}
        <button onClick={() => run(() => post(`/sites/${id}/auth-test`), '登录测试完成')}>登录测试</button>
      </div>

      <div className="panel">
        <h3>凭据（AES-256-GCM 加密存储，前端不留存）</h3>
        {auth && (
          <p>
            当前状态: <span className={`badge ${auth.auth_state === 'ok' ? 'ok' : auth.auth_state === 'none' ? 'muted' : 'warn'}`}>{auth.auth_state}</span>{' '}
            {auth.auth_state_msg && <span className="muted">{auth.auth_state_msg}</span>}
          </p>
        )}
        <div className="form-row">
          <label>认证方式</label>
          <select value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="username_password">账号密码</option>
            <option value="token">Token / PAT</option>
            <option value="hybrid">混合</option>
          </select>
        </div>
        {(mode === 'username_password' || mode === 'hybrid') && (
          <>
            <div className="form-row">
              <label>用户名</label>
              <input value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="form-row">
              <label>密码</label>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
          </>
        )}
        {(mode === 'token' || mode === 'hybrid') && (
          <div className="form-row">
            <label>Token / PAT</label>
            <input type="password" value={token} onChange={(e) => setToken(e.target.value)} />
          </div>
        )}
        <button
          className="primary"
          onClick={() =>
            run(
              () =>
                put(`/sites/${id}/credentials`, {
                  auth_mode: mode,
                  username,
                  password: mode === 'token' ? '' : password,
                  access_token: token,
                }),
              '凭据已保存',
            )
          }
        >
          保存凭据
        </button>
      </div>

      <div className="panel">
        <h3>Keys（{keys.length}）</h3>
        <table className="data">
          <thead>
            <tr>
              <th>名称</th>
              <th>分组</th>
              <th>状态</th>
              <th>已用/上限</th>
              <th>来源</th>
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.id}>
                <td>{k.name}</td>
                <td>{k.group}</td>
                <td>{k.status}</td>
                <td>
                  {k.unlimited
                    ? '无限'
                    : `${k.quota_used ?? '—'} / ${k.quota_limit ?? '—'}`}
                </td>
                <td className="muted">{k.key_source}</td>
              </tr>
            ))}
            {keys.length === 0 && (
              <tr>
                <td colSpan={5} className="muted">
                  暂无数据（先配置凭据并同步）
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="panel">
        <h3>分组（{groups.length}）</h3>
        <table className="data">
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
                <td>{g.name}</td>
                <td>{g.ratio}</td>
                <td>{g.available ? '是' : '否'}</td>
              </tr>
            ))}
            {groups.length === 0 && (
              <tr>
                <td colSpan={3} className="muted">
                  暂无数据
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="panel">
        <h3>站点返利（FR-10）</h3>
        {aff ? (
          <p>
            邀请码: <strong>{aff.aff_code || '—'}</strong> · 可用余额: <strong>{aff.available ?? '—'}</strong> · 累计:{' '}
            {aff.history ?? '—'} · 邀请人数: {aff.invitee_count ?? '—'}
          </p>
        ) : (
          <p className="muted">暂无返利数据（配置凭据并同步后自动获取）</p>
        )}
      </div>
    </div>
  )
}
