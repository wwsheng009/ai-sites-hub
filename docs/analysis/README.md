# 参考项目分析文档目录

本目录存放对三个参考项目的勘察分析文档，是本项目对接外部站点 API 的**契约依据**。

## 文档清单

| 文档 | 被分析项目 | 分析重点 |
| ---- | ---------- | -------- |
| `sub2api-analysis.md` | `E:\projects\ai\sub2api` | API 接口清单（登录/签到/token/分组/额度）、后端架构、前端骨架可复用清单 |
| `new-api-analysis.md` | `E:\projects\ai\new-api` | 标准 API 清单、认证机制（session/access_token）、定时任务、部署形态 |
| `ai-gateway-analysis.md` | `E:\projects\ai\ai-gateway` | 已有的站点聚合管理设计、cmd 命令入口、internal 分层、可复用模式与坑 |

## 结论去向

- 各站点可调用的 API 能力 → 汇总进 `../architecture/` 的适配层设计
- sub2api 前端骨架可复用文件清单 → 直接指导前端工程的初始化
- ai-gateway 的分层与作业模式 → 指导本项目 Go 服务端骨架
