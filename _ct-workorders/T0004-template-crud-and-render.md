---
schema_version: 1
schema_kind: workorder
id: T0004
title: 模板 CRUD 與 {{var}} 渲染（story id "5"）
type: impl
status: DONE
started_at: "2026-10-22T23:35:06+0800"
updated_at: "2026-10-22T23:44:02+0800"
completed_at: "2026-10-22T23:44:02+0800"
repo: sms-platform
project: sms-platform
created_at: "2026-10-22T23:06:36+0800"
sizing: L
depends_on:
  - T9003
followups:
  - T0005
  - T0006
  - T0007
affects_files:
  - internal/template/template.go
  - internal/template/template_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/httpapi/deps.go
  - internal/httpapi/templates.go
  - internal/httpapi/errors_templates.go
  - internal/httpapi/messages_send.go
  - internal/httpapi/httpapi_test.go
  - cmd/smsd/main.go
  - e2e/testutil/server.go
  - e2e/template_test.go
  - docs/openapi.yaml
  - _ct-workorders/T0004-template-crud-and-render.md
interaction:
  mode_hint: ask
  interactive: false
  intervention_type: context-dependent
renew_count: 0
worker_question_timeout: 30
workdir: main repo
---
# 工單 T0004-template-crud-and-render

## 元資料
- **工單編號**：T0004
- **任務名稱**：模板 CRUD 與 `{{var}}` 渲染（`stories.yaml` id "5"）
- **狀態**：DONE
- **類型**：impl
- **建立時間**：2026-10-22 23:06 (UTC+8)
- **intervention_type**：context-dependent（4-03 用 ask 模式派，使用者看完回報再收單）
- **對應 E2E**：`TestE2E_Template_RendersVariables`（以 e2e 測試名對應 story；epics 編號不同，不要用）
- **depends_on**：T9003（httpapi 已拆成每功能一檔，stub 在 `templates.go`、`errors_templates.go`）
- **affects_files**：見 frontmatter

## 任務指令

### 前置條件
- 讀 `AGENTS.md`、`_ct-workorders/_decision-log.md`（D001、D002、D003、D004）
- 讀 `_bmad-output/specs/spec-sms-platform/stories.yaml` id "5"，spine 的 AD-4、AD-8、AD-10、AD-16（`.../ARCHITECTURE-SPINE.md`），`internal/template/doc.go`，`internal/httpapi/doc.go`（新增功能怎麼接），`docs/README.md` 的 Template 資料模型
- 先跑 `make test`（Windows 需 `CGO_ENABLED=1`），確認開工前全綠

### 輸入上下文
T9003 之後每個功能一個檔：模板路由填 `templates.go` 的 `registerTemplates`（每條路由用 `s.v1(s.handler(...))`），模板的錯誤對應填 `errors_templates.go` 的 `templateError`，`Deps` 在 `deps.go` 加 `Templates`。`template` 套件只回 sentinel error，不寫 HTTP 狀態碼（AD-8）；模板屬租戶，所有查詢帶 `tenantID`，跨租戶回 404（AD-4）。

要做的事：
1. `template` 套件：`Template` 實體（`id` 前綴 `tpl_`、`tenant_id`、`name`、`body`、`created_at`）、`{{var}}` 語法解析與 `Render(body, vars)`；匯出 `ErrMissingVariable`（內含缺少的變數名清單）與 `ErrInvalidTemplate`。時間與 ID 依 AD-10 注入／`crypto/rand`
2. `store`：租戶範圍的模板儲存（in-memory）
3. HTTP：依 `docs/openapi.yaml` 已列的四條路由 `POST /v1/templates`（201）、`GET /v1/templates`、`GET /v1/templates/{id}`、`DELETE /v1/templates/{id}`（204）；不做更新（PUT）。要補 `openapi.yaml` 這四條的 TODO 與 `SendMessageRequest` 的 `template_id`、`vars`
4. `POST /v1/messages` 接受 `template_id`＋`vars`：先 `template.Render`，成功才 `queue.Submit`（AD-16）；渲染結果與直接內容走同一個 `validateMessage`（70 code point）。`template_id` 與直接 `body` 同時給或都沒給，回 400 `invalid_request`（訊息不回顯內容）
5. `ErrInvalidTemplate` → 400 `invalid_request`；模板不存在 → 404 `not_found`
6. 組裝：`cmd/smsd/main.go` 與 `e2e/testutil/server.go` 都要傳入 `Templates`

