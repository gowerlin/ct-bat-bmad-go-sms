- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-send-and-query-message.md`
  summary: `cmd/smsd` 的 `run`（優雅關機順序）與 `purgeLoop` 沒有自動化測試。
  evidence: 審查驗證缺口：刪掉 `go purgeLoop` 或調換 `srv.Shutdown`／`q.Close` 順序，現有測試全過；補測需注入 interval／clock 並新增 `cmd/smsd/main_test.go`，可在 Story 1.6 動到組裝處時一併處理。

## Deferred from: code review of T0008-queue-send-worker-and-retry (2026-10-23)

- source_spec: `_ct-workorders/T0008-queue-send-worker-and-retry.md`
  summary: `queue` 對每次 `Provider.Send` 沒有逾時；Provider 卡住時 worker 一直被佔住，4 個都卡住就整個停止送信。逾時回 `context.DeadlineExceeded` 時目前會依 AD-1 當永久失敗，不會重試。
  evidence: 審查（blind-hunter＋edge-case-hunter）。`deliver` 用的 `sendCtx` 只有 `Close` 會取消。AD 沒規定送信逾時與逾時算暫時還是永久，需塔台決定逾時值與歸類（可能與 T0010 的 HTTP adapter 一起定）。
- source_spec: `_ct-workorders/T0008-queue-send-worker-and-retry.md`
  summary: `Send` 回成功但 `ProviderReceipt.ProviderRef` 為空時，訊息照樣轉 `sent`、`provider_ref` 留空，之後的供應商回呼對不到這則訊息。
  evidence: 審查（edge-case-hunter），未驗證嚴重度（maybe-false，若成立為 medium）。要看 T0009 回呼以 `provider_ref` 對應時，空 ref 該算永久失敗還是只記 log。
- source_spec: `_ct-workorders/T0008-queue-send-worker-and-retry.md`
  summary: `cmd/smsd` 的 queue 組裝（`WithProvider(prov)`）沒有測試，且 `go func() { _ = q.Run(ctx) }()` 丟掉 `Run` 的錯誤（例如 `ErrNoProvider`），組裝錯時伺服器照樣回 202 卻永遠不送、也沒有 log。
  evidence: 審查（verification-gap）：改回 `queue.NewInProcess(st)` 時全部測試仍過。補測需新增 `cmd/smsd/main_test.go`，加 log 要動 `main.go`；T0008 的 `main.go` 依 D012 只許傳 `prov`，兩者都不在範圍。與上方 spec-1-1 那筆「`run`／`purgeLoop` 沒有自動化測試」相關。
