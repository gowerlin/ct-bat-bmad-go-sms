# sms-platform —— 課程示範專案（Go SMS 通訊平台）

課程《多代理規格驅動開發實戰：用 BMad-Method、Control Tower 與 BAT 以 Go 打造 SMS 通訊平台》的貫穿示範專案（`../COURSE-PLAN.md` § 5）。
P1-c（T0193）建立的是**骨架**：`GET /healthz`、E2E harness、套件邊界、OpenAPI 空殼、自裝 BMad、dev container。功能從 CH1 道館的 T0001 起逐張工單加入。

> 本課錄製時版本：Go **1.27.3**、BMad-Method **v6.12.4**（core＋bmm＋tea v1.27.2）、Control Tower **v5.12.2**、BAT **0.7.1-pre.6**、Claude Code 2.1.303（主機）／2.1.306（bat-server bundle 內建）、Docker Engine 29.1.3（WSL2 Ubuntu-24.04）＋ Compose 2.40.3＋ buildx 0.30.1；主機 GNU make 4.4.1、MinGW gcc 16.1.0。

## 目錄

```
sms-platform/
├── go.mod                 # module example.com/sms-platform；go 1.27.3
├── Makefile               # build / vet / test / e2e / run
├── AGENTS.md              # 給 agent 的專案規則（收單門檻、套件邊界、禁止事項）
├── cmd/smsd/              # 進入點（可注入 listener）
├── internal/              # httpapi queue provider(mock) template ratelimit tenant webhook store
├── e2e/                   # TestE2E_* 與 testutil/（NewTestServer、FakeClock、FakeProvider）
├── docs/                  # openapi.yaml、資料模型（也是 BMad project_knowledge）
├── _bmad/ _bmad-output/   # 自裝 BMad（進 git）與規劃產物（進 git）
├── _ct-workorders/        # 課程工單（CH3 道館起；示範工單用 T9xxx）
├── Dockerfile compose.yaml# dev container（以 bat-server:latest 為基底；掛課程 repo 根，D116 ①）
└── scripts/               # dev-container.sh / .ps1、wsl-docker-relay.sh、verify-bat-takeover.ps1 / .mjs（T0196 自動驗收）
```

## 快速開始（主機）

需求：Go 1.27.3、GNU make、gcc（`-race` 需要 cgo；Windows 可 `choco install make mingw`）。

```bash
cd courses/ct-bat-bmad-go-sms/sms-platform
make test      # go test ./... -race —— 每張工單的收單門檻
make e2e       # 只跑 TestE2E_*（-v）
make run       # smsd 在 http://127.0.0.1:8080，GET /healthz → {"status":"ok"}
make build     # bin/smsd（.gitignore 排除）
```

## 安裝 BMad（學員照抄；D-2／D-5）

本專案自己裝一份 BMad v6.12.4（core＋bmm＋tea），`project-root` 就是 `sms-platform/`。安裝器產物分兩類：

| 路徑 | 內容 | git |
|---|---|---|
| `_bmad/` | 設定、manifest、模組檔（`_bmad/_config/manifest.yaml` 是版本依據） | **進 git** |
| `_bmad-output/` | 規劃與實作產物（planning-artifacts／implementation-artifacts／test-artifacts） | **進 git** |
| `.claude/skills/`、`.agents/skills/` | 安裝器產生的 skill 副本（各 39 個、約 14 MB） | `.gitignore` 排除；clone 後重跑下面的指令即可還原 |

在 `sms-platform/` 執行（非互動，可重現；`--pin tea=v1.27.2` 是 v6 安裝器能裝的 TEA 最高版）。macOS／Linux／WSL 用 bash：

```bash
npx bmad-method@6.12.4 install --directory . \
  --modules bmm,tea --tools claude-code,codex \
  --user-name <你的名字> --communication-language 繁體中文 --document-output-language 繁體中文 \
  --output-folder _bmad-output --pin tea=v1.27.2 --no-shims --yes
```

