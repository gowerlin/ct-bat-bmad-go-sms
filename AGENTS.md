# AGENTS.md — sms-platform（課程示範專案）

> 雛形（P1-c，T0193）。CH2 2-07（`bmad-project-context`）會補 `<!-- bmad -->` 區塊；CH3 起課程工單在 `_ct-workorders/` 建立。
> 任何在本目錄工作的 agent（Claude Code Worker、Codex、BMad skill）都先讀本檔。

## 專案

- Go 示範專案：SMS 通訊平台（REST 發送 API、供應商 adapter、佇列重試、狀態回呼、模板、速率限制、多租戶 API key），**沒有 Web 主控台**。
- 獨立 Go module：`example.com/sms-platform`（`go.mod`）。`example.com` 是 RFC 2606 保留網域，永遠不會被 `go get` 解析，適合教學專案；學員換成自己的 host 路徑時只需改 `go.mod` 與 import。
- 上層課程計畫：`../COURSE-PLAN.md`（§ 5 貫穿線、§ 6.2 供應商策略、§ 7.1 目錄）。

## 工具版本（本課錄製時）

| 工具 | 版本 | 鎖定處 |
|---|---|---|
| Go | **1.27.3** | `go.mod` 的 `go 1.27.3`（Go 1.21+ 的 `go` 指示詞就是最低工具鏈版本，`GOTOOLCHAIN=auto` 會自動切換；同版的 `toolchain` 行會被 Go 視為多餘而拒絕）；CI 以 `go.mod` 為準 |
| BMad-Method | v6.12.4（core＋bmm＋tea） | `_bmad/_config/manifest.yaml` |
| GNU make | 任何 4.x | `Makefile` |

## 收單門檻（每張工單）

- **`make test`（= `go test ./... -race`）全綠**是每張工單的收單條件；`go build ./...` 與 `go vet ./...` 也必須乾淨。
- 每個功能增量**必附 E2E 測試**，放 `e2e/`，用 `e2e/testutil.NewTestServer`（httptest 包真實 handler）＋ `FakeClock`／`FakeProvider`，不連網路、不睡真實時間。
- E2E 命名：`TestE2E_<Feature>_<Behavior>`，例如 `TestE2E_Healthz`、`TestE2E_SendMessage_Returns202AndQueryable`、`TestE2E_RateLimit_Returns429WithRetryAfter`（清單見 COURSE-PLAN § 5.1）。
- 工單的 `affects_files` 以本目錄為相對路徑；只改列出的檔案。

## 套件邊界

| 目錄 | 角色 |
|---|---|
| `cmd/smsd/` | 進入點：組裝依賴、開 listener、優雅關機。不放業務邏輯 |
| `internal/httpapi/` | HTTP 邊界：路由、JSON、錯誤信封 `{"error":{"code","message"}}`、狀態碼 |
| `internal/queue/` | 發送佇列、重試、狀態轉移 |
| `internal/provider/` | `Provider` 介面（`Send`／`ParseCallback`／`Name`）；`mock/`（主線）、`mitake/`（CH5，三竹風格，只打 `httptest.Server`） |
| `internal/template/` | 模板 CRUD 與 `{{var}}` 渲染 |
| `internal/ratelimit/` | 每 API key 限流 |
| `internal/tenant/` | 租戶、API key hash、隔離規則 |
| `internal/webhook/` | 入站供應商回呼、出站 HMAC 通知（冪等） |
| `internal/store/` | 持久化邊界（in-memory；SPINE 留 DB 邊界） |
| `e2e/` | E2E 測試與 `testutil/` harness |
| `docs/` | `openapi.yaml`、資料模型（也是 BMad `project_knowledge`） |
| `_bmad/`、`_bmad-output/` | BMad 安裝與規劃產物（進 git） |
| `_ct-workorders/` | 課程工單（CH3 道館起） |

規則：`cmd` 只能 import `internal/*`；`internal/*` 之間依 `doc.go` 的「不擁有」互不越界（例如 `httpapi` 不碰 store 的資料結構細節，`provider` 不重試）。時間一律經注入的 `func() time.Time`，不直接呼叫 `time.Now`（`cmd` 組裝處除外）。

## 禁止事項

- **不連外網**：測試與示範不得呼叫真實 SMS 供應商或任何外部服務；供應商一律 `mock` 或 `httptest.Server` 假對端。
- **不用真實憑證**：不得出現真實 API key、token、手機號碼；示範號碼用保留號段 `+8869000000xx`；`.env` 不進 git，只有 `.env.example` 列鍵名。
- **不略過測試**：不得用 `t.Skip`、刪除測試或改寬斷言來讓門檻變綠。
- **不新增外部依賴**（`go.mod` 只有標準庫；godog 為 CH5 選配且需工單明示）。
- **不動 `_bmad/`**（安裝器產物）；要改設定走 `_bmad/custom/` 或重跑安裝器。

<!-- bmad: 由 CH2 2-07 的 bmad-project-context 填寫 -->