### 本單與 T0007 的分工
- 缺變數時 `Render` 必須回 `ErrMissingVariable`（含變數名清單），但**本單不做 422 對應、不寫 422 的 E2E**：該對應與 `TestE2E_Template_MissingVariableReturns422` 屬 T0007。T0007 送出前，缺變數的請求在 HTTP 上會是 500，這是已知暫態，回報區寫明即可。

### context-dependent 的提問規則
遇到工單與程式碼、spine 不一致，或 `template_id`／`vars` 的 JSON 形狀（例如 `vars` 的值只收字串嗎）無法由 spine 與 openapi 判斷時，可向使用者提問**一次**，之後自行完成。

### 預期產出
- 上列 `affects_files` 內的實作與測試
- `e2e/template_test.go`：`TestE2E_Template_RendersVariables`——建立模板、帶 `template_id`＋`vars` 送信、查回訊息內容已渲染（用 `FakeProvider` 收到的內容驗證）、跨租戶讀模板回 404；不連網、不睡真實時間

### 驗收條件
- [ ] `go test ./... -race` 全綠（貼最後 20 行輸出；`-race` 無法執行時寫明原因與替代環境）
- [ ] `go vet ./...` 沒有輸出
- [ ] `gofmt -l .` 沒有輸出
- [ ] `git diff --name-only <base>..HEAD` 只出現本工單 `affects_files` 列出的檔案（`<base>` = 本工單開工時的 HEAD；若同時有別張工單在同一個工作目錄 commit，改用 `git show --name-only <本工單的每個 commit>` 逐個核對，並在回報區註明）
- [ ] 回報區填好、`status` 與 body 鏡像一致；產品 commit 與工單 commit 訊息都含工單編號
- [ ] `go test ./e2e -run 'TestE2E_Template_RendersVariables' -v` 通過
- [ ] 渲染結果與直接內容走同一個 `validateMessage`；模板儲存每個查詢都帶 `tenantID`
- [ ] `ErrMissingVariable` 含變數名清單，但本單沒有 422 對應

### 不做
- 不新增 `go.mod` 的 `require`（D001）
- 不改另一張工單的 `affects_files`；發現需要改，寫進回報區〈遭遇問題〉交回塔台
- 不改 `_ct-workorders/` 底下的塔台私有檔（`_tower-state.md`、`_decision-log.md`…）
- 不 push；不改 `_bmad/`、`.claude/`；不用 `t.Skip`、不刪測試、不放寬斷言
- 不改 `e2e/architecture_test.go`（若它因新檔失敗，先回報，不要放寬）
- 不做 422 `missing_template_variable` 的對應與 E2E（T0007）、限流（T0005）、列表（T0006）
- 不做模板更新（PUT）、模板版本、條件式或迴圈語法

## Sub-session 執行指示

> 在 `sms-platform/` 執行 `/ct-exec T0004`。
> 時間值一律用系統指令取得（`date "+%Y-%m-%dT%H:%M:%S%z"`），不要憑印象填。驗證指令成功跑完、看到輸出之後，才在回報區寫「通過」。暫存腳本寫在 `_bmad-output/tmp/`。

---

## 回報區

> 以下由 sub-session 填寫

### 待塔台決策

#### Q-1：E2E 要「用 FakeProvider 收到的內容驗證」已渲染，但現在 queue 沒有送信 worker（`queue.Run` 只等 ctx，Story 1.6 才有），`testutil` 也沒把 FakeProvider 接進 queue，FakeProvider 永遠收不到訊息；改用什麼驗證？
- 提問時間：2026-10-22T23:36:36+0800
- 選項：[A] 改從 `env.Store.Get(testutil.TenantA, id)` 讀存下的 `Body` 驗證已渲染（不改 affects_files 外的檔），並斷言 FakeProvider 沒收到未渲染內容；FakeProvider 驗證待 queue worker 那張單補 / [B] 在 `GET /v1/messages/{id}` 回應加 `body` 欄位再用 HTTP 驗證（要改 `internal/httpapi/messages.go`，不在 affects_files，且屬 API 契約變更）/ [C] 本單補 queue 送信 worker 並把 FakeProvider 接進 queue（要改 `internal/queue/*`，超出範圍）/ [D] 其他：________
- Worker 建議：[A]，因為不擴大範圍、仍是 E2E（真實 handler＋httptest）且確實驗到「入佇列前已渲染」（AD-16）；B、C 都要改本單 affects_files 以外的檔
- 影響範圍：`e2e/template_test.go` 的斷言方式；驗收條件「用 FakeProvider 收到的內容驗證」
- 阻塞：否（先做 template／store／httpapi，E2E 最後寫）

