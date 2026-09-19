import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  apiAuthTest,
  apiDeleteSite,
  apiDetectSite,
  apiGetAffiliate,
  apiGetAffiliateInvitees,
  apiGetAffiliateRule,
  apiGetAuthState,
  apiGetSite,
  apiListEvents,
  apiListGroups,
  apiListKeys,
  apiListTransfers,
  apiPutAffiliateRule,
  apiPutCredentials,
  apiSyncSite,
  apiTransferAffiliate,
  apiUpdateSite,
} from '../api/endpoints'
import type { AffRule, AffTransfer, AffiliateInvitee, Event, Site, SiteAffiliateOut, SiteCredentialState, SiteGroup, SiteKey } from '../types'
import { useToast, errMsg } from '../components/Toast'
import Modal from '../components/Modal'
import ConfirmDialog from '../components/ConfirmDialog'
import { eventLevelBadgeCls, siteTypeBadgeCls, statusBadgeCls, timeDisplay } from '../components/ui'

type EditState = { name: string; proxy_url: string; site_type: string }
type Tab = 'keys' | 'groups' | 'affiliate' | 'events'

export default function SiteDetail() {
  const { id } = useParams()
  const navigate = useNavigate()
  const toast = useToast()

  const [site, setSite] = useState<Site | null>(null)
  const [auth, setAuth] = useState<SiteCredentialState | null>(null)
  const [keys, setKeys] = useState<SiteKey[]>([])
  const [groups, setGroups] = useState<SiteGroup[]>([])
  const [aff, setAff] = useState<SiteAffiliateOut | null>(null)
  const [invitees, setInvitees] = useState<AffiliateInvitee[]>([])
  const [events, setEvents] = useState<Event[]>([])
  const [transfers, setTransfers] = useState<AffTransfer[]>([])
  const [tab, setTab] = useState<Tab>('keys')
  const [busy, setBusy] = useState('')

  // 编辑站点
  const [edit, setEdit] = useState<EditState | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  // 凭据表单（明文仅提交，不落盘前端状态之外）
  const [mode, setMode] = useState('username_password')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')

  // 返利规则/划转表单
  const [rule, setRule] = useState<AffRule>({ enabled: false, min_amount: 0, max_per_transfer: 0, daily_limit: 1 })
  const [transferAmount, setTransferAmount] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    const [s, a, k, g, af, ev, tf, inv] = await Promise.allSettled([
      apiGetSite(id),
      apiGetAuthState(id),
      apiListKeys(id),
      apiListGroups(id),
      apiGetAffiliate(id),
      apiListEvents({ site_id: id, limit: 50 }),
      apiListTransfers(id),
      apiGetAffiliateInvitees(id),
    ])
    if (s.status === 'fulfilled') setSite(s.value)
    if (a.status === 'fulfilled') setAuth(a.value)
    if (k.status === 'fulfilled') setKeys(k.value)
    if (g.status === 'fulfilled') setGroups(g.value)
    if (af.status === 'fulfilled') {
      setAff(af.value)
      if (af.value) {
        // rule 不在返利投影内，单独拉取
        apiGetAffiliateRule(id)
          .then(setRule)
          .catch(() => setRule({ enabled: false, min_amount: 0, max_per_transfer: 0, daily_limit: 1 }))
      }
    }
    if (ev.status === 'fulfilled') setEvents(ev.value)
    if (tf.status === 'fulfilled') setTransfers(tf.value)
    if (inv.status === 'fulfilled') setInvitees(inv.value)
    else setInvitees([])
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const run = async (fn: () => Promise<unknown>, okMsg: string, tag: string) => {
    setBusy(tag)
    try {
      await fn()
      toast.success(okMsg)
      await load()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }

  const saveEdit = async () => {
    if (!edit || !id) return
    setBusy('edit')
    try {
      await apiUpdateSite(id, {
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
    if (!id) return
    setBusy('delete')
    try {
      await apiDeleteSite(id)
      toast.success('站点已删除')
      navigate('/sites')
    } catch (e) {
      toast.error(errMsg(e))
      setBusy('')
    }
  }

  const doTransfer = async (dryRun: boolean) => {
    if (!id) return
    const amount = Number(transferAmount)
    if (!amount || amount <= 0) {
      toast.warning('请输入有效的划转金额')
      return
    }
    setBusy(dryRun ? 'dry-transfer' : 'transfer')
    try {
      await apiTransferAffiliate(id, { amount, dry_run: dryRun })
      if (dryRun) {
        toast.info('试算完成，结果见划转记录（state=dry_run）')
      } else {
        toast.success('划转成功')
        setTransferAmount('')
      }
      await load()
    } catch (e) {
      toast.error(errMsg(e))
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
    { key: 'events', label: '事件', count: events.length },
  ]

  return (
    <div className="space-y-6">
      {/* 返回 + 页头 */}
      <div>
        <Link to="/sites" className="text-sm text-gray-500 hover:text-primary-600 dark:text-dark-400">
          ← 返回站点列表
        </Link>
        <div className="mt-2 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="flex flex-wrap items-center gap-2 text-2xl font-bold text-gray-900 dark:text-white">
              {site.name}
              <span className={`badge ${siteTypeBadgeCls(site.site_type)}`}>{site.site_type}</span>
              <span className={`badge ${statusBadgeCls(site.status)}`}>{site.status}</span>
            </h1>
            <p className="mt-1 font-mono text-sm text-gray-500 dark:text-dark-400">{site.base_url}</p>
            {site.proxy_url && (
              <p className="mt-0.5 font-mono text-xs text-gray-400 dark:text-dark-500">代理：{site.proxy_url}</p>
            )}
          </div>
          {/* 操作区 */}
          <div className="flex flex-wrap gap-2">
            <button className="btn btn-secondary btn-sm" onClick={() => setEdit({ name: site.name, proxy_url: site.proxy_url ?? '', site_type: site.site_type })}>
              编辑
            </button>
            <button className="btn btn-secondary btn-sm" disabled={busy === 'detect'} onClick={() => run(() => apiDetectSite(id!), '识别完成', 'detect')}>
              {busy === 'detect' ? '识别中…' : '重新识别'}
            </button>
            <button className="btn btn-secondary btn-sm" disabled={busy === 'sync'} onClick={() => run(() => apiSyncSite(id!), '同步完成', 'sync')}>
              {busy === 'sync' ? '同步中…' : '同步数据'}
            </button>
            <button className="btn btn-secondary btn-sm" disabled={busy === 'authtest'} onClick={() => run(() => apiAuthTest(id!), '登录测试完成', 'authtest')}>
              {busy === 'authtest' ? '测试中…' : '登录测试'}
            </button>
            <button
              className="btn btn-ghost btn-sm !text-red-500 hover:!bg-red-50 dark:hover:!bg-red-900/20"
              onClick={() => setConfirmDelete(true)}
            >
              删除
            </button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        {/* 左侧：数据 Tab */}
        <div className="xl:col-span-2">
          <div className="card">
            {/* Tab 栏 */}
            <div className="flex gap-1 overflow-x-auto border-b border-gray-100 px-4 pt-3 dark:border-dark-700">
              {tabs.map((t) => (
                <button
                  key={t.key}
                  onClick={() => setTab(t.key)}
                  className={`whitespace-nowrap rounded-t-xl px-4 py-2.5 text-sm font-medium transition-colors ${
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
              <div className="table-container rounded-t-none border-0">
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
              <div className="table-container rounded-t-none border-0">
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
              <div className="card-body space-y-6">
                {aff ? (
                  <>
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

                    {/* 返利规则 */}
                    <div>
                      <h4 className="mb-3 text-sm font-semibold text-gray-900 dark:text-white">自动划转规则</h4>
                      <div className="grid grid-cols-1 gap-3 sm:grid-cols-4">
                        <div>
                          <label className="input-label">启用自动划转</label>
                          <select
                            className="input"
                            value={String(rule.enabled)}
                            onChange={(e) => setRule({ ...rule, enabled: e.target.value === 'true' })}
                          >
                            <option value="false">关闭</option>
                            <option value="true">开启</option>
                          </select>
                        </div>
                        <div>
                          <label className="input-label">单笔最低金额</label>
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
                        </div>
                        <div>
                          <label className="input-label">每日次数上限</label>
                          <input
                            className="input"
                            type="number"
                            value={rule.daily_limit}
                            onChange={(e) => setRule({ ...rule, daily_limit: Number(e.target.value) })}
                          />
                        </div>
                      </div>
                      {rule.last_transfer_date && (
                        <p className="input-hint mt-1">最近自动划转日期：{rule.last_transfer_date}</p>
                      )}
                      <button
                        className="btn btn-secondary btn-sm mt-3"
                        disabled={busy === 'rule'}
                        onClick={() => run(() => apiPutAffiliateRule(id!, rule), '规则已保存', 'rule')}
                      >
                        {busy === 'rule' ? '保存中…' : '保存规则'}
                      </button>
                    </div>

                    {/* 手动划转 */}
                    <div>
                      <h4 className="mb-3 text-sm font-semibold text-gray-900 dark:text-white">手动划转</h4>
                      <div className="flex flex-wrap items-end gap-2">
                        <div className="flex-1 min-w-[160px]">
                          <label className="input-label">金额</label>
                          <input
                            className="input"
                            type="number"
                            value={transferAmount}
                            onChange={(e) => setTransferAmount(e.target.value)}
                            placeholder="0.00"
                          />
                        </div>
                        <button className="btn btn-secondary" disabled={busy === 'dry-transfer'} onClick={() => doTransfer(true)}>
                          {busy === 'dry-transfer' ? '试算中…' : '试算'}
                        </button>
                        <button className="btn btn-primary" disabled={busy === 'transfer'} onClick={() => doTransfer(false)}>
                          {busy === 'transfer' ? '划转中…' : '划转'}
                        </button>
                      </div>
                    </div>

                    {/* 划转记录 */}
                    {transfers.length > 0 && (
                      <div>
                        <h4 className="mb-3 text-sm font-semibold text-gray-900 dark:text-white">划转记录（{transfers.length}）</h4>
                        <div className="table-container rounded-xl border border-gray-100 dark:border-dark-700">
                          <table className="table">
                            <thead>
                              <tr>
                                <th>时间</th>
                                <th>金额</th>
                                <th>状态</th>
                                <th>消息</th>
                              </tr>
                            </thead>
                            <tbody>
                              {transfers.map((t) => (
                                <tr key={t.id}>
                                  <td className="text-muted whitespace-nowrap text-xs">{timeDisplay(t.created_at)}</td>
                                  <td className="font-mono text-xs">{t.amount}</td>
                                  <td>
                                    <span className={`badge ${t.state === 'success' ? 'badge-success' : t.state === 'failed' ? 'badge-danger' : t.state === 'dry_run' ? 'badge-muted' : 'badge-warning'}`}>
                                      {t.state}
                                    </span>
                                  </td>
                                  {t.message && <td className="text-muted max-w-[200px] truncate text-xs" title={t.message}>{t.message}</td>}
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      </div>
                    )}
                    {/* 受邀用户列表（参考 sub2api affiliate 页面；email 上游已脱敏） */}
                    <div>
                      <h4 className="mb-3 text-sm font-semibold text-gray-900 dark:text-white">受邀用户（{invitees.length}）</h4>
                      {invitees.length === 0 ? (
                        <p className="text-muted text-sm">暂无受邀用户</p>
                      ) : (
                        <div className="table-container rounded-xl border border-gray-100 dark:border-dark-700">
                          <table className="table">
                            <thead>
                              <tr>
                                <th>邮箱</th>
                                <th>用户名</th>
                                <th className="text-right">累计返利</th>
                                <th>加入时间</th>
                              </tr>
                            </thead>
                            <tbody>
                              {invitees.map((it) => (
                                <tr key={it.user_id}>
                                  <td className="font-medium text-gray-900 dark:text-white">{it.email || '—'}</td>
                                  <td>{it.username || '—'}</td>
                                  <td className="font-mono text-right text-emerald-600 dark:text-emerald-400">{it.total_rebate}</td>
                                  <td className="text-muted whitespace-nowrap text-xs">{timeDisplay(it.created_at)}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </div>
                  </>
                ) : (
                  <div className="empty-state">
                    <span className="text-3xl">💰</span>
                    <span className="empty-state-title">暂无返利数据</span>
                    <span className="empty-state-desc">配置凭据并同步后自动获取（FR-10）</span>
                  </div>
                )}
              </div>
            )}

            {/* 事件流 */}
            {tab === 'events' && (
              <div className="table-container rounded-t-none border-0">
                <table className="table">
                  <thead>
                    <tr>
                      <th>时间</th>
                      <th>级别</th>
                      <th>类型</th>
                      <th>消息</th>
                    </tr>
                  </thead>
                  <tbody>
                    {events.map((e) => (
                      <tr key={e.id}>
                        <td className="text-muted whitespace-nowrap text-xs" title={e.created_at}>
                          {timeDisplay(e.created_at)}
                        </td>
                        <td>
                          <span className={`badge ${eventLevelBadgeCls(e.level)}`}>{e.level}</span>
                        </td>
                        <td className="font-mono text-xs">{e.type}</td>
                        <td className="max-w-[380px]">
                          <span className="block truncate text-sm" title={e.message}>
                            {e.message}
                          </span>
                        </td>
                      </tr>
                    ))}
                    {events.length === 0 && (
                      <tr>
                        <td colSpan={4}>
                          <div className="empty-state">
                            <span className="text-3xl">📭</span>
                            <span className="empty-state-title">暂无事件</span>
                          </div>
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
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
                <span className={`badge ${auth.auth_state === 'ok' ? 'badge-success' : auth.auth_state === 'none' ? 'badge-muted' : 'badge-warning'}`}>
                  {auth.auth_state}
                </span>
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
              onClick={() =>
                run(
                  () =>
                    apiPutCredentials(id!, {
                      auth_mode: mode,
                      username,
                      password: mode === 'token' ? '' : password,
                      access_token: token,
                    }),
                  '凭据已保存',
                  'cred',
                )
              }
            >
              {busy === 'cred' ? '保存中…' : '保存凭据'}
            </button>
          </div>
        </div>
      </div>

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
              <select className="input" value={edit.site_type} onChange={(e) => setEdit({ ...edit, site_type: e.target.value })}>
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
        open={confirmDelete}
        title="删除站点"
        message={
          <>
            确定删除站点 <strong className="text-gray-900 dark:text-white">「{site.name}」</strong>
            吗？该站点的凭据、Keys、分组与返利数据将一并清除，此操作不可恢复。
          </>
        }
        busy={busy === 'delete'}
        onConfirm={doDelete}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  )
}
