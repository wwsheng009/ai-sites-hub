// 与 Go internal/model 对齐的类型投影
export interface Site {
  id: string
  name: string
  base_url: string
  /** 站点级出站代理（http/https/socks5；空=使用全局 proxy.url，两者皆空则直连） */
  proxy_url: string
  site_type: 'sub2api' | 'new-api' | 'unknown' | string
  status: string
  detect_score: number
  detect_report: string
  last_probe_at: string | null
  created_at: string
  updated_at: string
}

export interface SiteCredentialState {
  auth_mode: string
  username: string
  auth_state: string
  auth_state_msg: string
  last_login_at: string | null
  token_expire_hint: string | null
  updated_at: string
}

export interface SiteKey {
  id: string
  site_id: string
  remote_key_id: string
  name: string
  group: string
  status: string
  quota_limit: number | null
  quota_used: number | null
  quota_remaining: number | null
  unlimited: boolean
  key_source: string
}

export interface SiteGroup {
  id: string
  remote_group_id: string
  name: string
  ratio: number
  available: boolean
  desc: string
}

export interface SiteAffiliate {
  id: string
  site_id: string
  aff_code: string
  rebate_rate: number | null
  available: number | null
  frozen: number | null
  history: number | null
  invitee_count: number | null
  aff_cfg: string
  freshness: string
  last_sync_at: string | null
}

export interface AffRule {
  enabled: boolean
  min_amount: number
  max_per_transfer: number
  daily_limit: number
}

export interface AffTransfer {
  id: string
  site_id: string
  amount: number
  state: 'pending' | 'success' | 'failed' | 'dry_run' | string
  trigger_type: 'manual' | 'auto_rule' | string
  rule_snapshot: string
  amount_before: number | null
  error_class: string
  message: string
  created_at: string
  finished_at: string | null
}

export interface Event {
  id: string
  site_id: string
  type: string
  level: string
  message: string
  created_at: string
}

export interface DoctorCheck {
  name: string
  status: 'ok' | 'warn' | 'fail' | string
  detail: string
}

export interface DoctorReport {
  overall: string
  checked_at: string
  checks: DoctorCheck[]
}
