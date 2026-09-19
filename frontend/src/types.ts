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
  sync_cfg: string
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

/** AffiliateOut（含站点名便于跨站汇总） */
export interface SiteAffiliateOut extends SiteAffiliate {
  site_name: string
  site_url: string
  site_type: string
}

/** 受邀用户条目（参考 sub2api AffiliateDetail.invitees；email 上游已脱敏） */
export interface AffiliateInvitee {
  user_id: number
  email: string
  username: string
  created_at: string | null
  total_rebate: number
}

export interface AffRule {
  enabled: boolean
  min_amount: number
  max_per_transfer: number
  daily_limit: number
  last_transfer_date?: string
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
  site_id: string | null
  type: string
  level: string
  message: string
  data: string
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

export interface SyncResult {
  site_id?: string
  keys_synced?: number
  groups_synced?: number
  aff_synced?: boolean
  error?: string
  [k: string]: unknown
}

/** site_announcements 投影（S5；FR-4.4） */
export interface SiteAnnouncement {
  site_id: string
  site_name?: string
  title: string
  content: string
  published_at: string | null
}

/** usage_logs 投影（S2）：id 为本地行标识（后端另发 remote_ref，可用于与上游对账） */
export interface UsageLog {
  id: string
  model_name: string
  api_key_id: string
  api_key_mask: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  /** 缓存 token（Anthropic prompt caching）：读取 / 写入 / 5m / 1h */
  cache_read_tokens: number
  cache_creation_tokens: number
  cache_creation_5m_tokens: number
  cache_creation_1h_tokens: number
  /** 费用分解（站点币，仅标明不折算） */
  input_cost: number
  output_cost: number
  cache_creation_cost: number
  cache_read_cost: number
  /** 原价（未打折） */
  total_cost: number
  /** 实付 */
  amount: number
  /** 计费倍率（1 = 无折扣） */
  rate_multiplier: number
  currency: string
  status: string
  err_code: string
  /** 请求形态：stream / sync / ws_v2 / live / cyber */
  request_type: string
  stream: boolean
  billing_mode: string
  service_tier: string
  reasoning_effort: string
  inbound_endpoint: string
  group_id: string
  /** 首字延迟 / 总耗时（毫秒；null = 上游未记录） */
  first_token_ms: number | null
  duration_ms: number | null
  ts: string
  /** 账号归属（上游标识；未知为 ""，不是 null） */
  user_id: string
  account_id: string
  subscription_id: string
  /** 请求形态（上游归一化后的实际路由与开关；boolean 后端已把 0/1 转为 bool，false 为有效值） */
  upstream_endpoint: string
  long_context_billing_applied: boolean
  /** 计费类型枚举（0 = 未知） */
  billing_type: number
  openai_ws_mode: boolean
  native_compaction_v2: boolean
  cache_ttl_overridden: boolean
  /** 图片计费（无数据为 0 / ""；image_size_breakdown 无数据为 null） */
  image_count: number
  image_size: string
  image_input_size: string
  image_output_size: string
  image_input_tokens: number
  image_input_cost: number
  image_output_tokens: number
  image_output_cost: number
  image_size_source: string
  image_size_breakdown: Record<string, number> | null
  media_type: string
  /** 客户端信息（未知为 ""） */
  user_agent: string
  ip_address: string
  session_id: string
}

/** usage_daily 聚合（S3） */
export interface UsageDaily {
  id: string
  site_id: string
  day: string
  model_name: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  amount: number
  currency: string
  request_count: number
}

/** models 广场（S4） */
export interface SiteModel {
  site_id: string
  site_name: string
  model_name: string
  total_tokens: number
  amount: number
  currency: string
  balance: number | null
  freshness: string
}

/** site_sync_state 域级状态（S1 作业页） */
export interface SyncState {
  id: string
  site_id: string
  domain: string
  enabled: number
  interval_s: number
  last_run_at: string | null
  next_run_at: string | null
  cursor: string
  fingerprint: string
  consecutive_failures: number
  last_error_class: string
}
