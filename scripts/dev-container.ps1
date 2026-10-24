<#
.SYNOPSIS
  Windows wrapper for scripts/dev-container.sh — runs it inside WSL (every
  docker command goes to the WSL daemon, never to the Windows docker CLI).
  The container mounts the git repo root that contains sms-platform/ at
  /workspace/<root dir name>; test/shell run in sms-platform/.

.EXAMPLE
  .\scripts\dev-container.ps1 up        # build if needed, create + start, show port/health
  .\scripts\dev-container.ps1 test      # go test ./... -race inside the container
  .\scripts\dev-container.ps1 env       # SMS_* values for compose.yaml (this layout)
  .\scripts\dev-container.ps1 compose config
  .\scripts\dev-container.ps1 down
  .\scripts\dev-container.ps1 up -Distro Ubuntu-24.04
#>
param(
  [ValidateSet('build', 'up', 'start', 'stop', 'down', 'status', 'shell', 'logs', 'test', 'env', 'compose')]
  [string]$Cmd = 'up',
  [string]$Distro = '',
  # compose only: the docker compose arguments (e.g. config, up -d dev)
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$Rest = @()
)

$ErrorActionPreference = 'Stop'
$smsPlatform = Split-Path -Parent $PSScriptRoot

$wslArgs = @()
if ($Distro) { $wslArgs += @('-d', $Distro) }

# Windows path -> WSL path (D:\x\y -> /mnt/d/x/y)
$wslPath = (& wsl @wslArgs -e wslpath -a $smsPlatform).Trim()

# The script is read with CRLF stripped so a checkout with core.autocrlf=true
# still runs; .gitattributes keeps it LF anyway.
$restArgs = ($Rest | ForEach-Object { "'" + ($_ -replace "'", "'\''") + "'" }) -join ' '
$inner = "cd '$wslPath' && SMS_PLATFORM_DIR='$wslPath' bash <(tr -d '\r' < scripts/dev-container.sh) $Cmd $restArgs"
& wsl @wslArgs -e bash -lc $inner
exit $LASTEXITCODE
