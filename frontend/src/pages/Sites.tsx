import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { get, post } from '../api/client'
import type { Site } from '../types'

export default function Sites() {
  const [sites, setSites] = useState<Site[]>([])
  const [baseUrl, setBaseUrl] = useState('')
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const load = () => get<Site[]>('/sites').then(setSites).catch((e) => setError(String(e.message ?? e)))

  useEffect(() => {
    load()
  }, [])

  const create = async () => {
    if (!baseUrl.trim()) return
    setBusy(true)
    setError('')
    try {
      await post('/sites', { base_url: baseUrl.trim(), name: name.trim() })
      setBaseUrl('')
      setName('')
      await load()
    } catch (e) {
      setError(String((e as Error).message ?? e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <h1 className="page-title">站点管理</h1>
      {error && <p className="error-text">{error}</p>}

      <div className="panel">
        <h3>新增站点（自动识别 sub2api / new-api）</h3>
        <div className="form-row">
          <label>站点地址（必填，如 https://api.example.com）</label>
          <input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://…" />
        </div>
        <div className="form-row">
          <label>名称（可选，默认取域名）</label>
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <button className="primary" disabled={busy} onClick={create}>
          {busy ? '创建中…' : '创建并识别'}
        </button>
      </div>

      <div className="panel">
        <h3>站点列表</h3>
        <table className="data">
          <thead>
            <tr>
              <th>名称</th>
              <th>地址</th>
              <th>类型</th>
              <th>识别分</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {sites.map((s) => (
              <tr key={s.id}>
                <td>
                  <Link to={`/sites/${s.id}`}>{s.name}</Link>
                </td>
                <td className="muted">{s.base_url}</td>
                <td>
                  <span className={`badge ${s.site_type === 'unknown' ? 'muted' : 'ok'}`}>{s.site_type}</span>
                </td>
                <td>{s.detect_score}</td>
                <td>{s.status}</td>
                <td>
                  <button onClick={() => post(`/sites/${s.id}/detect`).then(load)}>重新识别</button>
                </td>
              </tr>
            ))}
            {sites.length === 0 && (
              <tr>
                <td colSpan={6} className="muted">
                  暂无站点
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