Windows 用 PowerShell（逗號清單加引號）：

```powershell
npx bmad-method@6.12.4 install --directory . `
  --modules 'bmm,tea' --tools 'claude-code,codex' `
  --user-name <你的名字> --communication-language 繁體中文 --document-output-language 繁體中文 `
  --output-folder _bmad-output --pin tea=v1.27.2 --no-shims --yes
```

- **Windows 不要在 Git Bash 跑**：安裝器會把 Git Bash 的目前目錄（`/c/…`）判成 WSL，報「Windows Node.js was launched from a WSL shell」並 exit 1。改用 PowerShell。
- **PowerShell 下 `--modules`／`--tools` 的逗號清單要加引號**：不加引號會被 PowerShell 拆成陣列，安裝器收到 `claude-code codex`，報 `Unknown tool ID: claude-code codex` 並 exit 1。（T0216 在副本實測：Git Bash exit 1；PowerShell 不加引號 exit 1；加引號 exit 0、約 18 秒。）

- 只用 Claude Code 的話把 `--tools claude-code,codex` 改成 `--tools claude-code`。
- 課程 repo 提交的 `_bmad/` 設定用中性名 `student`（`config.user.toml` 與 `{bmm,core,tea}/config.yaml` 的 `user_name`，T0215）。在**課程 repo 本身**重跑安裝時帶 `--user-name student`（或不帶，安裝器會沿用上次的值），不要帶自己的名字；重裝會把手改過的 `config.user.toml` 備份成 `config.user.toml.bak` 一次，刪掉即可。在自己的 repo 裡照上面填自己的名字。
- 驗證：`_bmad/_config/manifest.yaml` 的 `installation.version: 6.12.4` 與 core／bmm／tea 三個模組；`ls .claude/skills | wc -l` 為 39；在 `sms-platform/` 啟動 `claude`，`/bmad-help` 可用。
- **一律在 `sms-platform/` 啟動 BMad 的 session**：使用者層級可能有 V7 線的 skill，Claude Code 會把它們一起列出來（名稱相同時以專案層為準）。課程 repo 根目錄原本另一份製作用的完整安裝已於 G1 移出 repo（D116 ②），不再混入。

## dev container（§ 6.4 C；4-09／4-10 用）

容器＝`bat-server:latest`（BAT headless server，內建 node 與 claude）＋ Go 1.27.3＋git／make／gcc＋8 個 CT skill（`/root/.claude/skills`）。**bind mount 掛的是課程 repo 根**（`D:\…\BMad-Guide` → `/workspace/BMad-Guide`；D116 ①，T0194），不是 `sms-platform/`：這樣 `.git` 在 mount 內，Worker 在容器內 commit 會直接進課程 repo，主機端 `git log` 立即可見。容器的 `working_dir` 是 `/workspace/BMad-Guide/courses/ct-bat-bmad-go-sms/sms-platform`，`docker exec` 與 BAT 分頁一開就在 Go module 內。server 埠只發布到 `127.0.0.1:9880`（避開 BAT 自己的 9876）。

**學員版（自己的 repo）**：一樣掛**自己的 repo 根**（`compose.yaml` 的 `../../..:/workspace/BMad-Guide` 改成 `<你的 repo 根>:/workspace/<repo 目錄名>`，`working_dir` 改成 repo 內 Go module 的路徑；或 `SMS_REPO_ROOT=<repo 根> scripts/dev-container.sh up`），精靈也選同一個 repo 根。原則：精靈選哪個資料夾，compose 就掛哪個資料夾，容器路徑固定 `/workspace/<資料夾名>`。

### 一次性前置

1. **Docker 在 WSL 內**（本課錄製主機的做法，D113）：Ubuntu-24.04 內 `docker.io` 29.1.3＋`docker-compose-v2` 2.40.3＋`docker-buildx` 0.30.1（`sudo apt-get install -y docker.io docker-compose-v2 docker-buildx`，使用者加入 `docker` 群組）。**所有 docker 指令都在 WSL 內執行**；Windows 端的 `docker.exe` 只在「讓 BAT 精靈接管」時經下面的中繼使用。
2. **自建 `bat-server:latest`**（BAT 不發布映像；每次升級 BAT 要重建）。在 WSL 內：
   ```bash
   git clone <BAT 原始碼> ~/bat-build/better-agent-terminal && cd ~/bat-build/better-agent-terminal
   git checkout v<你的 BAT 版本>     # 與安裝版相同
   source ~/.nvm/nvm.sh && npm ci && npm run build:docker-image
   # → bat-server:<version> 與 bat-server:latest（0.7.1-pre.6 約 376 MB；BAT 自己的 verify:docker-image 有 300 MB 上限，超過只是警告）
   ```
   要在 WSL 內 clone 一份（不要直接用 Windows 檢出的目錄）：`build:server-bundle` 需要 `node_modules` 裡有 linux-x64 的原生套件，Windows 端 `npm install` 裝的是 win32-x64。
3. 主機（Windows）要跑 `make test` 的話需要 GNU make 與 MinGW gcc（`-race` 需 cgo；`choco install make mingw`）。容器內已經有 make／gcc。

### 啟動／停止

```bash
# WSL 內（compose）
cd /mnt/d/.../courses/ct-bat-bmad-go-sms/sms-platform
docker compose config | grep -A1 'source: /mnt'   # mount 應是 …/BMad-Guide → /workspace/BMad-Guide
docker compose build dev        # 第一次或 Dockerfile／CT skill 變更後
docker compose up -d dev        # 容器 bat-server-sms-platform
docker port bat-server-sms-platform          # 9876/tcp -> 127.0.0.1:9880
docker inspect --format '{{.State.Health.Status}}' bat-server-sms-platform   # healthy（約 6–10 秒）
docker exec bat-server-sms-platform sh -lc 'go test ./... -race'             # working_dir 已在 sms-platform/
docker exec bat-server-sms-platform sh -lc 'git log -1 --oneline'            # 容器內看得到課程 repo 的 git
docker compose stop dev / docker compose down   # down 保留 named volume（token 與憑證）

