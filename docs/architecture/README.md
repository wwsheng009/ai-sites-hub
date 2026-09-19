# 架构设计文档目录

本目录存放系统架构设计文档。

## 文档清单

| 文档 | 内容 |
| ---- | ---- |
| `architecture.md` | 总体架构：组件图、Go 服务端分层、站点适配层（sub2api/new-api 两类 adapter）、后台作业调度、存储模型、API 概览、前端与 cmd 的接入方式 |

## 设计基线

- 单体 Go 服务，内置 HTTP API + 静态托管前端 + 内置后台作业调度器
- cmd 子命令与 HTTP API 共用同一套 service 层
- 站点类型差异收敛到 adapter 接口后面
