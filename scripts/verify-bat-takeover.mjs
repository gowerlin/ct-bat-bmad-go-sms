#!/usr/bin/env node
/**
 * verify-bat-takeover.mjs — automated acceptance for "BAT Docker wizard →
 * Use existing" against the sms-platform dev container (Docker inside WSL, reached
 * from Windows through scripts/wsl-docker-relay.sh).
 *
 * BAT has no CLI / Remote API that creates or tests a Docker profile: `docker:*` and
 * `profile:create` / `profile:update` are `ipcMain.handle` channels for the renderer only
 * (electron/main.ts), none of them is in PROXIED_CHANNELS (electron/remote/protocol.ts), and a
 * helper capability (`bat-terminal.mjs`) is limited to `terminal:*` / `question:*`
 * (electron/remote/helper-capability.ts). So this script replays what the wizard's backend does
 * (src/components/setup-wizard/docker-flow.ts and its steps) without the GUI:
 *
 *   detect-env            docker --version && docker info                (electron/docker-detect.ts dockerStatus)
 *   pick-container        docker ps -a --format '{{json .}}' → first entry (steps/docker/pick-container.ts)
 *   configure-mounts      host folder → /workspace/<basename>; validateMountTable (electron/docker-validate.ts)
 *   install-server-bundle docker inspect <name> → image non-empty       (steps/docker/install-server-bundle.ts)
 *   start-server          pickDockerServerPort (published 9876/tcp, ≠ BAT's own port), detectContainerExposure,
 *                         docker start, docker exec cat server-token.json, health (electron/docker-lifecycle.ts)
 *   fetch-fingerprint     TLS handshake on 127.0.0.1:<port> → fingerprint256 (electron/tls-fingerprint.ts)
 *   connect-test          WSS + pinned fingerprint + `auth` frame with the token (electron/remote/remote-client.ts)
 *   write-profile         the ProfileEntry the wizard would write — DRY RUN only, BAT's index.json is never touched
 *   open profile → tab    `pty:create` over the same authenticated socket, commands typed with `pty:write`,
 *                         output read from `pty:output` events (never `docker exec`)
 *
 * Secrets: the token and the fingerprint are read and compared but never printed in full
 * (length and the first 4 hex characters only). A wrong-token probe (V4 negative) is sent
 * BEFORE the real auth: the server counts failed server-token auths per IP (5 within 60 s ⇒
 * 10-minute ban, electron/remote/remote-server.ts), and a successful auth clears that counter.
 *
 * Dependencies: only `ws` from a BAT source checkout's node_modules (--bat-src or SMS_BAT_SRC);
 * no global install. Node ≥ 20.
 *
 * Layout: the host folder defaults to the git repo root that contains sms-platform/ (the
 * public repo: sms-platform itself), the same folder compose.yaml / scripts/dev-container.sh
 * mount. SMS_REPO_ROOT and SMS_BAT_SRC come from the environment or from .env next to
 * compose.yaml (the environment wins); relative paths there are relative to sms-platform/.
 *
 * Exit code: 0 = every check PASS; 1 = at least one FAIL; 2 = usage / environment error.
 */
import { execFile, execFileSync } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import path from 'node:path'
import * as tls from 'node:tls'
import { fileURLToPath } from 'node:url'
import { parseArgs } from 'node:util'

const here = path.dirname(fileURLToPath(import.meta.url))
const smsPlatform = path.resolve(here, '..')