#### Q-2：`ErrInvalidTemplate` 的錯誤碼，工單寫 400 `invalid_request`，但 `docs/error-codes.md` 與 SPEC／`_bmad-output/specs/.../error-codes.md` 都寫 400 `invalid_template`（「錯誤碼變更＝相容性變更」）；用哪個？
- 提問時間：2026-10-22T23:36:36+0800
- 選項：[A] 照已文件化的契約回 400 `invalid_template` / [B] 照工單回 400 `invalid_request`，並在回報區記下與 `docs/error-codes.md` 的差異交塔台（`docs/error-codes.md` 不在 affects_files，本單不改它）/ [C] 其他：________
- Worker 建議：[A]，因為 `docs/error-codes.md` 是對外契約且 SPEC 一致，工單的 `invalid_request` 看起來是筆誤；T0007 只要求「保留 T0004 的 ErrInvalidTemplate → 400 對應」，兩者都相容
- 影響範圍：`internal/httpapi/errors_templates.go`、`docs/openapi.yaml` 的 400 描述、`internal/httpapi/httpapi_test.go`
- 阻塞：否（最後填對應即可）

#### Q-3：`vars` 的 JSON 形狀 spine 與 openapi 都沒定；是否定為「物件，值只收字串」？
- 提問時間：2026-10-22T23:36:36+0800
- 選項：[A] `vars` 為 `object`，`additionalProperties: {type: string}`；值不是字串（數字、布林、null、巢狀）回 400 `invalid_request`，訊息不回顯內容 / [B] 收字串、數字、布林，伺服器端轉成字串 / [C] 其他：________
- Worker 建議：[A]，因為最簡單、無隱性轉換（如 `1.0` → `"1"`），學員用 `curl` 也清楚；之後要放寬可相容地擴充
- 影響範圍：`internal/httpapi/messages_send.go` 的請求 DTO、`docs/openapi.yaml` 的 `SendMessageRequest`
- 阻塞：否

### 完成狀態
DONE

### 產出摘要
- **template**（`internal/template/template.go`）：`Template` 實體（`id` `tpl_`＋128 位元 hex，經 `store.NewID`／`crypto/rand`；`created_at` 由 store 以注入的 `Now` 蓋章，AD-10）；`{{var}}` 解析（名稱 `[\p{L}_][\p{L}\p{N}_]*`，括號內前後空白忽略；`{{` 無 `}}`、`}}` 無 `{{`、空名稱、不合法字元 → `ErrInvalidTemplate`）；`Validate`、`Render(body, vars)`（多餘變數忽略，值不再解析）；`ErrMissingVariable` sentinel＋`*MissingVariableError{Names}`（依首次出現排序、去重，`errors.Is`／`errors.As` 都可用）；`Service`（Create／Get／List／Delete，只持有 `store.TenantScoped`，AD-4）
- **store**（`internal/store/store.go`）：`Template` 紀錄；`TenantScoped` 加 `CreateTemplate`／`GetTemplate`／`ListTemplates`／`DeleteTemplate`，每個都以 `tenantID` 為第一參數、空字串回 `ErrEmptyTenant`、他租戶回 `ErrNotFound`；列表新到舊（同時間 id 大者先）；加鎖，`-race` 無 data race
- **httpapi**：`templates.go` 四條路由（`POST /v1/templates` 201、`GET /v1/templates` 200 `{"templates":[…]}`、`GET /v1/templates/{id}`、`DELETE /v1/templates/{id}` 204），皆 `s.v1(s.handler(...))`；`Templates` 介面放 `templates.go`（同 `Authenticator` 放 `auth.go` 的寫法），`Deps.Templates` 在 `deps.go`；`errors_templates.go` 的 `templateError`：`ErrInvalidTemplate` → 400 `invalid_template`（Q-2）；`messages_send.go`：`template_id`＋`vars`，`body` 與 `template_id` 同時給或都沒給 → 400 `invalid_request`，先 `Templates.Get`（不存在／他租戶 404）→ `template.Render` → 同一個 `validateMessage`（70 code point）→ `queue.Submit`（AD-16）
- **組裝**：`cmd/smsd/main.go`、`e2e/testutil/server.go` 傳入 `Templates: template.NewService(st)`；`registerTemplates` 在 `Templates` 為 nil 時 panic（啟動即失敗）
- **docs/openapi.yaml**：四條模板路由的 request／response、`TemplateRequest`／`Template`／`TemplateList` schema、`InvalidTemplate` 400 回應；`SendMessageRequest` 改 `required: [to]` 並加 `template_id`、`vars`（`additionalProperties: {type: string}`）；`POST /v1/messages` 加 404
- **測試**：`template_test.go`（渲染、缺變數清單、10 種不合法語法、Service 跨租戶）、`store_test.go`（租戶範圍、排序、並行）、`httpapi_test.go`（`Templates` 必填、`invalid_template` 信封）、`e2e/template_test.go`（`TestE2E_Template_RendersVariables`、`TestE2E_Template_RejectsInvalidRequests`）

