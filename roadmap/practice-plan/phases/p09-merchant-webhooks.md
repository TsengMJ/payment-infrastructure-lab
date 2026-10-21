# P09 商戶出站 Webhook（8 次迭代）

- **對應模組**：#16 Webhook & Notification
- **階段目標**：平台把付款、退款、出款、入駐的狀態變化，以簽章、可重試、可重放的方式通知商戶；這裡第一次引入 RabbitMQ。
- **前置**：P03（outbox）、P08（入駐事件）。
- **結束時的架構**：`webhook` 模組（`webhook` schema：endpoints、deliveries、attempts）；outbox dispatcher 改發到 RabbitMQ；webhook delivery consumer 在 worker 裡執行；compose 新增 RabbitMQ 與 Mailpit。

注意區分：P03 的 provider webhook 是 **PSP 打給我們**，信任邊界是「驗證對方」；這裡的 merchant webhook 是 **我們打給商戶**，信任邊界是「讓商戶能驗證我們」，並且要可靠投遞。

---

### I-09.01 事件目錄與 schema 版本

- **目標**：定義對外事件目錄（例如 `payment.succeeded`、`refund.failed`、`payout.paid`、`account.updated`），每個事件有版本與穩定的 payload 結構。
- **為什麼**：事件是公開契約；改了欄位，商戶的程式就壞了。
- **不做**：投遞。
- **先想清楚**：
  1. 內部 outbox 事件與對外事件是同一個格式嗎？為什麼最好分開？
  2. payload 帶完整物件快照，還是只帶 ID 讓商戶回查？
  3. 新增欄位、移除欄位、改型別，哪些需要新版本？
- **完成條件**：`docs/api/events.md`；每個事件有 JSON Schema 與範例；契約測試確保 payload 符合 schema。
- **常見陷阱**：直接把內部資料庫列序列化送出去。
- **Survey**：Stripe event object、CloudEvents、event schema versioning
- **產出**：事件目錄、ADR 0019「內部事件與對外事件分離」

### I-09.02 Endpoint 訂閱管理

- **目標**：商戶可以建立、修改、停用 webhook endpoint，並選擇訂閱哪些事件；建立時驗證 URL。
- **為什麼**：商戶要能控制收什麼、送到哪；平台也要避免被當成 SSRF 跳板。
- **不做**：投遞。
- **先想清楚**：
  1. 允許的 URL 有哪些限制（https、不可指向內網、localhost）？
  2. 一個商戶可以有幾個 endpoint？同一事件送多個 endpoint 嗎？
- **完成條件**：測試：內網 IP、非 https URL 被拒；只訂閱的事件會被投遞。
- **常見陷阱**：只在建立時檢查 URL，DNS 之後改指向內網（DNS rebinding）。
- **Survey**：SSRF webhook、DNS rebinding
- **產出**：endpoint API

### I-09.03 簽章與 secret 輪替

- **目標**：每次投遞帶有 HMAC 簽章與時間戳；每個 endpoint 有自己的 secret，可輪替，輪替期間兩個簽章並存。
- **為什麼**：商戶要能確認通知真的來自平台，而且沒有被重放。
- **不做**：mTLS。
- **先想清楚**：
  1. 簽章包含哪些內容（時間戳 + body）？
  2. 輪替期間怎麼讓商戶同時驗證新舊簽章？
  3. secret 在資料庫怎麼保存？
- **完成條件**：提供一個商戶端驗證範例程式；測試證明篡改 body 或時間戳都驗證失敗。
- **常見陷阱**：所有商戶共用一個 secret。
- **Survey**：Standard Webhooks spec、Stripe signature scheme
- **產出**：簽章元件、`examples/webhook-receiver`

### I-09.04 引入 RabbitMQ

- **目標**：outbox dispatcher 改為把事件發布到 RabbitMQ exchange；webhook delivery 與 ledger consumer 各自有 queue。
- **為什麼**：多個消費者、延遲重試、DLQ 是 broker 擅長的事；但 outbox 仍然是事件可靠性的來源。
- **不做**：移除 DB 輪詢版本的程式（保留比較）。
- **先想清楚**：
  1. publisher confirm 是什麼？dispatcher 要等 confirm 才標記已發送嗎？
  2. exchange 類型（topic、direct）與 routing key 怎麼設計？
  3. RabbitMQ 掛掉時 outbox 會怎樣？
