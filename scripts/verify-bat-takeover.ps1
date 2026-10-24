<#
.SYNOPSIS
  One-shot automated acceptance of the BAT Docker wizard "Use existing" takeover of the
  sms-platform dev container, without the GUI and without touching the user's docker
  context, DOCKER_HOST or BAT profiles.

.DESCRIPTION
  1. Snapshots the environment (docker context, $env:DOCKER_HOST, BAT profiles index hash,
     container health).
  2. Starts the WSL socat relay (scripts/wsl-docker-relay.sh start → 127.0.0.1:2375) unless it
     is already running.
  3. Runs scripts/verify-bat-takeover.mjs (V1–V5 + the write-profile dry run), which passes
     DOCKER_HOST only to its own docker child processes.
  4. In `finally`: stops the relay (unless it was already running and -KeepRelay, or -KeepRelay),
     then runs V6: no listener on 2375, docker context / DOCKER_HOST unchanged, BAT profiles
     index unchanged, container still healthy.

  The host folder and the BAT source checkout (for node_modules/ws) are resolved by
  verify-bat-takeover.mjs: SMS_REPO_ROOT / SMS_BAT_SRC from the environment or from .env
  next to compose.yaml; -BatSrc overrides SMS_BAT_SRC.

  Exit code: 0 = V1–V6 all PASS; 1 = a check failed; 2 = usage / environment error.
  Secrets (server token, certificate fingerprint) are never printed in full.

.EXAMPLE
  .\scripts\verify-bat-takeover.ps1
  .\scripts\verify-bat-takeover.ps1 -Json "$env:TEMP\bat-takeover.json"
  .\scripts\verify-bat-takeover.ps1 -BatSrc <BAT source checkout>
  .\scripts\verify-bat-takeover.ps1 -Distro Ubuntu-24.04 -Container bat-server-sms-platform
#>
param(
  [string]$Container = 'bat-server-sms-platform',
  [string]$Distro = '',
  [string]$Json = '',
  [string]$NodeExe = '',
  [string]$BatSrc = '',
  [int]$RelayPort = 2375,
  [switch]$KeepRelay,
  [switch]$SkipPty
)

$ErrorActionPreference = 'Stop'
$smsPlatform = Split-Path -Parent $PSScriptRoot
$mjs = Join-Path $PSScriptRoot 'verify-bat-takeover.mjs'
if (-not (Test-Path $mjs)) { Write-Error "missing $mjs"; exit 2 }

$node = if ($NodeExe) { $NodeExe } elseif ($env:BAT_HELPER_NODE) { $env:BAT_HELPER_NODE } else { 'node' }
$wslArgs = @()
if ($Distro) { $wslArgs += @('-d', $Distro) }

function Invoke-Wsl([string]$Command) {
  # -lc: login shell so ~/.wsl-docker-relay.pid resolves against the user's $HOME
  $out = & wsl @wslArgs -e bash -lc $Command 2>&1
  return [pscustomobject]@{ Output = ($out | Out-String).Trim(); ExitCode = $LASTEXITCODE }
}

# The relay script is read with CRLF stripped so a checkout with core.autocrlf=true still runs.
$wslPath = (& wsl @wslArgs -e wslpath -a $smsPlatform).Trim()
$relay = "bash <(tr -d '\r' < '$wslPath/scripts/wsl-docker-relay.sh')"

# ── snapshot (V6 baseline) ──────────────────────────────────────────────────────
$ctxBefore = (& docker context show 2>$null | Out-String).Trim()
$envBefore = $env:DOCKER_HOST
$userData = if ($env:BAT_USER_DATA) { $env:BAT_USER_DATA } else { Join-Path $env:APPDATA 'BetterAgentTerminal' }
$indexPath = Join-Path $userData 'profiles\index.json'
$indexBefore = if (Test-Path $indexPath) { (Get-FileHash -Algorithm SHA256 $indexPath).Hash } else { '(absent)' }
$healthBefore = (Invoke-Wsl "docker inspect --format '{{.State.Health.Status}}' $Container").Output

