# ai-sites-hub 构建脚本（PowerShell 5.1+ / pwsh 通用）
#
# 用法（在仓库根目录执行）：
#   scripts/build.ps1                   # 后端单二进制（API-only，不依赖前端）
#   scripts/build.ps1 -WithWebUI        # npm build + 嵌入前端 → 单文件 dist/aiclient.exe
#   scripts/build.ps1 -WithWebUI -Version v0.1.0   # 注入版本号（version 命令可见）
#   scripts/build.ps1 -h                # 查看帮助（不执行构建）
#
# 产物：dist/aiclient.exe（-WithWebUI 时内含前端 SPA，部署只需该文件 + 可选 config.yaml）
param(
    [Alias("h")]
    [switch]$Help,
    [switch]$WithWebUI,
    [string]$Version = "",
    [string]$Out = "dist"
)

$ErrorActionPreference = "Stop"

if ($Help) {
    $usage = @"
ai-sites-hub 构建脚本（scripts/build.ps1）

用法:
  scripts/build.ps1 [选项]

选项:
  -h, -Help         显示本帮助并退出（不执行构建）
  -WithWebUI        构建前执行 npm run build，并把 frontend/dist 同步到
                    internal/webui/dist 后以 -tags webui_embed 嵌入二进制
  -Version <ver>    注入版本号（如 v0.1.0），同时把 buildInfo 标记为 release
  -Out <dir>        产物输出目录，默认 dist

产物:
  <Out>/aiclient.exe
    不带 -WithWebUI：API-only，不含前端资源，serve 仅提供 /api
    带 -WithWebUI  ：单文件二进制，内含前端 SPA（部署只需该文件 + 可选 config.yaml）

示例:
  scripts/build.ps1                                # 默认：API-only 构建
  scripts/build.ps1 -WithWebUI                     # 打包前端资源
  scripts/build.ps1 -WithWebUI -Version v0.1.0     # 打包前端 + 注入版本号
  scripts/build.ps1 -Out bin -Version v0.1.0       # 指定输出目录
  scripts/build.ps1 -h                             # 查看帮助

说明:
  -WithWebUI 流程：npm install（node_modules 缺失时）-> npm run build
  -> 同步 frontend/dist 到 internal/webui/dist -> go build -tags webui_embed
"@
    Write-Output $usage
    exit 0
}

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$ldflags = ""
if ($Version) {
    $ldflags = "-X aiclient/internal/cmd.version=$Version -X aiclient/internal/cmd.buildInfo=release"
}

New-Item -ItemType Directory -Force -Path $Out | Out-Null

if ($WithWebUI) {
    Write-Host "==> [1/4] 前端依赖安装（node_modules 缺失时）"
    Push-Location frontend
    try {
        if (-not (Test-Path node_modules)) {
            npm install --no-fund --no-audit
            if ($LASTEXITCODE -ne 0) { throw "npm install 失败" }
        }

        Write-Host "==> [2/4] 前端构建（tsc + vite）"
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "npm run build 失败" }
    } finally {
        Pop-Location
    }

    Write-Host "==> [3/4] 同步 frontend/dist -> internal/webui/dist"
    $sync = Join-Path $root "internal/webui/dist"
    Get-ChildItem $sync -Exclude README.md | Remove-Item -Recurse -Force
    Copy-Item (Join-Path $root "frontend\dist\*") $sync -Recurse -Force

    Write-Host "==> [4/4] go build -tags webui_embed（嵌入前端）"
    go build -tags webui_embed -ldflags $ldflags -o (Join-Path $Out "aiclient.exe") ./cmd/aiclient
    if ($LASTEXITCODE -ne 0) { throw "go build 失败" }
} else {
    Write-Host "==> go build（API-only，无前端依赖）"
    go build -ldflags $ldflags -o (Join-Path $Out "aiclient.exe") ./cmd/aiclient
    if ($LASTEXITCODE -ne 0) { throw "go build 失败" }
}

$size = (Get-Item (Join-Path $Out "aiclient.exe")).Length / 1MB
$ver = if ($Version) { $Version } else { "dev" }
Write-Host ("构建完成: {0}\aiclient.exe ({1:N1} MB, version={2}, webui={3})" -f $Out, $size, $ver, $(if ($WithWebUI) { "embedded" } else { "no" }))