- **完成條件**：故障場景「Payment 成功寫入，但 broker 暫時故障」：broker 恢復後事件補送，業務效果只有一次。
- **常見陷阱**：以為有了 broker 就不需要 outbox。
- **Survey**：RabbitMQ publisher confirms、consumer acknowledgements、quorum queues
- **產出**：ADR 0020「DB 輪詢改為 RabbitMQ 的取捨」、`docs/evidence/broker-outage.md`

### I-09.05 投遞 worker 與投遞紀錄

- **目標**：consumer 把事件 POST 到商戶 endpoint，記錄每次嘗試的時間、狀態碼、延遲、回應摘要。
- **為什麼**：商戶說「沒收到」時，你要能拿出證據。
- **不做**：重試（下一次）。
- **先想清楚**：
  1. 哪些回應算成功（2xx）？3xx 要跟隨嗎？
  2. 逾時要設多短，才不會被慢的商戶拖垮？
  3. 回應 body 要存多少？
- **完成條件**：測試：成功、4xx、5xx、逾時四種結果都被正確記錄。
- **常見陷阱**：投遞時跟隨重新導向到內網。
- **設計模式**：Competing consumers、Bulkhead（一個慢的商戶端點不能佔滿所有投遞 worker）
- **Survey**：webhook delivery logs、http client timeout Go
- **產出**：delivery consumer、deliveries / attempts 表

### I-09.06 退避重試與 DLQ

- **目標**：失敗的投遞以指數退避重試（例如最長 3 天）；超過上限進入 DLQ 並通知商戶。
- **為什麼**：商戶端短暫故障不應造成事件永久遺失。
- **不做**：順序保證（寫筆記說明為什麼不保證）。
- **先想清楚**：
  1. 用 RabbitMQ 的 DLX + TTL 做延遲，還是用資料庫排程？
  2. 某個 endpoint 一直失敗，要不要暫停它以保護系統？
  3. 為什麼 webhook 通常不保證順序？商戶應該怎麼處理？
- **完成條件**：測試：endpoint 前 3 次失敗、第 4 次成功時，重試間隔符合設計；永遠失敗時最終進入 DLQ。
- **常見陷阱**：一個慢的商戶塞住所有商戶的投遞。
- **設計模式**：退避重試與 jitter、DLQ 與 replay
- **Survey**：RabbitMQ dead letter exchange、delayed retry、per-tenant fairness
- **產出**：重試策略、DLQ

### I-09.07 Replay 與手動重送

- **目標**：商戶可以查詢投遞紀錄，並對單一事件或某段時間內的事件要求重送；連續失敗的 endpoint 會被自動停用。
- **為什麼**：商戶修好自己的系統後，需要補收錯過的事件。
- **不做**：UI（P15）。
- **先想清楚**：
  1. 重送是用原本的事件 ID 還是新的？商戶怎麼去重？
  2. 大量 replay 時怎麼限流？
- **完成條件**：測試：重送的事件 ID 與原本相同；replay 一段時間只送出該範圍內的事件。
- **常見陷阱**：replay 產生新的事件 ID，商戶重複處理。
- **Survey**：webhook replay、event redelivery
- **產出**：replay API

### I-09.08 通知郵件與接收端範例（含階段檢核）

- **目標**：endpoint 被停用、出款失敗等重要事件時寄信給商戶（本機用 Mailpit）；完善商戶接收端範例（驗簽、去重、快速回應）。
- **為什麼**：webhook 失敗時，商戶需要另一個管道知道；範例程式能減少商戶整合錯誤。
- **不做**：真實寄信服務。
- **先想清楚**：
  1. 郵件寄送也是外部依賴，要不要走 adapter 與 outbox？
  2. 郵件內容能包含哪些資訊？
- **完成條件**：endpoint 停用時 Mailpit 收到信；接收端範例有測試。
- **常見陷阱**：在交易內同步寄信。
- **Survey**：Mailpit、transactional email outbox
- **產出**：email adapter、`docs/notes/p09-review.md`

## 階段檢核

- **面試題**：
  1. 商戶說沒收到 webhook，你怎麼查？
  2. 商戶怎麼確認 webhook 真的來自你？
  3. 為什麼有了 RabbitMQ 還需要 outbox？
  4. 一個商戶的 endpoint 一直很慢，會影響其他商戶嗎？
  5. Provider webhook 與 merchant webhook 的差別？
- **故障場景**：broker 暫時故障；商戶 endpoint 持續 500；慢 endpoint；replay。
- **tag**：`p09-webhooks-done`
