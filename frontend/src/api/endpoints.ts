// 按业务域组织的 API 封装（与 Go internal/httpx/router.go 对齐）
import { del, get, post, put } from './client'
import type { AffRule, AffTransfer, AffiliateInvitee, DoctorReport, Event, Site, SiteAffiliateOut, SiteAnnouncement, SiteCredentialState, SiteGroup, SiteKey, SiteModel, SyncState, SyncResult, UsageDaily, UsageLog } from '../types'

// ---- system ----
export const apiHealth = () => get<{ status: string; version: string }>('/health')
export const apiDoctor = () => get<DoctorReport>('/doctor')

// ---- sites（FR-1）----
export const apiListSites = (q?: { status?: string; site_type?: string; q?: string }) => {
  const p = new URLSearchParams()
  if (q?.status) p.set('status', q.status)
  if (q?.site_type) p.set('site_type', q.site_type)
  if (q?.q) p.set('q', q.q)
  const qs = p.toString()
  return get<Site[]>(`/sites${qs ? `?${qs}` : ''}`)
}
export const apiCreateSite = (input: { base_url: string; name?: string; proxy_url?: string; site_type?: string; detect_now?: boolean }) =>
  post<Site>('/sites', input)
export const apiGetSite = (id: string) => get<Site>(`/sites/${id}`)
export const apiUpdateSite = (
  id: string,
  input: { name?: string; status?: string; site_type?: string; proxy_url?: string | null; sync_cfg?: string },
) => put<Site>(`/sites/${id}`, input)
export const apiDeleteSite = (id: string) => del<{ deleted: boolean }>(`/sites/${id}`)
export const apiDetectSite = (id: string) => post<Site>(`/sites/${id}/detect`)

// ---- credentials（FR-2）----
export const apiPutCredentials = (
  id: string,
  input: { auth_mode: string; username?: string; password?: string; access_token?: string },
) => put<{ saved: boolean }>(`/sites/${id}/credentials`, input)
export const apiGetAuthState = (id: string) => get<SiteCredentialState>(`/sites/${id}/auth-state`)
export const apiAuthTest = (id: string) => post<SiteCredentialState>(`/sites/${id}/auth-test`)

// ---- sync（FR-4）----
export const apiSyncSite = (id: string) => post<SyncResult>(`/sites/${id}/sync`)
export const apiSyncAll = () => post<SyncResult[]>('/sync/all')

// ---- keys / groups（FR-4.2/4.4）----
export const apiListKeys = (siteId?: string) =>
  get<SiteKey[]>(`/keys${siteId ? `?site_id=${encodeURIComponent(siteId)}` : ''}`)
export const apiListGroups = (siteId?: string) =>
  get<SiteGroup[]>(`/groups${siteId ? `?site_id=${encodeURIComponent(siteId)}` : ''}`)

// ---- affiliates（FR-10）----
export const apiListAffiliates = () => get<SiteAffiliateOut[]>('/affiliates')
export const apiGetAffiliate = (siteId: string) => get<SiteAffiliateOut>(`/affiliates/site/${siteId}`)
export const apiGetAffiliateInvitees = (siteId: string) =>
  get<AffiliateInvitee[]>(`/affiliates/site/${siteId}/invitees`)
export const apiGetAffiliateRule = (siteId: string) => get<AffRule>(`/affiliates/site/${siteId}/rule`)
export const apiPutAffiliateRule = (siteId: string, rule: AffRule) => put<AffRule>(`/affiliates/site/${siteId}/rule`, rule)
export const apiTransferAffiliate = (siteId: string, input: { amount: number; dry_run: boolean }) =>
  post<AffTransfer>(`/affiliates/site/${siteId}/transfer`, input)
export const apiListTransfers = (siteId: string) => get<AffTransfer[]>(`/affiliates/site/${siteId}/transfers`)

// ---- events（FR-5.3）----
export const apiListEvents = (q?: { site_id?: string; limit?: number }) => {
  const p = new URLSearchParams()
  if (q?.site_id) p.set('site_id', q.site_id)
  if (q?.limit) p.set('limit', String(q.limit))
  const qs = p.toString()
  return get<Event[]>(`/events${qs ? `?${qs}` : ''}`)
}

// ---- announcements（FR-4.4；S5）----
export const apiListAnnouncements = (limit?: number) =>
  get<SiteAnnouncement[]>(`/announcements${limit ? `?limit=${limit}` : ''}`)

// ---- usage（S2/S3）----
export const apiListUsageLogs = (siteId: string, q?: { start?: string; end?: string; model?: string; limit?: number }) => {
  const p = new URLSearchParams()
  if (q?.start) p.set('start', q.start)
  if (q?.end) p.set('end', q.end)
  if (q?.model) p.set('model', q.model)
  if (q?.limit) p.set('limit', String(q.limit))
  const qs = p.toString()
  return get<UsageLog[]>(`/sites/${encodeURIComponent(siteId)}/usage/logs${qs ? `?${qs}` : ''}`)
}
export const apiListUsageDaily = (siteId: string, q?: { start?: string; end?: string; model?: string; limit?: number }) => {
  const p = new URLSearchParams()
  if (q?.start) p.set('start', q.start)
  if (q?.end) p.set('end', q.end)
  if (q?.model) p.set('model', q.model)
  if (q?.limit) p.set('limit', String(q.limit))
  const qs = p.toString()
  return get<UsageDaily[]>(`/sites/${encodeURIComponent(siteId)}/usage/daily${qs ? `?${qs}` : ''}`)
}

// ---- sync states（S1 作业页）----
export const apiListSyncStates = (siteId: string) =>
  get<SyncState[]>(`/sites/${encodeURIComponent(siteId)}/sync-states`)

// ---- models（S4）----
export const apiListSiteModels = (limit?: number) =>
  get<SiteModel[]>(`/models${limit ? `?limit=${limit}` : ''}`)
