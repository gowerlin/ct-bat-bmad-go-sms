Feature: 送出簡訊
  租戶用自己的 API key 送出一則簡訊，平台先收下、排進佇列，之後可以查詢。
  對應驗收條件：TestE2E_SendMessage_Returns202AndQueryable

  Scenario: 送出訊息回 202 且可查詢
    Given 租戶 A 帶著有效的 API key
    When 對 /v1/messages 送出 {"to":"+886900000001","body":"hi"}
    Then 回應狀態碼是 202
    And 訊息狀態是 queued
