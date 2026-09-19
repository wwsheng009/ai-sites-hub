-- 0003: site_affiliates 增加 invitees JSON 列（受邀用户列表投影）。
-- 参考 sub2api GET /api/v1/user/aff 的 AffiliateDetail.invitees：
-- [{user_id, email(上游已脱敏), username, created_at, total_rebate}]
ALTER TABLE site_affiliates ADD COLUMN invitees TEXT NOT NULL DEFAULT '[]';