# 沒有 compose 外掛時（純 docker run，同樣在 WSL 內；mount 與 working_dir 同 compose）
scripts/dev-container.sh up | test | status | shell | stop | down

# Windows 端包裝（呼叫 wsl -e 執行上面的腳本）
.\scripts\dev-container.ps1 up
.\scripts\dev-container.ps1 test      # 容器內 go test ./... -race
.\scripts\dev-container.ps1 status    # 印 mount、workdir、埠、health
```

CT skill 來源是 `../../../BMad-Control-Tower/BMad-Control-Tower-v5.12.2/skills`，以 BuildKit 具名 context `ct-skills` 進映像（`compose.yaml` 的 `additional_contexts`；`dev-container.sh build` 用 `docker build --build-context`）。升級 CT 後重新 `docker compose build dev`。沒有 buildx 的環境可改成在 `compose.yaml` 加一行 volume `../../../BMad-Control-Tower/BMad-Control-Tower-v5.12.2/skills:/root/.claude/skills:ro` 並刪掉 Dockerfile 的 `COPY --from=ct-skills`。

### 讓 BAT 的 Docker 精靈「Use existing」接管（G1 驗收：`scripts/verify-bat-takeover.ps1` 自動化；手動 5 步為學員教學版）

BAT 的精靈呼叫的是 **Windows 端的 `docker` CLI**（`execFile('docker', …)`，`electron/docker-detect.ts`／`docker-lifecycle.ts`），不透過 WSL、不讀 BAT profile，所以 Windows 的 `docker` 必須指到 WSL 內的 daemon。本課採**做法 A**（T0193 實測、D115）：使用者層級的 socat 中繼＋`docker context`，**不需 sudo、不改 dockerd 設定**。

**自動驗收（T0196；錄製主機一鍵跑）**

```powershell
.\scripts\verify-bat-takeover.ps1                      # exit 0 = V1–V6 全 PASS
.\scripts\verify-bat-takeover.ps1 -Json "$env:TEMP\t0196.json"   # 另存結果（機密已遮）
```

BAT 沒有能從外部建立／測試 Docker profile 的 CLI 或 Remote API（`docker:*`、`profile:create/update` 只是 renderer 的 `ipcMain.handle`，不在 `PROXIED_CHANNELS`；helper 權杖只授權 `terminal:*`／`question:*`），所以腳本**以程式重現精靈後端每一步**（`src/components/setup-wizard/docker-flow.ts` 與各 step），不經 GUI：

| 項 | 做什麼 | 對應精靈步驟／BAT 程式碼 |
|---|---|---|
| 前置 | `wsl-docker-relay.sh start`（已在跑就沿用）；先記下 docker context、`DOCKER_HOST`、BAT `profiles/index.json` 雜湊、容器 health | — |
| V1 | Windows `docker`（`DOCKER_HOST=tcp://127.0.0.1:2375` 只給腳本的子程序）看得到容器，且是 `docker ps -a` 第一個 | `pick-container` 取第一個 |
| V2 | `docker --version`／`info`／`ps -a --format json`／`inspect`；mount 轉成 `/workspace/<資料夾名>` 並與容器實際 mount 比對；已發布埠 `9876/tcp → 9880` 且 ≠ BAT 自己的埠；loopback 發布、entrypoint 有 `--bind-interface`；`docker start`；`docker exec cat server-token.json`；health | `detect-env`、`configure-mounts`、`install-server-bundle`、`start-server`（`docker-detect.ts`、`docker-validate.ts`、`docker-lifecycle.ts`） |
| V3 | 對 `127.0.0.1:9880` TLS 握手取 `fingerprint256`，與容器內 `server-cert.json` 的 `fingerprint` 欄位比對（只取該欄位） | `fetch-fingerprint`（`tls-fingerprint.ts`） |
| V4 | 先用錯誤 token 連線（預期 `Invalid token`），再用正確 token＋釘住的指紋走 WSS `auth` 幀 | `connect-test`（`remote-client.ts`；錯誤連線放在前面，成功認證會清掉 server 的失敗計數） |
| V5 | 在同一條已認證連線上 `pty:create`（不是 `docker exec`），`pty:write` 輸入指令、從 `pty:output` 收：`pwd`、`go version`、`git log -1`（與主機比對）、`ls ~/.claude/skills`（8 個）、`BAT_*` 鍵名；另開一個 cwd 在 mount 根的分頁驗 BAT 視窗的實際起點 | 開 profile → 分頁（`pty-manager.ts`） |
| P | 印出精靈 `write-profile` 會寫的 `ProfileEntry`（token／指紋遮蔽），**不寫入** BAT 的 `index.json` | `write-profile`（`profile-manager.ts`） |
| V6 | 停中繼（`ss -ltn` 無 2375）、docker context 與 `DOCKER_HOST` 未動、`index.json` 雜湊未變、容器仍 healthy | 收尾 |

