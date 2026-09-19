import { useCallback, useEffect, useState } from 'react'
import { get, post, put } from '../api/client'
import type { AffRule, AffTransfer, Site, SiteAffiliate } from '../types'

/** 返利中心（FR-10）：全站点返利概览 + 规则配置 + 手动划转 + 记录 */

const transferBadge = (s: string) =>
  s === 'success' ? 'badge-success' : s === 'failed' ? 'badge-danger' : s === 'pending' ? 'badge-warning' : 'badge-muted'

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
  const [busy, setBusy] = useState('')

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

  const run = async (fn: () => Promise<unknown>, okMsg: string, tag: string) => {
    setError('')
    setMsg('')
    setBusy(tag)
    try {
      const r = await fn()
      setMsg(typeof r === 'string' ? r : okMsg)
      if (selected) loadRule(selected)
    } catch (e) {
      setError(String((e as Error).message ?? e))
    } finally {
      setBusy('')
    }
  }

  const saveRule = () => run(() => put(`/affiliates/site/${selected}/rule`, rule), '规则已保存', 'rule')
  const transfer = () =>
    run(
      () =>
        post<{ state?: string; message?: string }>(`/affiliates/site/${selected}/transfer`, {
          amount: Number(amount),
          dry_run: dryRun,
        }).then((r) => `划转[${r.state ?? '?'}] ${r.message ?? ''}`),
      '划转完成',
      'transfer',
    )

  const selectedName = sites.find((s) => s.id === selected)?.name ?? selected

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">返利中心</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-dark-400">
          全站点返利概览 · 自动划转规则 · 手动划转（FR-10）
        </p>
      </div>

      {msg && <p className="text-sm text-emerald-600 dark:text-emerald-400">{msg}</p>}
      {error && <p className="text-error">{error}</p>}

      {/* 全站点返利表格 */}
      <section className="card">
        <div className="card-header flex items-center justify-between">
          <h3 className="font-semibold text-gray-900 dark:text-white">全站点返利（{affs.length}）</h3>
          {selected && <span className="badge badge-primary">已选中：{selectedName}</span>}
        </div>
        <div className="card-body p-0">
          <div className="table-container border-0">
            <table className="table">
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
                  <tr
                    key={a.id}
                    onClick={() => setSelected(a.site_id)}
                    className={`cursor-pointer ${selected === a.site_id ? 'bg-primary-50/60 dark:bg-primary-900/10' : ''}`}
                  >
                    <td className="font-medium text-gray-900 dark:text-white">
                      {sites.find((s) => s.id === a.site_id)?.name ?? a.site_id}
                    </td>
                    <td className="font-mono text-xs">{a.aff_code || '—'}</td>
                    <td>{a.available ?? '—'}</td>
                    <td>{a.frozen ?? '—'}</td>
                    <td>{a.history ?? '—'}</td>
                    <td>{a.invitee_count ?? '—'}</td>
                    <td className="text-muted whitespace-nowrap">{a.last_sync_at ?? '—'}</td>
                  </tr>
                ))}
                {affs.length === 0 && (
                  <tr>
                    <td colSpan={7}>
                      <div className="empty-state">
                        <span className="text-3xl">💰</span>
                        <span className="empty-state-title">暂无返利数据</span>
                        <span className="empty-state-desc">先在站点页配置凭据并同步</span>
                      </div>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        {/* 自动划转规则 */}
        <section className="card">
          <div className="card-header">
            <h3 className="font-semibold text-gray-900 dark:text-white">自动划转规则</h3>
            <p className="mt-0.5 text-xs text-gray-400 dark:text-dark-400">
              选中站点：{selected ? selectedName : '未选择（点击上方表格行选择）'}
            </p>
          </div>
          <div className="card-body space-y-4">
            <label className="flex cursor-pointer items-center gap-3">
              <input
                type="checkbox"
                className="h-4 w-4 accent-primary-500"
                checked={rule.enabled}
                onChange={(e) => setRule({ ...rule, enabled: e.target.checked })}
              />
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">启用自动划转</span>
            </label>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div>
                <label className="input-label">触发阈值</label>
                <input
                  className="input"
                  type="number"
                  value={rule.min_amount}
                  onChange={(e) => setRule({ ...rule, min_amount: Number(e.target.value) })}
                />
              </div>
              <div>
                <label className="input-label">单笔上限</label>
                <input
                  className="input"
                  type="number"
                  value={rule.max_per_transfer}
                  onChange={(e) => setRule({ ...rule, max_per_transfer: Number(e.target.value) })}
                />
                <p className="input-hint">0 = 不限</p>
              </div>
              <div>
                <label className="input-label">日累计上限</label>
                <input
                  className="input"
                  type="number"
                  value={rule.daily_limit}
                  onChange={(e) => setRule({ ...rule, daily_limit: Number(e.target.value) })}
                />
                <p className="input-hint">0 = 不限</p>
              </div>
            </div>
            <button className="btn btn-primary" disabled={!selected || busy === 'rule'} onClick={saveRule}>
              {busy === 'rule' ? '保存中…' : '保存规则'}
            </button>
          </div>
        </section>

        {/* 手动划转 */}
        <section className="card">
          <div className="card-header">
            <h3 className="font-semibold text-gray-900 dark:text-white">手动划转</h3>
            <p className="mt-0.5 text-xs text-gray-400 dark:text-dark-400">不可逆操作 · 默认 dry_run 仅试算</p>
          </div>
          <div className="card-body space-y-4">
            <div>
              <label className="input-label">金额（站点余额单位）</label>
              <input className="input" type="number" value={amount} onChange={(e) => setAmount(e.target.value)} />
            </div>
            <label className="flex cursor-pointer items-center gap-3">
              <input
                type="checkbox"
                className="h-4 w-4 accent-primary-500"
                checked={dryRun}
                onChange={(e) => setDryRun(e.target.checked)}
              />
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">dry_run（仅试算，不执行）</span>
            </label>
            <button className="btn btn-primary" disabled={!selected || !amount || busy === 'transfer'} onClick={transfer}>
              {busy === 'transfer' ? '执行中…' : '执行划转'}
            </button>
          </div>
        </section>
      </div>

      {/* 划转记录 */}
      <section className="card">
        <div className="card-header">
          <h3 className="font-semibold text-gray-900 dark:text-white">划转记录（审计留证）</h3>
        </div>
        <div className="card-body p-0">
          <div className="table-container border-0">
            <table className="table">
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
                    <td className="text-muted whitespace-nowrap">{t.created_at}</td>
                    <td className="font-mono text-xs">{t.amount}</td>
                    <td>
                      <span className={`badge ${transferBadge(t.state)}`}>{t.state}</span>
                    </td>
                    <td>
                      <span className={`badge ${t.trigger_type === 'manual' ? 'badge-muted' : 'badge-primary'}`}>
                        {t.trigger_type}
                      </span>
                    </td>
                    <td>{t.amount_before ?? '—'}</td>
                    <td className="text-muted max-w-xs truncate">{t.message}</td>
                  </tr>
                ))}
                {transfers.length === 0 && (
                  <tr>
                    <td colSpan={6}>
                      <div className="empty-state">
                        <span className="text-3xl">📋</span>
                        <span className="empty-state-title">暂无划转记录</span>
                      </div>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  )
}
