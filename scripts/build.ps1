# ai-sites-client 构建脚本（PowerShell 5.1+ / pwsh 通用）
#
# 用法（在仓库根目录执行）：
#   scripts/build.ps1                   # 后端单二进制（API-only，不依赖前端）
#   scripts/build.ps1 -WithWebUI        # npm build + 嵌入前端 → 单文件 dist/aiclient.exe
#   scripts/build.ps1 -WithWebUI -Version v0.1.0   # 注入版本号（version 命令可见）
#
# 产物：dist/aiclient.exe（-WithWebUI 时内含前端 SPA，部署只需该文件 + 可选 config.yaml）
param(
    [switch]$WithWebUI,
    [string]$Version = "",
    [string]$Out = "dist"
)

$ErrorActionPreference = "Stop"
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