不做的事：不改使用者的 docker context、不寫 BAT 設定、不重啟 BAT、不印 token／指紋全值（只記長度與前 4 碼）。**真正把 profile 寫進 BAT 仍要走 GUI**（BAT 在跑時只有精靈能讓它寫入並載入），但精靈每一步呼叫的後端都已由腳本驗過；GUI 操作見下面的手動步驟。只跑 Node 部分：`node scripts/verify-bat-takeover.mjs --help`（`ws` 取自 repo 內 BAT 原始碼的 `node_modules`，不另裝）。

**做法 A 手動步驟（學員教學版；課程 4-09 用）**

1. WSL 內：`scripts/wsl-docker-relay.sh start`（socat 把 `/var/run/docker.sock` 轉到 `127.0.0.1:2375`；WSL `networkingMode=Mirrored` 時 Windows 的 127.0.0.1 直接可達）。印出 `relay ok: server=29.1.3` 即成功。
2. Windows 端（PowerShell；context 只需建一次，之後只要切換）：
   ```powershell
   docker context create wsl --docker host=tcp://127.0.0.1:2375   # 只需做一次
   docker context use wsl
   docker ps          # 應列出 bat-server-sms-platform  127.0.0.1:9880->9876/tcp
   ```
   不想動 context 的話，改成啟動 BAT 前設 `$env:DOCKER_HOST = 'tcp://127.0.0.1:2375'`。
