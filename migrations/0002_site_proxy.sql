-- 0002_site_proxy.sql — sites 增加出站代理列（站点级覆盖，空则回落全局 proxy.url 配置）。

ALTER TABLE sites ADD COLUMN proxy_url TEXT NOT NULL DEFAULT '';
