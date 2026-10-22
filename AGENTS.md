# AGENTS.md — sms-platform（課程示範專案）

<!-- bmad:context -->
<!-- Verified 2026-10-22 against b357062. Managed by bmad-project-context; edits inside this block are replaced on refresh. Keep anything you want preserved outside the markers. -->

## sms-platform

Go 示範專案：SMS 通訊平台（REST 發送 API、供應商 adapter、佇列重試、狀態回呼、模板、速率限制、多租戶 API key），沒有 Web 主控台。獨立 module `example.com/sms-platform`（保留網域，永不被 `go get` 解析；學員換成自己的 host 時只改 `go.mod` 與 import）。任何在本目錄工作的 agent 都先讀本檔。規劃產物在 `_bmad-output/`，課程計畫 `../COURSE-PLAN.md`（§ 5 貫穿線、§ 6.2 供應商策略、§ 7.1 目錄）。

## Policy

- 不連外網：測試與示範不得呼叫真實 SMS 供應商或外部服務；供應商一律用 `mock` 或 `httptest.Server`。
- 不用真實憑證：不得出現真實 API key、token、手機號碼；示範號碼用 `+8869000000xx`。`.env` 不進 git，只有 `.env.example` 列鍵名。
- 不略過測試：不得用 `t.Skip`、刪除測試或改寬斷言讓門檻變綠。
- 不新增外部依賴：`go.mod` 只直接依賴 godog（CH5 5-07 選配，只給 `features/` 用，`e2e/architecture_test.go` 把關）；其他依賴須工單明示。
- 不動 `_bmad/`（安裝器產物）；要改設定走 `_bmad/custom/` 或重跑安裝器。
- 工單只改 `affects_files` 列出的檔案（路徑相對本目錄）。

## Where things are

| 目錄 | 角色 |
|---|---|
| `cmd/smsd/` | 進入點：組裝依賴、開 listener、優雅關機。不放業務邏輯 |
| `internal/httpapi/` | HTTP 邊界：路由、JSON、錯誤信封 `{"error":{"code","message"}}`、狀態碼 |
| `internal/queue/` | 發送佇列、重試、狀態轉移 |
| `internal/provider/` | `Provider` 介面（`Send`／`ParseCallback`／`Name`）；`mock/`（主線）、`mitake/`（CH5，尚未建立，只打 `httptest.Server`） |
| `internal/template/` | 模板 CRUD 與 `{{var}}` 渲染 |
| `internal/ratelimit/` | 每 API key 限流 |
| `internal/tenant/` | 租戶、API key hash、隔離規則 |
| `internal/webhook/` | 入站供應商回呼、出站 HMAC 通知（冪等）；目前只有 `doc.go` |
| `internal/store/` | 持久化邊界（in-memory；SPINE 留 DB 邊界） |
| `e2e/` | E2E 測試與 `testutil/` harness |
| `docs/` | `openapi.yaml`、資料模型（也是 BMad `project_knowledge`） |
| `_bmad/`、`_bmad-output/` | BMad 安裝與規劃產物（進 git） |
| `_ct-workorders/` | 課程工單 |

- `cmd` 只能 import `internal/*`；`internal/*` 之間依各 `doc.go` 的「不擁有」互不越界（例如 `httpapi` 不碰 store 資料結構細節，`provider` 不重試）。

## Running and verifying

- 收單門檻：`make test`（= `go test ./... -race`）全綠，`go build ./...` 與 `go vet ./...` 乾淨。
- Windows 主機跑 `make test` 要先裝 MinGW gcc 並設 `CGO_ENABLED=1`（`-race` 需要 cgo）；dev container 已內建 gcc。
- Go 1.27.3 以 `go.mod` 的 `go 1.27.3` 為準（`GOTOOLCHAIN=auto` 自動切換）；不要加同版的 `toolchain` 行，Go 會拒絕。BMad-Method v6.12.4（core＋bmm＋tea），GNU make 4.x。
- BMad skill 都需要 `uv`；沒有 `uv` 會停下來，先裝再繼續。
- 每個功能增量必附 E2E 測試放 `e2e/`，用 `e2e/testutil.NewTestServer` ＋ `FakeClock`／`FakeProvider`，不連網路、不睡真實時間。

## Conventions that differ from defaults

- 時間經注入的 `func() time.Time`；業務邏輯不得直接呼叫 `time.Now`。建構子的預設值與 `cmd` 組裝處可以。
- E2E 命名 `TestE2E_<Feature>_<Behavior>`，例如 `TestE2E_SendMessage_Returns202AndQueryable`、`TestE2E_RateLimit_Returns429WithRetryAfter`（清單見 COURSE-PLAN § 5.1）。

## Known pitfalls

- 驗證腳本還沒成功跑完前，不要在 `.memlog.md` 寫「通過」；先看到腳本成功的輸出，再寫。（bmad-spec 發生過兩次）
- 暫存腳本寫在 `_bmad-output/tmp/`，不要寫到專案目錄外的系統暫存資料夾。（PRD validate 發生過一次）

<!-- /bmad:context -->

## Windows 主機跑 `-race`：先讓這個 shell 找得到 gcc

`go test ./... -race` 要 cgo，cgo 要 gcc。Windows 主機請先在**目前這個 shell** 做下面三步，再跑收單門檻；不要改系統設定或使用者設定檔。dev container 已內建 gcc，不必做。

1. 檢查：`gcc --version`（PowerShell 也可用 `Get-Command gcc`）。有版本號就跳到第 3 步。
2. 找不到時，把 MinGW-w64 安裝位置底下含 `gcc.exe` 的 `bin` 目錄加入 PATH（位置依各人安裝而定；不知道裝在哪就搜尋，例如 PowerShell：`Get-ChildItem C:\ProgramData, C:\msys64, "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Filter gcc.exe -Recurse -Depth 5 -ErrorAction SilentlyContinue | Select-Object -First 3 FullName`）：
   - PowerShell：`$env:Path = '<MinGW 的 bin 目錄>;' + $env:Path`
   - Git Bash：`export PATH="$(cygpath -u '<MinGW 的 bin 目錄>'):$PATH"`
3. 設 `CGO_ENABLED=1`（PowerShell：`$env:CGO_ENABLED = '1'`；Git Bash：`export CGO_ENABLED=1`），用 `go env CGO_ENABLED CC` 確認是 `1` 與 `gcc`，再跑 `go test ./... -race -count=1`。

找不到 gcc 就停下來回報，不要拿掉 `-race` 讓門檻變綠。