3. 確認容器是 `docker ps -a` 的**第一個**（最新建立的；精靈沒有選擇器）。剛 `up` 完就是。
4. BAT：側欄 **Profiles → + More ▾ → Add Docker Profile** → 偵測到 docker → **Use existing** → 工作區資料夾選 **課程 repo 根 `D:\…\BMad-Guide`**（學員：自己的 repo 根）。容器端固定 `/workspace/<資料夾名>`＝`/workspace/BMad-Guide`，必須與 compose 的 mount 一致 → 精靈讀 `server-token.json`、取指紋、連線測試、寫 profile。
5. 開 profile → Docker 分頁 → `pwd`（應是 `/workspace/BMad-Guide/courses/ct-bat-bmad-go-sms/sms-platform`）、`env | grep ^BAT_ | cut -d= -f1`、`go version`、`git log -1 --oneline`、`ls ~/.claude/skills`。
6. **用完收尾**：Windows `docker context use default`；WSL `scripts/wsl-docker-relay.sh stop`（`ss -ltn` 不再有 2375）。已建好的 profile 不受影響（連的是 `127.0.0.1:9880` 的 server，不是 docker API）；只有再跑精靈或讓 BAT 替你 `docker start` 時才需要重開中繼。

錄製主機實測（T0196）：上面每一步精靈會呼叫的後端（第 1–3 步的 docker 指令、第 4 步的 token／指紋／連線測試、第 5 步的分頁與指令輸出、第 6 步收尾）都由 `verify-bat-takeover.ps1` 跑過並 PASS；第 4 步 GUI 本身（點選精靈、寫入 profile）沒有程式入口，留給學員照步驟操作。

**安全邊界**：`127.0.0.1:2375` 是**無認證**的 docker API，拿到它等於拿到 WSL 的 root（與 `docker` 群組同等信任）。中繼只綁 loopback（腳本寫死 `bind=127.0.0.1`，不要改成 0.0.0.0）、只在需要精靈時啟動、用完就 `stop`；不要把 `wsl` context 留作預設。

替代做法（評估，未採用）：
- WSL 內 dockerd 加 `-H tcp://127.0.0.1:2375`（`/etc/systemd/system/docker.service.d/override.conf` 覆寫 `ExecStart`）——效果相同但要 sudo 與改 daemon 設定、常駐，且 dockerd 會警告明文 TCP。
- Docker Desktop（WSL2 backend）——Windows `docker` 預設就通，但本課錄製主機不用（D113）。
- `ssh://` context 到 WSL——WSL 內沒有 sshd，多一個服務，不採。

### 常見坑

