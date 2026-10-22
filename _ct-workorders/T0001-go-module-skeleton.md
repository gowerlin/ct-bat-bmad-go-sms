---
schema_version: 1
schema_kind: workorder
id: T0001
title: 建 Go module 骨架與 /healthz E2E
type: impl
status: DONE
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T10:25:03+0800"
started_at: "2026-10-22T10:25:03+0800"
completed_at: "2026-10-22T10:25:43+0800"
updated_at: "2026-10-22T10:25:43+0800"
sizing: S
affects_files:
  - go.mod
  - cmd/smsd/main.go
  - internal/httpapi/doc.go
  - internal/httpapi/handler.go
  - internal/provider/doc.go
  - internal/provider/provider.go
  - internal/provider/mock/mock.go
  - e2e/healthz_test.go
  - e2e/testutil/server.go
  - e2e/testutil/clock.go
  - e2e/testutil/provider.go
  - _ct-workorders/T0001-go-module-skeleton.md
interaction:
  mode_hint: off
  interactive: false
  intervention_type: fire-and-forget
renew_count: 0
workdir: main repo
---
# 工單 T0001-go-module-skeleton

<!--
CH1 道館參考解（git tag dojo-ch1）。
- repo 以「sms-platform/ 自己就是 git repo 根」為準，與學員的做法相同。課程 repo 裡的 sms-platform/ 只是子目錄：
  若在課程 repo 裡派這張工單，repo 要填課程 repo 的目錄名，affects_files 也要加上 courses/ct-bat-bmad-go-sms/sms-platform/ 前綴（道館 README Q3）。
- 本檔的回報區對應課程 repo 裡現有的骨架（commit 2fa0921，課程維護者預先建好，之後每一章都在它上面長功能），
  驗收指令是在課程 repo 的 sms-platform/ 實際重跑的結果。Worker 真實執行 /ct-exec T0001 的完整過程
  （工單、逐字稿、commit 順序）見 chapters/ch1/unit-06/assets/：那是在獨立的示範 repo 跑出來的，檔案少、結構相同。
-->

## 元資料
- **工單編號**：T0001
- **任務名稱**：建 Go module 骨架與 /healthz E2E
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 10:25 (UTC+8)
- **開始時間**：2026-10-22 10:25 (UTC+8)
- **完成時間**：2026-10-22 10:25 (UTC+8)
- **intervention_type**：fire-and-forget
- **affects_files**：
  - `go.mod`
  - `cmd/smsd/main.go`
  - `internal/httpapi/doc.go`、`internal/httpapi/handler.go`
  - `internal/provider/doc.go`、`internal/provider/provider.go`、`internal/provider/mock/mock.go`
  - `e2e/healthz_test.go`
  - `e2e/testutil/server.go`、`e2e/testutil/clock.go`、`e2e/testutil/provider.go`
  - `_ct-workorders/T0001-go-module-skeleton.md`

## 任務指令

### 前置條件
- 讀 `_ct-workorders/_tower-state.md`（專案現況）
- 本機 `go version` 為 go1.27.3（未裝 Go → 先回報 BLOCKED，不要自己換版本）

### 輸入上下文
sms-platform 是 Go 寫的 SMS 發送 API 平台，這張工單只建骨架：一個獨立的 Go module、一個可啟動的 `smsd`、`GET /healthz`，
以及一個用 `httptest` 跑真實 handler 的 E2E 測試。不做發送、佇列、資料庫；不加任何第三方套件（D001）。

### 預期產出
- `go.mod`：`module example.com/sms-platform`、`go 1.27.3`，沒有 `require`
- `cmd/smsd/main.go`：預設監聽 `127.0.0.1:8080`（`-addr` 可改），掛上 `internal/httpapi` 的 handler，收到中斷訊號時優雅關閉
- `internal/httpapi/handler.go`：`NewHandler(Deps)` 回傳路由表；`GET /healthz` 回 `200`、`Content-Type: application/json`、body `{"status":"ok"}`
- `internal/provider/`：簡訊供應商介面與 `mock` 實作（handler 的 `Deps` 預設值；本單不接任何路由）
- `e2e/testutil/server.go`：用 `httptest.NewServer` 包真實 handler，注入假時鐘與假供應商，測試結束自動關閉
- `e2e/healthz_test.go`：`TestE2E_Healthz`