/** SMS_<key> from the environment, else from sms-platform/.env (same rule as dev-container.sh). */
function smsSetting(key) {
  if (process.env[key]) return process.env[key]
  let text
  try { text = readFileSync(path.join(smsPlatform, '.env'), 'utf8') } catch { return undefined }
  for (const raw of text.split('\n')) {
    const line = raw.replace(/\r$/, '')
    if (line.startsWith(`${key}=`)) return line.slice(key.length + 1).replace(/^(["'])(.*)\1$/, '$2') || undefined
  }
  return undefined
}

/** The folder compose.yaml mounts: SMS_REPO_ROOT, else the git repo root holding sms-platform/. */
function defaultHostFolder() {
  const configured = smsSetting('SMS_REPO_ROOT')
  if (configured) return path.resolve(smsPlatform, configured)
  try {
    const top = execFileSync('git', ['-C', smsPlatform, 'rev-parse', '--show-toplevel'], {
      encoding: 'utf8', windowsHide: true, stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
    if (top) return path.resolve(top)
  } catch { /* not a git checkout: sms-platform is the root */ }
  return smsPlatform
}

/** Container working_dir: /workspace/<host folder name>/<sms-platform relative to it>. */
function defaultExpectCwd(hostFolder) {
  const rel = path.relative(hostFolder, smsPlatform).split(path.sep).filter(Boolean).join('/')
  return `/workspace/${basename(hostFolder)}${rel ? `/${rel}` : ''}`
}

const DEFAULTS = {
  container: 'bat-server-sms-platform',
  dockerHost: 'tcp://127.0.0.1:2375',
  expectGo: '1.27.3',
  expectSkills: '8',
  timeout: '15000',
}
const SERVER_CONTAINER_PORT_KEY = '9876/tcp'
const BAT_SERVER_BIN = '/opt/bat-server/bin/bat-server'
const TOKEN_FILE = '/root/.local/share/bat-server/server-token.json'
const CERT_FILE = '/root/.local/share/bat-server/server-cert.json'
const CONTAINER_NAME_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_.-]+$/
const FINGERPRINT_RE = /^[0-9A-F]{2}(:[0-9A-F]{2}){31}$/
const ANSI_RX = /\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[()][0-9A-Za-z]|\x1b[=>]/g

const USAGE = `Usage: node verify-bat-takeover.mjs [options]
  --container <name>    Docker container (default ${DEFAULTS.container})
  --docker-host <url>   DOCKER_HOST for every docker call (default ${DEFAULTS.dockerHost}; the WSL relay)
  --host-folder <path>  the folder the wizard would be given (default: SMS_REPO_ROOT, else the git
                        repo root containing sms-platform/)
  --expect-cwd <path>   expected pwd of a tab started in the container working_dir
                        (default /workspace/<host folder name>/<sms-platform relative to it>)
  --expect-go <ver>     expected Go version (default ${DEFAULTS.expectGo})
  --expect-skills <n>   expected number of entries in ~/.claude/skills (default ${DEFAULTS.expectSkills})
  --bat-port <n>        BAT's own RemoteServer port to refuse (default $BAT_REMOTE_PORT or 9876)
  --bat-src <dir>       BAT source checkout providing node_modules/ws (default SMS_BAT_SRC; required)
  --json <file>         write the results (secrets redacted) as JSON
  --timeout <ms>        per-step timeout (default ${DEFAULTS.timeout})
  --skip-pty            skip V5 (no PTY is opened)
  -h, --help`

class UsageError extends Error {}

function parseCli(argv) {
  const { values } = parseArgs({
    args: argv,
    options: {
      container: { type: 'string' }, 'docker-host': { type: 'string' }, 'host-folder': { type: 'string' },
      'expect-cwd': { type: 'string' }, 'expect-go': { type: 'string' }, 'expect-skills': { type: 'string' },
      'bat-port': { type: 'string' }, 'bat-src': { type: 'string' }, json: { type: 'string' },
      timeout: { type: 'string' }, 'skip-pty': { type: 'boolean' }, help: { type: 'boolean', short: 'h' },
    },
    strict: true,
  })
  if (values.help) { console.log(USAGE); process.exit(0) }
  const hostFolder = path.resolve(values['host-folder'] ?? defaultHostFolder())
  const batSrc = values['bat-src'] ?? (smsSetting('SMS_BAT_SRC') && path.resolve(smsPlatform, smsSetting('SMS_BAT_SRC')))
  if (!batSrc) throw new UsageError('no BAT source checkout: pass --bat-src <dir> or set SMS_BAT_SRC (environment or .env); it must hold node_modules/ws')
  const opts = {
    container: values.container ?? DEFAULTS.container,
    dockerHost: values['docker-host'] ?? DEFAULTS.dockerHost,
    hostFolder,
    expectCwd: values['expect-cwd'] ?? defaultExpectCwd(hostFolder),
    expectGo: values['expect-go'] ?? DEFAULTS.expectGo,
    expectSkills: Number(values['expect-skills'] ?? DEFAULTS.expectSkills),
    batPort: Number(values['bat-port'] ?? process.env.BAT_REMOTE_PORT ?? 9876),
    batSrc: path.resolve(batSrc),
    json: values.json,
    timeout: Number(values.timeout ?? DEFAULTS.timeout),
    skipPty: values['skip-pty'] === true,
  }
  if (!CONTAINER_NAME_PATTERN.test(opts.container)) throw new UsageError(`invalid container name: ${opts.container}`)
  if (!Number.isInteger(opts.timeout) || opts.timeout < 1000) throw new UsageError('--timeout must be an integer ≥ 1000')
  return opts
}

// ---------------------------------------------------------------------------
// docker — same CLI, same args as BAT (execFile('docker', …), DOCKER_HOST inherited)
// ---------------------------------------------------------------------------
function execDocker(args, { dockerHost, allowFailure = false, timeoutMs = 20_000 } = {}) {
  return new Promise((resolve, reject) => {
    execFile('docker', args, {
      encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, windowsHide: true, timeout: timeoutMs,
      env: { ...process.env, DOCKER_HOST: dockerHost },
    }, (error, stdout, stderr) => {
      if (error && !allowFailure) {
        reject(new Error(`docker ${args.slice(0, 2).join(' ')}: ${(stderr || stdout || error.message).trim().split('\n')[0]}`))
        return
      }
      resolve({ stdout: (stdout ?? '').trim(), stderr: (stderr ?? '').trim(), code: error ? (error.code ?? 1) : 0 })
    })
  })
}

/** electron/docker-lifecycle.ts parsePersistedToken */
function parsePersistedToken(raw) {
  const trimmed = raw.trim()
  if (!trimmed) return null
  try {
    const parsed = JSON.parse(trimmed)
    if (typeof parsed.token === 'string') return parsed.token
    if (parsed.encrypted === false && typeof parsed.data === 'string') return parsed.data
  } catch {
    return trimmed
  }
  return null
}

function isLoopbackHostIp(hostIp) {
  const ip = String(hostIp ?? '').trim().toLowerCase()
  return ip === 'localhost' || ip === '::1' || ip === '[::1]' || /^127(?:\.\d{1,3}){3}$/.test(ip)
}

/** electron/docker-lifecycle.ts publishedServerPort */
function publishedServerPort(entry) {
  for (const source of [entry?.NetworkSettings?.Ports, entry?.HostConfig?.PortBindings]) {
    const list = source && typeof source === 'object' ? source[SERVER_CONTAINER_PORT_KEY] : null
    if (!Array.isArray(list)) continue
    for (const binding of list) {
      const port = Number(binding?.HostPort)
      if (Number.isInteger(port) && port > 0 && port <= 65535) return port
    }
  }
  return null
}

function basename(p) {
  const parts = p.split(/[\\/]/).filter(Boolean)
  return parts[parts.length - 1] || 'workspace'
}

function normalizeFingerprint(raw) {
  const hex = String(raw ?? '').replace(/[^0-9A-Fa-f]/g, '').toUpperCase()
  return hex.match(/.{2}/g)?.join(':') ?? ''
}
const redactFp = (fp) => (fp ? `${fp.slice(0, 5)}…(${fp.replace(/:/g, '').length} hex)` : '(none)')
const redactToken = (t) => (t ? `len=${t.length}, starts ${t.slice(0, 4)}…` : '(none)')

// ---------------------------------------------------------------------------
// TLS fingerprint — electron/tls-fingerprint.ts fetchTlsFingerprint
// ---------------------------------------------------------------------------
function fetchTlsFingerprint(port, host = '127.0.0.1', timeoutMs = 5000) {
  return new Promise((resolve) => {
    let settled = false
    let socket = null
    const finish = (result) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      socket?.destroy()
      resolve(result)
    }
    const timer = setTimeout(() => finish({ ok: false, error: `TLS handshake with ${host}:${port} did not complete within ${timeoutMs}ms` }), timeoutMs)
    try {
      socket = tls.connect({ host, port, rejectUnauthorized: false })
    } catch (error) {
      finish({ ok: false, error: error.message })
      return
    }
    socket.on('error', (error) => finish({ ok: false, error: `${error.code ?? ''} ${error.message}`.trim() }))
    socket.once('close', () => finish({ ok: false, error: 'connection closed before the TLS handshake completed' }))
    socket.once('secureConnect', () => {
      const fp = String(socket.getPeerCertificate()?.fingerprint256 ?? '').toUpperCase()
      if (!FINGERPRINT_RE.test(fp)) { finish({ ok: false, error: 'server presented no certificate' }); return }
      finish({ ok: true, fingerprint: fp, protocol: socket.getProtocol() })
    })
  })
}

// ---------------------------------------------------------------------------
// Minimal remote client — electron/remote/remote-client.ts connect(): pin on `upgrade`,
// then `{type:'auth', id, token, args:[label]}`; invoke / event frames per protocol.ts
// ---------------------------------------------------------------------------
class RemoteClient {
  constructor({ WebSocket, url, token, fingerprint, timeoutMs, label }) {
    this.WebSocket = WebSocket
    this.url = url
    this.token = token
    this.expected = normalizeFingerprint(fingerprint)
    this.timeoutMs = timeoutMs
    this.label = label
    this.ws = null
    this.seq = 0
    this.pending = new Map()
    this.listeners = new Set()
  }
  nextId() { this.seq += 1; return `t0196-${this.seq}` }
  onEvent(fn) { this.listeners.add(fn); return () => this.listeners.delete(fn) }
  get isOpen() { return this.ws !== null && this.ws.readyState === this.WebSocket.OPEN }

  /** Resolves { metadata } on auth success; rejects with code 'fingerprint-mismatch' | 'auth-failed' | 'closed' | 'timeout'. */
  connect() {
    return new Promise((resolve, reject) => {
      const ws = new this.WebSocket(this.url, { rejectUnauthorized: false, handshakeTimeout: this.timeoutMs })
      this.ws = ws
      let settled = false
      const settle = (error, value) => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        if (error) { try { ws.terminate() } catch { /* gone */ } reject(error) } else resolve(value)
      }
      const timer = setTimeout(() => settle(Object.assign(new Error(`connect timeout after ${this.timeoutMs}ms`), { code: 'timeout' })), this.timeoutMs)
      ws.on('upgrade', (res) => {
        const actual = normalizeFingerprint(res?.socket?.getPeerCertificate?.(false)?.fingerprint256)
        if (!actual || actual !== this.expected) {
          settle(Object.assign(new Error(`fingerprint-mismatch: expected ${redactFp(this.expected)}, got ${redactFp(actual)}`), { code: 'fingerprint-mismatch' }))
        }
      })
      ws.on('open', () => {
        if (settled) return
        ws.send(JSON.stringify({ type: 'auth', id: this.nextId(), token: this.token, args: [this.label] }))
      })
      ws.on('message', (raw) => {
        let frame
        try { frame = JSON.parse(raw.toString()) } catch { return }
        if (frame.type === 'auth-result') {
          if (frame.error) settle(Object.assign(new Error(frame.error), { code: 'auth-failed' }))
          else settle(null, { metadata: frame.result })
          return
        }
        if (frame.type === 'invoke-result' || frame.type === 'invoke-error') {
          const entry = this.pending.get(frame.id)
          if (!entry) return
          this.pending.delete(frame.id)
          clearTimeout(entry.timer)
          if (frame.type === 'invoke-error') entry.reject(new Error(frame.error || 'remote invoke failed'))
          else entry.resolve(frame.result)
          return
        }
        if (frame.type === 'ping') { try { ws.send(JSON.stringify({ type: 'pong', id: frame.id })) } catch { /* ignore */ } return }
        if (frame.type === 'event' && typeof frame.channel === 'string') {
          for (const fn of this.listeners) fn(frame.channel, Array.isArray(frame.args) ? frame.args : [])
        }
      })
      ws.on('close', (code) => {
        for (const [id, entry] of this.pending) { clearTimeout(entry.timer); entry.reject(new Error(`connection closed (code ${code})`)); this.pending.delete(id) }
        settle(Object.assign(new Error(`connection closed before auth (code ${code})`), { code: 'closed' }))
      })
      ws.on('error', (error) => settle(Object.assign(error, { code: error.code ?? 'error' })))
    })
  }

  invoke(channel, ...args) {
    if (!this.isOpen) return Promise.reject(new Error('not connected'))
    const id = this.nextId()
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error(`invoke ${channel} timed out after ${this.timeoutMs}ms`)) }, this.timeoutMs)
      this.pending.set(id, { resolve, reject, timer })
      this.ws.send(JSON.stringify({ type: 'invoke', id, channel, args }))
    })
  }

  close() {
    const ws = this.ws
    if (!ws || ws.readyState === this.WebSocket.CLOSED) return Promise.resolve()
    return new Promise((resolve) => {
      const timer = setTimeout(() => { try { ws.terminate() } catch { /* ignore */ } resolve() }, 2000)
      ws.once('close', () => { clearTimeout(timer); resolve() })
      ws.close()
    })
  }
}