| 坑 | 說明／處置 |
|---|---|
| 路徑變長 | 容器內的專案路徑是 `/workspace/BMad-Guide/courses/ct-bat-bmad-go-sms/sms-platform`，不是 `/workspace/sms-platform`。compose 與 Dockerfile 已把 `working_dir` 設在這裡；派工 `--cwd` 與 tape 裡的 `cd` 要用完整路徑。 |
| 分頁起點不是 `working_dir` | BAT 分頁的 cwd 由用戶端在 `pty:create` 帶入（工作區資料夾經 profile mount 轉譯：精靈選 repo 根 → `/workspace/BMad-Guide`）；容器的 `working_dir` 只影響 `docker exec` 與 entrypoint。T0196 實測 cwd 帶 `/workspace/BMad-Guide` 或 `…/sms-platform` 都能開。要在 `sms-platform/` 工作：Docker 視窗把工作區指到該子目錄，或派工時 `--cwd` 給完整路徑。 |
| 容器看得到整個 mono-repo | mount 是課程 repo 根，容器內 `ls /workspace/BMad-Guide` 會看到課程以外的目錄（CT、BAT 原始碼、其他專案）。示範與 tape 一律在 `sms-platform/` 內操作；`git status` 不帶路徑時會掃整個 repo（/mnt/d 上第一次約 20 秒，之後約 2 秒），在 `sms-platform/` 內用 `git status .`。 |
| 容器內 `.git` 可用、commit 直接進課程 repo | 容器內 `git log`／`git commit` 操作的就是課程 repo（`dev-main` 分支），不是副本；commit 一律 `git commit --only <檔案>`、訊息帶工單號，不要 `git add -A`。主機端立刻可見（`git log -1`）。 |
| 根目錄的 gitlink 會讓 Linux git 炸掉 | 課程 repo 根追蹤了一個 gitlink（`better-agent-terminal/.ct-integration/better-agent-terminal`），它的 `.git` 指標是 Windows 絕對路徑，在容器（與 WSL）內任何 `git status`／`git commit` 都會 `fatal: not a git repository: …D:/…`。映像已設 `git config --global diff.ignoreSubmodules all` 避開（`sms-platform/` 沒有 submodule，不受影響）；自己的 repo 沒有這個問題。 |
| root 擁有者 | server 以 root 跑；在 `/mnt/d`（drvfs，`uid=1000,gid=1000`、權限一律 777）上容器建的檔案主機端看到的仍是你的 WSL 使用者，Windows 端無差；Linux 主機（ext4）則會變 root 擁有，要 `chown` 或改用 `--user`。容器內 git 已設 `safe.directory '*'`。 |
| 路徑轉譯 | BAT 只轉譯 mount 內的路徑（`DockerPathTranslator`）；現在 mount 是 repo 根，課程 repo 內的檔案都能轉譯，repo 外的附件仍回 `outside-docker-mounts`。 |
| 登入 shell 的 PATH | `sh -lc` 會重讀 `/etc/profile`；映像已把 Go 放進 `/etc/profile.d/go.sh` 與 `/usr/local/bin`。 |
| `/mnt/d` 效能 | 9p；冷快取 `go test ./... -race` 約 8–9 秒、暖快取 0.4 秒（骨架規模；掛整個 mono-repo 後沒有變慢，Go 只讀 module 目錄）。大專案可把 `GOCACHE` 放 named volume。 |
| 升級重建 | 升級 BAT → 重建 `bat-server:latest` → `docker compose build dev` → `up -d --force-recreate`；精靈每次都建新 profile，舊的自己刪。 |
| 埠 | 只發布 loopback；9880 避開 BAT 自己的 9876。換埠改 `compose.yaml` 的 `ports`（精靈 Use existing 讀容器已發布的埠）。 |
| BAT 不會 `docker start` | 停掉的容器要自己 `docker compose start dev` 再開 profile。 |
| 行尾 | `.gitattributes` 強制 `*.sh`、`Makefile`、`Dockerfile`、`*.yaml`、`*.go` 為 LF；`dev-container.ps1` 執行前會再去掉 CR。 |
| Windows `docker.exe` 找不到容器 | context 還在 `default`（npipe）或中繼沒開。`docker context ls` 確認 `wsl *`；WSL `scripts/wsl-docker-relay.sh status`。 |

## 供應商免責

本課只模擬公開文件描述的表單風格供應商介面（`internal/provider/mitake`，CH5），不代表三竹實際介面，不連接真實端點，與三竹資訊無合作或背書關係。課程主線一律用 `internal/provider/mock`。