### 驗收條件
- [x] `go test ./... -v` 全綠，且輸出含 `--- PASS: TestE2E_Healthz`
- [x] `go vet ./...` 沒有輸出
- [x] `go.mod` 沒有 `require`（`go list -m all` 只列出 `example.com/sms-platform`）
- [x] 回報區填好、`status` 改成 `DONE`，產品檔與工單分開 commit，commit 訊息含 `T0001`（參考解例外：產品檔是課程維護者預先 commit 的，見回報區）

### 不做
- 不改 `_bmad/`、`.claude/`；不動 `_ct-workorders/` 裡本工單以外的檔案
- 不 push

## Sub-session 執行指示

> 在第二個 Claude Code 終端（同一個 `sms-platform/`）執行 `/ct-exec T0001`。
> Worker 先做落點檢查（C-0 比對 `repo`），再把 `status` 改成 `IN_PROGRESS`、填 `started_at`，最後依序：產品 commit → 回報區 → 狀態與完成時間 → 工單 commit。

---

## 回報區

> 以下由 Worker 填寫，塔台不在這裡編輯

### 完成狀態
DONE

### 產出摘要
- 產品檔（commit `2fa0921`）：`go.mod`、`cmd/smsd/main.go`、`internal/httpapi/{doc.go,handler.go}`、`internal/provider/{doc.go,provider.go}`、`internal/provider/mock/mock.go`、`e2e/healthz_test.go`、`e2e/testutil/{server.go,clock.go,provider.go}`
- `GET /healthz` 在 `internal/httpapi/handler.go` 的 `handleHealthz`，由 `NewHandler` 以 `mux.HandleFunc("GET /healthz", …)` 註冊（Go 1.22 起的「方法＋路徑」寫法，標準庫即可）
- 與 1-05 草稿的差異：草稿把 handler 放在 `internal/httpapi/healthz.go`；這裡放在 `handler.go`，因為之後每支 API 都掛在同一張路由表上。`e2e/testutil` 多了假時鐘（`clock.go`）與假供應商（`provider.go`），讓之後的 E2E 不受時間與網路影響
- 同一個 commit 也建了之後章節要用的空套件（`internal/{queue,ratelimit,store,template,tenant,webhook}/doc.go`，只有套件說明），不屬本單範圍，`go test` 顯示 `[no test files]`
- 落點檢查：C-0 PASS（`repo` = `sms-platform`，`basename(REPO_ROOT)` = `sms-platform`）

### 驗收證據
- `go test ./... -v`（`-count=1`，不用快取）：
  ```text
  ?   	example.com/sms-platform/cmd/smsd	[no test files]
  === RUN   TestE2E_Healthz
  --- PASS: TestE2E_Healthz (0.00s)
  PASS
  ok  	example.com/sms-platform/e2e	0.013s
  ?   	example.com/sms-platform/e2e/testutil	[no test files]
  ?   	example.com/sms-platform/internal/httpapi	[no test files]
  ```
  （其餘 8 個 `internal/*` 套件同為 `[no test files]`，結束碼 0，沒有 `FAIL`）
- `go vet ./...`：沒有輸出，結束碼 0
- `go list -m all`：只有一行 `example.com/sms-platform`；`go.mod` 沒有 `require`
- `gofmt -l .`：沒有輸出
- commit：產品檔在 `2fa0921`，與工單分開；但它是課程維護者建骨架時的 commit，訊息不含 `T0001`。你自己做時，Worker 的產品 commit 訊息會含 `T0001`（示範 repo 的實際例子：`feat(T0001): Go module 骨架與 /healthz E2E`）。本工單檔的 commit 訊息含 `T0001`

### 互動紀錄
無

### 遭遇問題
無

### 回報時間
2026-10-22T10:25:43+0800