### 驗收證據
- 環境：Windows 主機，Go 1.27.3，MinGW gcc（`C:\ProgramData\mingw64\mingw64\bin`），`CGO_ENABLED=1`
- 開工前 `make test`：全綠（PATH 未含 gcc 時第一次執行 `cgo: C compiler "gcc" not found`，把 MinGW 加進 PATH 後重跑全綠）
- `go test ./... -race -count=1`（最後 20 行，exit 0）：
  ```
  ?   	example.com/sms-platform/cmd/smsd	[no test files]
  ok  	example.com/sms-platform/e2e	1.362s
  ?   	example.com/sms-platform/e2e/testutil	[no test files]
  ok  	example.com/sms-platform/internal/httpapi	1.042s
  ?   	example.com/sms-platform/internal/provider	[no test files]
  ok  	example.com/sms-platform/internal/provider/mock	1.074s
  ok  	example.com/sms-platform/internal/queue	1.027s
  ?   	example.com/sms-platform/internal/ratelimit	[no test files]
  ok  	example.com/sms-platform/internal/store	1.028s
  ok  	example.com/sms-platform/internal/template	1.027s
  ok  	example.com/sms-platform/internal/tenant	1.027s
  ?   	example.com/sms-platform/internal/webhook	[no test files]
  ```
  （`make test` 同樣 exit 0）
- `go test ./e2e -run 'TestE2E_Template_RendersVariables' -v`：`--- PASS: TestE2E_Template_RendersVariables`、`ok example.com/sms-platform/e2e`
- `go build ./...`、`go vet ./...`、`gofmt -l .`：都沒有輸出，exit 0
- `go list -m all`：只有 `example.com/sms-platform`（D001）
- `docs/openapi.yaml`：以 pyyaml 解析成功，paths 含 `/v1/templates`、`/v1/templates/{id}`
- `git diff --name-only 2e34a93..HEAD`（`2e34a93` = 開工時 HEAD；期間沒有別張工單在本目錄 commit）：只有 frontmatter `affects_files` 列出的 14 個檔
- 渲染與直接內容同一個 `validateMessage`：`messages_send.go` 只有一處呼叫，在兩條路徑匯合之後；E2E「rendered over 70」「rendered blank」回 400 且不建立訊息
- 模板查詢都帶 `tenantID`：`TenantScoped` 四個模板方法簽章第一參數 `tenantID`；E2E 驗證 B 讀／用／列／刪 A 的模板都 404 或看不到
- `ErrMissingVariable` 帶變數名清單（`TestRenderMissingVariablesListsNames`）；`templateError` 沒有 422 對應

### 落地檢查
- 結果：PASS
- C-0：frontmatter `repo` = `sms-platform`，`basename(REPO_ROOT)` = `sms-platform` → PASS（`REPO_ROOT` = `<demo>/sms-platform`）
- C-1：工單在 `REPO_ROOT/_ct-workorders/` 下 → PASS
- C-3：前 5 個 `affects_files` 都在（或祖先在）→ PASS（資訊性）
- C-2：工單沒有 `branch` 欄位，不適用；目前分支 `main`
- `CT_HOME` 未設（非 worktree）；`CT_MODE=yolo`、`CT_INTERACTIVE=0`；`BAT_WORKSPACE_ID` = `<workspace>`（僅記錄）

