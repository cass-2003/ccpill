# ccpill one-click installer (Windows PowerShell 5.1+)
# Usage:  irm https://raw.githubusercontent.com/cass-2003/ccpill/main/scripts/install.ps1 | iex
# Flow:   GitHub Releases prebuilt binary -> fallback to `go install` -> write Claude Code settings.json
# NOTE: keep this file pure ASCII - PS 5.1 misreads BOM-less UTF-8 as ANSI.
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'cass-2003/ccpill'
$claudeDir = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME '.claude' }
$binDir = Join-Path $claudeDir 'ccpill\bin'
$exe = Join-Path $binDir 'ccpill.exe'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$installed = $false

# 1) Prebuilt binary from GitHub Releases (available since V0.3; skip silently if none)
try {
    $rel = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    $asset = $rel.assets | Where-Object { $_.name -eq 'ccpill-windows-amd64.exe' } | Select-Object -First 1
    if ($asset) {
        Write-Host "Downloading prebuilt binary $($rel.tag_name) ..."
        Invoke-WebRequest $asset.browser_download_url -OutFile $exe -UseBasicParsing
        $installed = $true
    }
} catch { }

# 2) Local Go toolchain: install straight from source, no clone needed
if (-not $installed) {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host 'No prebuilt release yet, building from source via go install ...'
        $env:GOBIN = $binDir
        go install "github.com/$repo@latest"
        if ($LASTEXITCODE -ne 0) { throw 'go install failed - check network / GOPROXY and retry' }
        $installed = $true
    }
}

if (-not $installed) {
    throw 'No prebuilt release and no local Go. Install Go first (https://go.dev/dl/) then rerun this script.'
}

# 3) Write Claude Code settings.json (ccpill backs up and writes atomically)
& $exe --install
if ($LASTEXITCODE -ne 0) { throw 'ccpill --install failed' }

# 4) Add bin dir to user PATH so `ccpill --config` works from any new terminal
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $binDir), 'User')
    Write-Host "Added to user PATH: $binDir"
    Write-Host "Open a NEW terminal, then run:  ccpill --config"
} else {
    Write-Host 'Web config center:  ccpill --config'
}
