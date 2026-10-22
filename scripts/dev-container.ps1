<#
.SYNOPSIS
  Windows wrapper for scripts/dev-container.sh — runs it inside WSL (D113:
  every docker command goes to the WSL daemon, never to the Windows docker CLI).
  The container mounts the course repo root (sms-platform/../../..) at
  /workspace/<root dir name> (T0194 / D116 ①); test/shell run in sms-platform/.

.EXAMPLE
  .\scripts\dev-container.ps1 up        # build if needed, create + start, show port/health
  .\scripts\dev-container.ps1 test      # go test ./... -race inside the container
  .\scripts\dev-container.ps1 down
  .\scripts\dev-container.ps1 up -Distro Ubuntu-24.04
#>
param(
  [ValidateSet('build', 'up', 'start', 'stop', 'down', 'status', 'shell', 'logs', 'test')]
  [string]$Cmd = 'up',
  [string]$Distro = ''
)

$ErrorActionPreference = 'Stop'
$smsPlatform = Split-Path -Parent $PSScriptRoot

$wslArgs = @()
if ($Distro) { $wslArgs += @('-d', $Distro) }

# Windows path -> WSL path (D:\x\y -> /mnt/d/x/y)
$wslPath = (& wsl @wslArgs -e wslpath -a $smsPlatform).Trim()

# The script is read with CRLF stripped so a checkout with core.autocrlf=true
# still runs; .gitattributes keeps it LF anyway.
$inner = "cd '$wslPath' && SMS_PLATFORM_DIR='$wslPath' bash <(tr -d '\r' < scripts/dev-container.sh) $Cmd"
& wsl @wslArgs -e bash -lc $inner
exit $LASTEXITCODE