### 產品 commit
- `9c7e7aa` feat(T0004): 模板 CRUD 與 {{var}} 渲染
- 工單 commit：`8bd7144`（開工＋提問）；收尾 commit 見 git log（訊息含 T0004）

### 互動紀錄
- [23:38] Q-1: queue 沒有送信 worker，FakeProvider 收不到訊息，E2E 改用什麼驗證已渲染 → A（塔台，BAT 通道，choice 1）: [A] 改從 `env.Store.Get(testutil.TenantA, id)` 讀存下的 Body 驗證，並斷言 FakeProvider 沒收到未渲染內容；「用 FakeProvider 收到的內容驗證」留給 queue 送信 worker 那張單補，回報區寫明差異 → Action: E2E 以 store 讀回的 Body 驗證，另斷言 FakeProvider 沒收到含 `{{` 的內容
- [23:38] Q-2: `ErrInvalidTemplate` 回 `invalid_request`（工單）還是 `invalid_template`（`docs/error-codes.md`／SPEC） → A（塔台，BAT 通道，choice 1）: [A] 400 `invalid_template`，工單寫 `invalid_request` 是塔台筆誤；openapi 的 400 描述照 `invalid_template` → Action: `templateError` 對應 400 `invalid_template`
- [23:38] Q-3: `vars` 的 JSON 形狀 → A（塔台，BAT 通道，choice 1）: [A] object、值只收 string，非字串值回 400 `invalid_request`，訊息不回顯內容 → Action: DTO 用 `map[string]*string`（`null` 也能擋下），型別錯誤與 `null` 都回固定訊息 `field 'vars' must be an object of string values` 的 400

### Renew 歷程
無

### 遭遇問題
1. **與工單的差異（塔台已決，Q-1）**：E2E 沒有用 `FakeProvider` 收到的內容驗證渲染結果。`queue.Run` 目前只等 ctx（送信 worker 屬 Story 1.6），`testutil` 也沒把 `FakeProvider` 接進 queue，所以 provider 收不到任何訊息。改以 `env.Store.Get(testutil.TenantA, id).Body` 驗證已渲染，另斷言 `FakeProvider.Sent()` 沒有含 `{{` 的內容。**待補**：queue 送信 worker 那張單應把這個 E2E 改成用 `FakeProvider` 驗證。
2. **與工單的差異（塔台已決，Q-2）**：`ErrInvalidTemplate` 回 400 `invalid_template`（照 `docs/error-codes.md`／SPEC），不是工單寫的 `invalid_request`。
3. **已知暫態（照工單）**：缺變數時 `template.Render` 回 `ErrMissingVariable`，但本單沒有 422 對應，HTTP 上目前回 500 `internal_error`（固定訊息，`slog` 只記錯誤型別），且不建立訊息。T0007 補 422。給 T0007：`errors.As(err, &mv)` 取 `*template.MissingVariableError` 的 `Names`；本單沒有寫「缺變數＝500」的測試，T0007 加 422 對應時不會弄壞既有測試。
4. **`handler.go` 不在 affects_files**：`NewHandler` 開頭的 nil 檢查與 panic 訊息仍只列 Queue／Store／Tenants；`Templates` 的檢查放在 `registerTemplates`（panic `httpapi: Deps.Templates is required`），`deps.go` 註解已改為四個必填。日後有人改 `handler.go` 時可把檢查併過去。
5. **沒做（範圍外）**：`Message` 沒存 `template_id`／`vars`（`docs/README.md` 資料模型有列為選填欄位；要改 `queue.Draft`，不在本單範圍）；`validate.go` 註解「(later) rendered templates」的 later 已過時，該檔不在 affects_files，沒改。
6. **環境**：Windows 主機 PATH 預設沒有 gcc，`-race` 要先把 `C:\ProgramData\mingw64\mingw64\bin` 加進 PATH（AGENTS.md 已寫要裝 MinGW，沒寫路徑）。

### sprint-status.yaml 已更新
不適用

### 回報時間
2026-10-22T23:44:02+0800
