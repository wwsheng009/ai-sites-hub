import { useEffect, useState } from 'react'
import { get } from '../api/client'
import type { DoctorReport, Site, SiteAffiliate } from '../types'

export default function Dashboard() {
  const [sites, setSites] = useState<Site[]>([])
  const [affs, setAffs] = useState<SiteAffiliate[]>([])
  const [doctor, setDoctor] = useState<DoctorReport | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([
      get<Site[]>('/sites'),
      get<SiteAffiliate[]>('/affiliates'),
      get<DoctorReport>('/doctor'),
    ])
      .then(([s, a, d]) => {
        setSites(s)
        setAffs(a)
        setDoctor(d)
      })
      .catch((e) => setError(String(e.message ?? e)))
  }, [])

  const byType = sites.reduce<Record<string, number>>((m, s) => {
    m[s.site_type] = (m[s.site_type] ?? 0) + 1
    return m
  }, {})

  return (
    <div>
      <h1 className="page-title">概览</h1>
      {error && <p className="error-text">{error}</p>}

      <div className="panel">
        <h3>站点统计</h3>
        <p className="muted">
          共 {sites.length} 个站点
          {Object.entries(byType).map(([t, n]) => ` · ${t}: ${n}`).join('')}
        </p>
      </div>

      <div className="panel">
        <h3>返利概览（FR-10）</h3>
        {affs.length === 0 ? (
          <p className="muted">暂无返利数据；同步站点后自动获取（AffiliateInfo）。</p>
        ) : (
          <table className="data">
            <thead>
              <tr>
                <th>站点</th>
                <th>可用余额</th>
                <th>累计</th>
                <th>邀请人数</th>
                <th>更新时间</th>
              </tr>
            </thead>
            <tbody>
              {affs.map((a) => (
                <tr key={a.id}>
                  <td>{sites.find((s) => s.id === a.site_id)?.name ?? a.site_id}</td>
                  <td>{a.available ?? '—'}</td>
                  <td>{a.history ?? '—'}</td>
                  <td>{a.invitee_count ?? '—'}</td>
                  <td className="muted">{a.last_sync_at ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="panel">
        <h3>系统体检（doctor）</h3>
        {doctor ? (
          <>
            <p>
              overall: <span className={`badge ${doctor.overall === 'ok' ? 'ok' : doctor.overall === 'warn' ? 'warn' : 'fail'}`}>{doctor.overall}</span>
            </p>
            <ul>
              {doctor.checks.map((c) => (
                <li key={c.name}>
                  <span className={`badge ${c.status === 'ok' ? 'ok' : c.status === 'warn' ? 'warn' : 'fail'}`}>{c.status}</span>{' '}
                  <strong>{c.name}</strong> — {c.detail}
                </li>
              ))}
            </ul>
          </>
        ) : (
          <p className="muted">加载中…</p>
        )}
      </div>
    </div>
  )
}
