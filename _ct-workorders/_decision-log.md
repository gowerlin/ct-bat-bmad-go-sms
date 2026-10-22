# Decision Log

## 索引

| # | 日期 | 決策 | 相關工單 |
|---|------|------|---------|
| D001 | 2026-10-22 | 只用標準庫 `net/http`＋`httptest`，不引入第三方套件 | T0001 |

---

## 詳細紀錄（倒序）

### D001 2026-10-22 — 只用標準庫 `net/http`＋`httptest`，不引入第三方套件

- **背景**：T0001 建 sms-platform 的第一個 Go module 與 `GET /healthz`。這是之後每一章長功能的地基：HTTP 路由與測試寫法在這裡定下來，後面的工單都會沿用。Go 1.22 起標準庫 `http.ServeMux` 已支援「方法＋路徑」（`GET /healthz`）與 `{id}` 路徑參數；`net/http/httptest` 可以在本機起真實的 HTTP server 跑 E2E，不需要網路或帳號。
- **選項**：
  - [A] 只用標準庫：`net/http` 路由、`httptest` 跑 E2E、`testing` 斷言。`go.mod` 沒有 `require`，clone 下來不必下載任何東西就能 `go test`；代價是路由群組、middleware 串接要自己多寫幾行
  - [B] 第三方 router（例如 chi）＋標準庫測試：路由群組與 middleware 現成；但多一個依賴要跟版本，課程錄影後套件改版，學員畫面就會和影片不一致
  - [C] 第三方 router＋斷言套件（例如 testify）：測試寫起來較短；依賴再多一個，學員要同時認兩套 API，偏離「第一章不教 Go 語法」的目標
- **決定**：選 A，理由：骨架只有一條路由，標準庫足夠；沒有依賴，「`go.mod` 無 `require`」就能直接寫成驗收條件（`go list -m all` 只列出 `example.com/sms-platform`），離線、在 dev container 裡都能重現。標準庫真的不夠用時，再開 EXP 比較「標準庫路由 vs 第三方 router」（3-06），結論另記新的決策，不改寫本條
- **影響**：`go.mod` 不得有 `require`；`internal/httpapi` 用 `http.NewServeMux` 註冊路由；`e2e/testutil` 用 `httptest.NewServer` 包真實 handler；之後的工單要加依賴，必須在工單明示並記一條新的決策
- **相關工單**：T0001
