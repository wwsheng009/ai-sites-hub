# 前端产物同步目录

本目录内容由 `scripts/build.ps1 -WithWebUI` 从 `frontend/dist` 同步生成（先清空再复制），
随 `-tags webui_embed` 构建嵌入二进制。除本 README 外的内容不入库（见 .gitignore）。

直接执行 `go build -tags webui_embed` 而不先跑脚本时，只会嵌入本 README，
运行时访问非 /api 路径将得到 404 —— 请使用构建脚本。