/** Collects `pty:output` of our own PTY ids (ANSI stripped) and lets callers wait for a regex. */
class PtyTracker {
  constructor() { this.own = new Set(); this.buffers = new Map(); this.exits = new Map(); this.waiters = new Set() }
  handle(channel, args) {
    const id = args[0]
    if (typeof id !== 'string' || !this.own.has(id)) return
    if (channel === 'pty:output') {
      this.buffers.set(id, ((this.buffers.get(id) ?? '') + String(args[1] ?? '').replace(ANSI_RX, '')).slice(-262144))
    } else if (channel === 'pty:exit') {
      this.exits.set(id, args[1])
    } else return
    for (const w of [...this.waiters]) w()
  }
  text(id) { return this.buffers.get(id) ?? '' }
  waitFor(predicate, timeoutMs, what) {
    return new Promise((resolve, reject) => {
      const check = () => { const v = predicate(); if (v) { this.waiters.delete(check); clearTimeout(timer); resolve(v) } }
      const timer = setTimeout(() => { this.waiters.delete(check); reject(new Error(`timed out after ${timeoutMs}ms waiting for ${what}`)) }, timeoutMs)
      this.waiters.add(check)
      check()
    })
  }
}

// ---------------------------------------------------------------------------
// Checks
// ---------------------------------------------------------------------------
const results = []
function record(id, name, status, evidence, extra = {}) {
  results.push({ id, name, status, evidence, ...extra })
  const mark = status === 'PASS' ? '✅' : status === 'SKIP' ? '⏭️' : '❌'
  console.log(`${mark} ${id} ${name} — ${status}`)
  for (const line of String(evidence).split('\n')) console.log(`     ${line}`)
}
function fail(id, name, error, extra) { record(id, name, 'FAIL', error instanceof Error ? error.message : String(error), extra) }

