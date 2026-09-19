import { useCallback, useEffect, useState } from 'react'
import { get, post, put } from '../api/client'
import type { AffRule, AffTransfer, Site, SiteAffiliate } from '../types'

/** 返利中心（FR-10）：全站点返利概览 + 规则配置 + 手动划转 + 记录 */
export default function Affiliates() {
  const [sites, setSites] = useState<Site[]>([])
  const [affs, setAffs] = useState<SiteAffiliate[]>([])
  const [selected, setSelected] = useState('')
  const [rule, setRule] = useState<AffRule>({ enabled: false, min_amount: 0, max_per_transfer: 0, daily_limit: 0 })
  const [transfers, setTransfers] = useState<AffTransfer[]>([])
  const [amount, setAmount] = useState('')
  const [dryRun, setDryRun] = useState(true)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  const loadRule = useCallback((siteId: string) => {
    get<AffRule>(`/affiliates/site/${siteId}/rule`).then(setRule).catch(() => {})
    get<AffTransfer[]>(`/affiliates/site/${siteId}/transfers`).then(setTransfers).catch(() => {})
  }, [])

  useEffect(() => {
    Promise.all([get<Site[]>('/sites'), get<SiteAffiliate[]>('/affiliates')])
      .then(([s, a]) => {
        setSites(s)
        setAffs(a)
      })
      .catch((e) => setError(String(e.message ?? e)))
  }, [])

  useEffect(() => {
    if (selected) loadRule(selected)
  }, [selected, loadRule])

  const run = async (fn: () => Promise<unknown>, okMsg: string) => {
    setError('')
    setMsg('')
    try {
      const r = await fn()
      setMsg(typeof r === 'string' ? r : okMsg)
      if (selected) loadRule(selected)
    } catch (e) {
      setError(String((e as Error).message ?? e))
    }
  }

  const saveRule = () =>
    run(() => put(`/affiliates/site/${selected}/rule`, rule), '规则已保存')

  const transfer = () =>
    run(
      () =>
        post<{ state?: string; message?: string }>(`/affiliates/site/${selected}/transfer`, {
          amount: Number(amount),
          dry_run: dryRun,
        }).then((r) => `划转[${r.state ?? '?'}] ${r.message ?? ''}`),
      '划转完成',
    )

  return (
    <div>
      <h1 className="page-title">站点返利中心（FR-10）</h1>
      {msg && <p className="muted">{msg}</p>}
      {error && <p className="error-text">{error}</p>}

      <div className="panel">
        <h3>全站点返利</h3>
        <table className="data">
          <thead>
            <tr>
              <th>站点</th>
              <th>邀请码</th>
              <th>可用</th>
              <th>冻结</th>
              <th>累计</th>
              <th>邀请人数</th>
              <th>同步时间</th>
            </tr>
          </thead>
          <tbody>
            {affs.map((a) => (
              <tr key={a.id} onClick={() => setSelected(a.site_id)} style={{ cursor: 'pointer' }}>
                <td>{sites.find((s) => s.id === a.site_id)?.name ?? a.site_id}</td>
                <td className="muted">{a.aff_code || '—'}</td>
                <td>{a.available ?? '—'}</td>
                <td>{a.frozen ?? '—'}</td>
                <td>{a.history ?? '—'}</td>
                <td>{a.invitee_count ?? '—'}</td>
                <td className="muted">{a.last_sync_at ?? '—'}</td>
              </tr>
            ))}
            {affs.length === 0 && (
              <tr>
                <td colSpan={7} className="muted">
                  暂无数据（先在站点页配置凭据并同步）
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="panel">
        <h3>自动划转规则（选中站点: {selected || '—'}）</h3>
        <div className="form-row">
          <label>
            <input type="checkbox" checked={rule.enabled} onChange={(e) => setRule({ ...rule, enabled: e.target.checked })} /> 启用自动划转
          </label>
        </div>
        <div className="form-row">
          <label>阈值（达到该余额触发）</label>
          <input type="number" value={rule.min_amount} onChange={(e) => setRule({ ...rule, min_amount: Number(e.target.value) })} />
        </div>
        <div className="form-row">
          <label>单笔上限（0=不限）</label>
          <input type="number" value={rule.max_per_transfer} onChange={(e) => setRule({ ...rule, max_per_transfer: Number(e.target.value) })} />
        </div>
        <div className="form-row">
          <label>日累计上限（0=不限）</label>
          <input type="number" value={rule.daily_limit} onChange={(e) => setRule({ ...rule, daily_limit: Number(e.target.value) })} />
        </div>
        <button className="primary" disabled={!selected} onClick={saveRule}>
          保存规则
        </button>
      </div>

      <div className="panel">
        <h3>手动划转（不可逆；默认 dry_run 试算）</h3>
        <div className="form-row">
          <label>金额（站点余额单位）</label>
          <input type="number" value={amount} onChange={(e) => setAmount(e.target.value)} />
        </div>
        <div className="form-row">
          <label>
            <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} /> dry_run（仅试算，不执行）
          </label>
        </div>
        <button className="primary" disabled={!selected || !amount} onClick={transfer}>
          执行划转
        </button>
      </div>

      <div className="panel">
        <h3>划转记录（审计留证）</h3>
        <table className="data">
          <thead>
            <tr>
              <th>时间</th>
              <th>金额</th>
              <th>状态</th>
              <th>触发</th>
              <th>划转前余额</th>
              <th>消息</th>
            </tr>
          </thead>
          <tbody>
            {transfers.map((t) => (
              <tr key={t.id}>
                <td className="muted">{t.created_at}</td>
                <td>{t.amount}</td>
                <td>
                  <span className={`badge ${t.state === 'success' ? 'ok' : t.state === 'failed' ? 'fail' : 'muted'}`}>{t.state}</span>
                </td>
                <td>{t.trigger_type}</td>
                <td>{t.amount_before ?? '—'}</td>
                <td className="muted">{t.message}</td>
              </tr>
            ))}
            {transfers.length === 0 && (
              <tr>
                <td colSpan={6} className="muted">
                  暂无记录
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