Write-Host "verify-bat-takeover.ps1 — container $Container, relay 127.0.0.1:$RelayPort"
Write-Host "  before: docker context=$ctxBefore DOCKER_HOST=$(if ($envBefore) { $envBefore } else { '(unset)' }) profiles-index=$($indexBefore.Substring(0, [Math]::Min(12, $indexBefore.Length)))… container=$healthBefore"

# ── relay ───────────────────────────────────────────────────────────────────────
$status = Invoke-Wsl "$relay status"
$relayWasRunning = ($status.ExitCode -eq 0)
if ($relayWasRunning) {
  Write-Host "  relay: already running ($($status.Output))"
} else {
  $start = Invoke-Wsl "PORT=$RelayPort $relay start"
  if ($start.ExitCode -ne 0) { Write-Error "relay start failed: $($start.Output)"; exit 2 }
  Write-Host "  relay: started ($($start.Output -replace "`n", ' / '))"
}

$nodeExit = 2
$v6 = @()
try {
  $nodeArgs = @($mjs, '--container', $Container, '--docker-host', "tcp://127.0.0.1:$RelayPort")
  if ($Json) { $nodeArgs += @('--json', $Json) }
  if ($SkipPty) { $nodeArgs += '--skip-pty' }
  if ($BatSrc) { $nodeArgs += @('--bat-src', $BatSrc) }
  & $node @nodeArgs
  $nodeExit = $LASTEXITCODE
} finally {
  # ── cleanup + V6 ──────────────────────────────────────────────────────────────
  if ($KeepRelay) {
    Write-Host "  relay: kept running (-KeepRelay)"
  } else {
    $stop = Invoke-Wsl "$relay stop"
    Write-Host "  relay: stop → $($stop.Output)"
  }
  $listen = (Invoke-Wsl "ss -ltn | grep -c ':$RelayPort ' || true").Output
  $ctxAfter = (& docker context show 2>$null | Out-String).Trim()
  $envAfter = $env:DOCKER_HOST
  $indexAfter = if (Test-Path $indexPath) { (Get-FileHash -Algorithm SHA256 $indexPath).Hash } else { '(absent)' }
  $healthAfter = (Invoke-Wsl "docker inspect --format '{{.State.Health.Status}}' $Container").Output

  $v6 = @(
    [pscustomobject]@{ Check = 'relay stopped (no listener on 2375)'; Pass = ($KeepRelay -or $listen -eq '0'); Evidence = "ss -ltn matches ':$RelayPort ' = $listen$(if ($KeepRelay) { ' (kept by request)' })" },
    [pscustomobject]@{ Check = 'docker context unchanged'; Pass = ($ctxBefore -eq $ctxAfter); Evidence = "$ctxBefore → $ctxAfter" },
    [pscustomobject]@{ Check = 'DOCKER_HOST unchanged'; Pass = ($envBefore -eq $envAfter); Evidence = "$(if ($envBefore) { $envBefore } else { '(unset)' }) → $(if ($envAfter) { $envAfter } else { '(unset)' })" },
    [pscustomobject]@{ Check = 'BAT profiles index.json unchanged'; Pass = ($indexBefore -eq $indexAfter); Evidence = "sha256 $($indexBefore.Substring(0, [Math]::Min(12, $indexBefore.Length)))… → $($indexAfter.Substring(0, [Math]::Min(12, $indexAfter.Length)))…" },
    [pscustomobject]@{ Check = 'container still healthy'; Pass = ($healthAfter -eq 'healthy' -or $healthAfter -eq 'none'); Evidence = "$healthBefore → $healthAfter" }
  )
  $v6Fail = @($v6 | Where-Object { -not $_.Pass }).Count
  Write-Host ("{0} V6 cleanup — {1}" -f $(if ($v6Fail -eq 0) { '✅' } else { '❌' }), $(if ($v6Fail -eq 0) { 'PASS' } else { 'FAIL' }))
  foreach ($c in $v6) { Write-Host ("     {0} {1} → {2}" -f $(if ($c.Pass) { 'ok ' } else { 'NOK' }), $c.Check, $c.Evidence) }
}

$exit = if ($nodeExit -eq 0 -and $v6Fail -eq 0) { 0 } elseif ($nodeExit -eq 2) { 2 } else { 1 }
Write-Host "exit $exit (V1–V5 exit $nodeExit, V6 failures $v6Fail)"
exit $exit