async function main() {
  const opts = parseCli(process.argv.slice(2))
  const d = { dockerHost: opts.dockerHost }
  const startedAt = new Date().toISOString()
  console.log(`verify-bat-takeover — container ${opts.container}, DOCKER_HOST=${opts.dockerHost}, host folder ${opts.hostFolder}`)

  // `ws` from the BAT source checkout (no global install)
  let WebSocket
  try {
    const req = createRequire(path.join(opts.batSrc, 'package.json'))
    ;({ WebSocket } = req('ws'))
  } catch (error) {
    throw new UsageError(`cannot load "ws" from ${opts.batSrc}/node_modules (--bat-src): ${error.message}`)
  }

  // ── V1: Windows docker CLI reaches the WSL daemon through the relay; container is first of `docker ps -a`
  let containers = []
  try {
    const ps = await execDocker(['ps', '--format', '{{.Names}}|{{.Status}}|{{.Ports}}'], d)
    const psAll = await execDocker(['ps', '-a', '--format', '{{json .}}'], d)
    containers = psAll.stdout.split(/\r?\n/).filter(Boolean).map((l) => JSON.parse(l)).map((e) => ({ name: e.Names ?? '', state: e.State ?? '', status: e.Status ?? '', image: e.Image ?? '' }))
    const running = ps.stdout.split(/\r?\n/).find((l) => l.startsWith(`${opts.container}|`))
    const first = containers[0]?.name
    const ok = Boolean(running) && first === opts.container
    record('V1', 'Windows docker → WSL daemon via relay; container listed and first of `docker ps -a`', ok ? 'PASS' : 'FAIL',
      `docker ps: ${running ?? '(not running)'}\ndocker ps -a order: ${containers.map((c) => c.name).join(', ') || '(none)'} → pick-container would take "${first ?? '-'}"`)
  } catch (error) { fail('V1', 'Windows docker → WSL daemon via relay', error) }

  // ── V2: every docker probe the wizard backend runs, in order, on the same CLI / env
  let inspect = null
  let serverPort = null
  let token = null
  const mount = { host: opts.hostFolder, container: `/workspace/${basename(opts.hostFolder)}` }
  try {
    const lines = []
    const version = await execDocker(['--version'], d)                               // dockerStatus()
    await execDocker(['info', '--format', '{{.ServerVersion}}'], d)
    lines.push(`detect-env: ${version.stdout}; docker info ok`)
    if (!containers.length) throw new Error('pick-container: docker ps -a returned no container')
    lines.push(`pick-container: mode "existing" (default when any container exists) → ${containers[0].name}`)
    // configure-mounts: validateMountTable (host exists, container path absolute), /workspace/<basename>
    if (!existsSync(mount.host)) throw new Error(`configure-mounts: host path does not exist: ${mount.host}`)
    if (!mount.container.startsWith('/')) throw new Error('configure-mounts: container path must start with /')
    lines.push(`configure-mounts: ${mount.host} → ${mount.container} (validateMountTable ok)`)
    // install-server-bundle (existing): inspectContainer → image non-empty
    const raw = await execDocker(['inspect', '--type', 'container', opts.container], { ...d, timeoutMs: 5000 })
    inspect = JSON.parse(raw.stdout)[0] ?? null
    if (!inspect?.Config?.Image) throw new Error('install-server-bundle: inspect returned no image')
    lines.push(`install-server-bundle: image ${inspect.Config.Image} (bundle baked into the image, warning "existingNeedsBundle" as in the wizard)`)
    // start-server: pickDockerServerPort({container}) → published 9876/tcp, refused only when == BAT's own port
    serverPort = publishedServerPort(inspect)
    if (serverPort === null) throw new Error(`start-server: container does not publish ${SERVER_CONTAINER_PORT_KEY} on the host (docker-port-unpublished)`)
    if (serverPort === opts.batPort) throw new Error(`start-server: host port ${serverPort} is BAT's own RemoteServer port (docker-port-in-use)`)
    lines.push(`start-server/pick-server-port: ${SERVER_CONTAINER_PORT_KEY} → host ${serverPort} (reusedExistingPort; BAT's own port ${opts.batPort} not hit)`)
    // detectContainerExposure: loopback-only publish + entrypoint carries --bind-interface
    const hostIps = []
    for (const [cport, list] of Object.entries(inspect.HostConfig?.PortBindings ?? {})) {
      if (!Array.isArray(list)) continue
      for (const b of list) {
        const relevant = cport === SERVER_CONTAINER_PORT_KEY || String(b?.HostPort ?? '') === String(serverPort)
        if (relevant && !isLoopbackHostIp(b?.HostIp ?? '') && !hostIps.includes(b?.HostIp ?? '')) hostIps.push(b?.HostIp ?? '')
      }
    }
    const command = [...(inspect.Config?.Entrypoint ?? []), ...(inspect.Config?.Cmd ?? [])].map(String)
    const legacyImage = command.includes(BAT_SERVER_BIN) && !command.includes('--bind-interface')
    if (hostIps.length) throw new Error(`start-server: port published on non-loopback host interface(s): ${hostIps.map((ip) => ip || '0.0.0.0').join(', ')}`)
    if (legacyImage) throw new Error('start-server: image entrypoint predates --bind-interface (legacy image)')
    lines.push(`start-server/detectContainerExposure: exposed=false, legacyImage=false (entrypoint: ${command.join(' ')})`)
    // mount consistency: the profile's mount only translates paths; it must match the container's own bind mount
    const bind = (inspect.Mounts ?? []).find((m) => m.Destination === mount.container)
    if (!bind) throw new Error(`container has no mount at ${mount.container}; wizard folder ${mount.host} would not match`)
    if (basename(bind.Source) !== basename(mount.host)) throw new Error(`mount source ${bind.Source} is not the host folder ${mount.host}`)
    lines.push(`mount: container ${bind.Source} → ${bind.Destination} matches the profile mount ${mount.container}`)
    // startContainer(name, {port}): docker start (no-op when running) → docker exec cat server-token.json
    const wasRunning = inspect.State?.Running === true
    await execDocker(['start', opts.container], d)
    const tokenRead = await execDocker(['exec', opts.container, 'cat', TOKEN_FILE], { ...d, allowFailure: true })
    token = parsePersistedToken(tokenRead.stdout)
    if (!token) throw new Error(`start-server: no token in ${TOKEN_FILE} (${tokenRead.stderr || 'empty'})`)
    lines.push(`start-server: docker start ok (was running before: ${wasRunning}); token read from ${TOKEN_FILE} (${redactToken(token)})`)
    // waitForHealthy: inspect --format {{.State.Health.Status}} → healthy | none
    const health = (await execDocker(['inspect', '--format', '{{.State.Health.Status}}', opts.container], { ...d, allowFailure: true })).stdout || 'none'
    if (health !== 'healthy' && health !== 'none') throw new Error(`start-server: health is "${health}"`)
    lines.push(`start-server/waitForHealthy: ${health}`)
    record('V2', 'BAT wizard backend probes (detect-env → start-server) succeed on this CLI/env', 'PASS', lines.join('\n'), { serverPort, mount })
  } catch (error) { fail('V2', 'BAT wizard backend probes', error) }

  // ── V3: fetch-fingerprint — TLS handshake fingerprint == the container's server-cert.json fingerprint
  let fingerprint = ''
  if (serverPort !== null) {
    try {
      const hs = await fetchTlsFingerprint(serverPort)
      if (!hs.ok) throw new Error(hs.error)
      fingerprint = hs.fingerprint
      // only the fingerprint field leaves the container (the file also holds the private key)
      const grep = await execDocker(['exec', opts.container, 'grep', '-o', '-E', '"fingerprint"[[:space:]]*:[[:space:]]*"[0-9A-Fa-f:]+"', CERT_FILE], { ...d, allowFailure: true })
      const inFile = normalizeFingerprint(/"fingerprint"\s*:\s*"([0-9A-Fa-f:]+)"/.exec(grep.stdout)?.[1])
      if (!inFile) throw new Error(`cannot read the fingerprint field of ${CERT_FILE} in the container`)
      const same = inFile === fingerprint
      record('V3', `TLS handshake on 127.0.0.1:${serverPort}; fingerprint matches the container's server-cert.json`, same ? 'PASS' : 'FAIL',
        `handshake ${hs.protocol}, fingerprint ${redactFp(fingerprint)}; in container ${redactFp(inFile)}; equal=${same}`)
      if (!same) fingerprint = ''
    } catch (error) { fail('V3', 'TLS fingerprint', error) }
  } else record('V3', 'TLS fingerprint', 'SKIP', 'no server port (V2 failed)')

  // ── V4: connect-test — wrong token refused (first, so the real auth clears the failure counter), real token accepted
  let client = null
  let metadata = null
  if (fingerprint && token) {
    const url = `wss://127.0.0.1:${serverPort}`
    const mk = (tok, label) => new RemoteClient({ WebSocket, url, token: tok, fingerprint, timeoutMs: opts.timeout, label })
    try {
      // a random token of the same length; never `batcap.`-shaped so it exercises the server-token path
      const wrong = randomBytes(Math.max(16, Math.ceil(token.length / 2))).toString('hex').slice(0, token.length)
      let negative
      try { await mk(wrong, 'verify-bat-takeover negative probe').connect(); negative = 'ACCEPTED' } catch (error) { negative = `${error.code}: ${error.message}` }
      if (negative === 'ACCEPTED') throw new Error('a wrong token was ACCEPTED')
      client = mk(token, 'verify-bat-takeover')
      ;({ metadata } = await client.connect())
      const md = metadata && typeof metadata === 'object' ? metadata : {}
      record('V4', 'connect-test: wrong token refused; token + pinned fingerprint authenticated', 'PASS',
        `negative: ${negative}\npositive: auth-result metadata serverPlatform=${md.serverPlatform} serverArch=${md.serverArch} serverEnv=${md.serverEnv ?? '-'} node=${md.nodeVersion} bundle=${md.bundleVersion} claude=${md.claudeVersion ?? '-'}`,
        { metadata: md })
    } catch (error) { fail('V4', 'connect-test', error); client = null }
  } else record('V4', 'connect-test', 'SKIP', 'no fingerprint or token (V2/V3 failed)')

  // ── V5: a tab in the container through the BAT protocol (pty:create / pty:write / pty:output)
  if (opts.skipPty) record('V5', 'container tab via pty:create', 'SKIP', '--skip-pty')
  else if (!client) record('V5', 'container tab via pty:create', 'SKIP', 'not connected (V4 failed)')
  else {
    const tracker = new PtyTracker()
    const off = client.onEvent((ch, args) => tracker.handle(ch, args))
    const created = []
    const runTab = async (cwd) => {
      const id = `t0196-${randomBytes(4).toString('hex')}`
      tracker.own.add(id)
      const res = await client.invoke('pty:create', { id, cwd, type: 'terminal' })
      const ok = res === true || (res && typeof res === 'object' && res.ok === true)
      if (!ok) throw new Error(`pty:create(${id}) → ${JSON.stringify(res)}`)
      created.push(id)
      await tracker.waitFor(() => tracker.text(id).length > 0, opts.timeout, `first pty:output of ${id}`)
      const nonce = randomBytes(6).toString('hex')
      // The typed line echoes back as `"<<$M>>"`; only the executed command prints `<<nonce>>`.
      await client.invoke('pty:write', id, `M=${nonce}\r`)
      const script = [
        'echo "<<$M>>"',
        'echo "PWD=$(pwd)"',
        'echo "GO=$(go version 2>&1)"',
        `echo "GIT=$(git -C ${mount.container} log -1 --oneline 2>&1)"`,
        'echo "SKILLS=$(ls -1 ~/.claude/skills 2>/dev/null | tr \'\\n\' \',\')"',
        'echo "BATENV=$(env | grep ^BAT_ | cut -d= -f1 | sort | tr \'\\n\' \',\')"',
        'echo "<<END-$M>>"',
      ].join('; ')
      await client.invoke('pty:write', id, `${script}\r`)
      const m = await tracker.waitFor(() => new RegExp(`<<${nonce}>>([\\s\\S]*?)<<END-${nonce}>>`).exec(tracker.text(id)), opts.timeout, `marker output of ${id}`)
      const body = m[1].replace(/\r/g, '')
      const field = (k) => (new RegExp(`^${k}=(.*)$`, 'm').exec(body)?.[1] ?? '').trim()
      return { id, pwd: field('PWD'), go: field('GO'), git: field('GIT'), skills: field('SKILLS').split(',').filter(Boolean), batEnv: field('BATENV').split(',').filter(Boolean) }
    }
    try {
      const hostGit = await new Promise((resolve, reject) => execFile('git', ['-C', opts.hostFolder, 'log', '-1', '--oneline'], { encoding: 'utf8', windowsHide: true }, (e, out) => (e ? reject(e) : resolve(out.trim()))))
      const tab = await runTab(opts.expectCwd)
      const checks = [
        [`pwd == ${opts.expectCwd}`, tab.pwd === opts.expectCwd, tab.pwd],
        [`go version ${opts.expectGo}`, tab.go.includes(`go${opts.expectGo} `), tab.go],
        ['git log -1 equals the host', tab.git === hostGit, `container "${tab.git}" / host "${hostGit}"`],
        [`${opts.expectSkills} CT skills`, tab.skills.length === opts.expectSkills, `${tab.skills.length}: ${tab.skills.join(' ')}`],
        ['BAT_* keys injected', tab.batEnv.length > 0, tab.batEnv.join(' ')],
      ]
      // What a BAT Docker window does: the tab's cwd is the workspace folder translated through the
      // profile mount (host folder → /workspace/<basename>), not the container's working_dir.
      const rootTab = await runTab(mount.container)
      checks.push([`a tab whose workspace is the wizard folder starts at ${mount.container}`, rootTab.pwd === mount.container, rootTab.pwd])
      const allOk = checks.every((c) => c[1])
      record('V5', 'container tab via BAT protocol: pwd, go, git, CT skills, BAT_* env', allOk ? 'PASS' : 'FAIL',
        checks.map((c) => `${c[1] ? 'ok ' : 'NOK'} ${c[0]} → ${c[2]}`).join('\n'), { tab, rootTab })
    } catch (error) {
      fail('V5', 'container tab via pty:create', error)
    } finally {
      for (const id of created) { try { await client.invoke('pty:kill', id) } catch { /* best effort */ } }
      off()
    }
  }
  if (client) await client.close()

  // ── P: write-profile dry run — the ProfileEntry the wizard would persist (never written to BAT's index.json)
  if (serverPort !== null && token) {
    const now = Date.now()
    const name = `Docker ${opts.container}`
    const entry = {
      id: name.toLowerCase().replace(/[^a-z0-9\u4e00-\u9fff]+/g, '-').replace(/^-|-$/g, '') || 'profile',
      name, type: 'remote', remoteHost: 'localhost', remotePort: serverPort,
      remoteToken: `<redacted ${redactToken(token)}>`, remoteFingerprint: fingerprint ? `<redacted ${redactFp(fingerprint)}>` : undefined,
      createdAt: now, updatedAt: now, targetOS: 'docker-linux', dockerContainer: opts.container, dockerMounts: [mount],
    }
    const problems = []
    if (!entry.id || !entry.name) problems.push('id/name empty (validateProfileShape)')
    if (!fingerprint) problems.push('no fingerprint to pin (TOFU)')
    if (!(serverPort > 0 && serverPort <= 65535)) problems.push('port out of range')
    record('P', 'write-profile (dry run): ProfileEntry as electron/profile-manager.ts would store it', problems.length ? 'FAIL' : 'PASS',
      `${JSON.stringify(entry)}\nnot written: BAT's <userData>/profiles/index.json is left untouched (no program entry to make a running BAT reload it; see report)`, { profileEntry: entry })
  } else record('P', 'write-profile (dry run)', 'SKIP', 'no port/token')

  const failed = results.filter((r) => r.status === 'FAIL')
  const summary = { startedAt, finishedAt: new Date().toISOString(), container: opts.container, dockerHost: opts.dockerHost, serverPort, results, exitCode: failed.length ? 1 : 0 }
  if (opts.json) { writeFileSync(opts.json, JSON.stringify(summary, null, 2)); console.log(`results written to ${opts.json}`) }
  console.log(failed.length ? `\n${failed.length} check(s) FAILED: ${failed.map((r) => r.id).join(', ')}` : '\nall checks PASS')
  return summary.exitCode
}

main().then((code) => process.exit(code)).catch((error) => {
  if (error instanceof UsageError) { console.error(`error: ${error.message}\n\n${USAGE}`); process.exit(2) }
  console.error(error instanceof Error ? error.stack : String(error))
  process.exit(2)
})
