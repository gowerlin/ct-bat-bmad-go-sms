- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-send-and-query-message.md`
  summary: `cmd/smsd` 的 `run`（優雅關機順序）與 `purgeLoop` 沒有自動化測試。
  evidence: 審查驗證缺口：刪掉 `go purgeLoop` 或調換 `srv.Shutdown`／`q.Close` 順序，現有測試全過；補測需注入 interval／clock 並新增 `cmd/smsd/main_test.go`，可在 Story 1.6 動到組裝處時一併處理。
