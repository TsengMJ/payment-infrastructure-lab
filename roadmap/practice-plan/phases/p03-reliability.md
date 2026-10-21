# P03 可靠性（13 次迭代）

- **對應模組**：#03 Gateway API（冪等）、#06 Connector（逾時、provider webhook）、架構文件第 6 節
- **階段目標**：重送、併發、逾時、當機、webhook 重複與亂序，都不會造成重複扣款、狀態倒退或事件遺失。
- **前置**：P02。
- **結束時的架構**：`internal/platform` 新增 idempotency、inbox、outbox；`tools/mockpsp` 可注入故障；worker 開始處理 inbox、outbox、recovery。

這是整個計劃最重要的階段之一。面試時「PSP 逾時之後你怎麼避免重複扣款」幾乎必問，答案的證據全部來自這裡。

---

### I-03.01 冪等紀錄與唯一約束

- **目標**：`POST /v1/payments` 支援 `Idempotency-Key`；同一商戶同一 key 的第二次請求回傳第一次的結果。
- **為什麼**：網路斷線時商戶一定會重送；沒有冪等就會建立兩筆付款。
- **不做**：payload 比對（下一次）、處理中狀態（I-03.03）。
- **先想清楚**：
  1. 冪等的範圍：商戶 + key？還是商戶 + key + 路徑？
  2. 結果要快取多久（TTL）？過期後同一個 key 再來怎麼辦？
  3. 為什麼一定要用資料庫唯一約束，不能只用 Redis？
- **完成條件**：測試：同 key 送兩次，只建立一筆 Intent，兩次回應相同。
- **常見陷阱**：先查詢有沒有紀錄、沒有再插入，兩步之間有競態。
- **設計模式**：Idempotency key
- **Survey**：Stripe idempotent requests、Brandur idempotency keys Postgres
- **產出**：`platform.idempotency_records`、ADR 0008「冪等的範圍、TTL 與儲存」

### I-03.02 Request fingerprint

- **目標**：同一 key 但不同 payload 時回 422，並寫入稽核紀錄。
- **為什麼**：同一 key 對應不同請求，通常代表商戶的程式有 bug；默默回舊結果會讓商戶以為新請求成功了。
- **不做**：其他 API 的冪等。
- **先想清楚**：
  1. fingerprint 用哪些欄位計算？JSON 欄位順序不同算不算同一個請求？
  2. 哪些欄位不應該納入（例如每次都不同的 metadata 時間戳）？
- **完成條件**：測試：同 key 同 payload 回舊結果；同 key 不同金額回 422。
- **常見陷阱**：直接對原始 body 做雜湊，空白或欄位順序不同就誤判。
- **Survey**：request fingerprint canonical JSON
- **產出**：fingerprint 函式

### I-03.03 處理中狀態與 100 次併發

- **目標**：第一個請求還在處理時，第二個同 key 請求立即回 409（或等待），不會再執行一次。
- **為什麼**：商戶重送的時間點常常就在第一個請求還沒結束時。
- **不做**：跨 API 的冪等框架。
- **先想清楚**：
  1. 處理中的請求如果行程當機，紀錄永遠卡在處理中怎麼辦？
  2. 回 409 讓商戶稍後重試，還是伺服器端等待？
  3. 哪些錯誤結果要快取（例如 400），哪些不要（例如 500）？
- **完成條件**：測試：100 個 goroutine 同 key 同 payload 併發送出，只建立一筆 Intent、最多一次 PSP 呼叫。
- **常見陷阱**：快取 500 錯誤，讓商戶永遠無法重試。
- **Survey**：idempotency in-progress lock、recovery points idempotency
- **產出**：`tests/failure/concurrent_same_key_test.go`

### I-03.04 對 PSP 的冪等

- **目標**：每次呼叫 Stripe 都帶 Attempt 級的 `Idempotency-Key`；同一個 Attempt 重送給 Stripe 不會重複扣款。
- **為什麼**：商戶層的冪等擋不住「我們自己」在逾時後重送給 PSP。
- **不做**：逾時處理（I-03.06）。
- **先想清楚**：
  1. 為什麼 PSP 冪等 key 要綁 Attempt，而不是 Intent？
  2. 授權、請款、退款各自需要不同的 key 嗎？
  3. Stripe 的冪等 key 保存多久？超過之後重送會怎樣？
- **完成條件**：測試：同一 Attempt 的授權呼叫送兩次，Stripe 只產生一筆。
- **常見陷阱**：每次重試都產生新的 key，等於沒有冪等。
- **設計模式**：Idempotency key（呼叫端產生、綁 Attempt）
- **Survey**：Stripe idempotency key retention、PSP idempotency
- **產出**：connector 呼叫的 key 規則

### I-03.05 Mock PSP 與故障注入

- **目標**：建立 Mock PSP，可控制：延遲、回 500、連線中斷、成功但不回應、重複送 webhook、亂序送 webhook、不送 webhook。
- **為什麼**：真實 PSP 的故障無法按需重現；沒有 Mock PSP，可靠性只能靠想像。
- **不做**：模擬所有 Stripe API。
- **先想清楚**：
  1. 故障要怎麼指定：每個請求帶 header、Mock 的管理 API，還是依金額觸發？
  2. Mock PSP 自己需要記住交易狀態嗎（才能被查詢）？
  3. Mock PSP 的 adapter 實作同一個 Connector 介面，並註冊進 I-02.05b 的 registry。這讓它成為第二個真正走 HTTP 的實作，可以順便驗證介面。它在哪些地方和 Stripe 的行為不同？
- **完成條件**：每種故障都有一個測試能觸發並觀察到；I-02.13 的 conformance suite 在 Mock PSP adapter 上通過。
- **常見陷阱**：Mock 的行為比真實 PSP 理想，讓測試變得無意義。
- **Survey**：fault injection、toxiproxy、chaos testing payments
- **產出**：`tools/mockpsp`、`internal/connector/mock`

### I-03.06 逾時進入 UNKNOWN

- **目標**：PSP 逾時或連線中斷時，Attempt 進入 `unknown`，保留原本的 PSP 冪等 key 與 request id；Intent 不會被標成失敗。
- **為什麼**：逾時不代表失敗，PSP 可能已經扣款。把未知當成失敗再重試，就是重複扣款最常見的原因。
- **不做**：自動查詢收斂（下一次）。
- **先想清楚**：
  1. 哪些錯誤是「確定失敗」，哪些是「不知道」？HTTP 400、402、500、503、連線重設分別屬於哪一類？
  2. Attempt 在 unknown 時，商戶查詢會看到什麼？
  3. unknown 時能不能建立新的 Attempt？
- **完成條件**：Mock PSP 注入「成功但不回應」：Attempt 為 unknown，沒有第二個 Attempt，商戶看到 processing。
- **常見陷阱**：timeout 設太長，請求執行緒全部卡住；或設太短，大量正常請求變成 unknown。
- **設計模式**：逾時即未知（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 1 節）
- **Survey**：unknown outcome payments、timeout vs failure distributed systems
- **產出**：錯誤分類、ADR 0009「逾時一律視為未知結果」

### I-03.06a Connector 的 decorator 鏈

- **目標**：把 timeout、trace span、metrics（依 provider 與錯誤分類）、日誌遮罩、對 PSP 的出站限流，做成包在 Connector 外面的 decorator；I-03.06 的逾時處理搬進 decorator 後，所有測試仍然通過。
- **為什麼**：這些橫切邏輯如果寫在每個 adapter 裡，第二家 PSP 一定會漏掉某一項。做成 decorator 之後，每個 adapter 自動擁有；P12 的 circuit breaker 也只是再加一層。
- **不做**：重試、circuit breaker（P12）。
- **先想清楚**：
  1. decorator 的順序有影響嗎？timeout 包在 metrics 裡面或外面，量到的延遲有什麼差別？
  2. 為什麼「通用重試」不能當成 decorator 隨便加上？哪些操作重試是安全的（查詢），哪些只有帶同一個冪等 key 才安全，哪些完全不行？
  3. PSP 有自己的速率限制。超過時要排隊、直接拒絕，還是降級？
  4. 日誌要記下 request 與 response 的哪些部分，才能除錯又不洩漏 token 或個資？
- **完成條件**：測試：Mock PSP 注入延遲時，Stripe 與 Mock 兩個 adapter 都在設定時間內回 Unknown；metrics 依 provider 與錯誤類別出現；adapter 程式碼裡沒有 timeout 與 metrics 的邏輯。
- **常見陷阱**：decorator 把錯誤包成一般 error，吞掉了 Unknown 與 Failed 的區別。
- **設計模式**：Decorator、Rate limiting（出站）（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 2 節）
- **Survey**：decorator pattern Go、http.RoundTripper middleware、golang.org/x/time/rate、errors.Is wrapping
- **產出**：connector decorators、每個 provider 的 metrics

### I-03.07 Recovery worker

- **目標**：worker 定期找出 unknown 的 Attempt，用原本的 reference 向 PSP 查詢，收斂成成功或失敗；超過上限次數後轉人工處理。
- **為什麼**：unknown 不能永遠停留；收斂必須用原本的 reference，而不是發新請求。
- **不做**：人工處理介面（P15）。
- **先想清楚**：
  1. 查詢的退避間隔與上限？
  2. 多個 worker 同時跑時，怎麼避免重複處理同一筆（`FOR UPDATE SKIP LOCKED`）？
  3. 查詢結果是「PSP 沒有這筆交易」時，可以安全地判定失敗嗎？要等多久？
- **完成條件**：故障場景「PSP 已扣款但回應遺失」：recovery 把 Attempt 收斂為 authorized，全程只有一筆 PSP 交易。
- **常見陷阱**：查不到就立刻判定失敗，但 PSP 只是還沒寫入。
- **設計模式**：先查再做（query-then-act）、Competing consumers（`SKIP LOCKED`）、退避重試與 jitter
- **Survey**：SKIP LOCKED job queue、exponential backoff jitter、Stripe retrieve PaymentIntent
- **產出**：worker job、`docs/evidence/psp-lost-response.md`

### I-03.08 Provider webhook 驗簽

- **目標**：`POST /v1/webhooks/stripe` 驗證簽章與時間戳；驗證失敗回 400 且不產生任何業務效果。
- **為什麼**：任何人都可以對你的 webhook URL 發請求；沒驗簽就等於讓陌生人修改付款狀態。
- **不做**：處理事件內容（下一次）。
- **先想清楚**：
  1. 驗簽為什麼要用原始 body，而不是解析後的 JSON？
  2. 時間戳容忍度要多大？怎麼防止重放？
  3. webhook secret 輪替時怎麼同時接受新舊兩個？
- **完成條件**：測試：錯誤簽章、過期時間戳、被修改的 body，全部拒絕。
- **常見陷阱**：框架先解析 JSON 再驗簽，導致 body 被改寫而驗證失敗或被繞過。
- **設計模式**：HMAC 簽章 + 時間戳（驗簽邏輯屬於各家 adapter，不放在共用 HTTP handler）
- **Survey**：Stripe webhook signature verification、HMAC SHA256、replay attack
- **產出**：`internal/connector/stripe/webhook.go`

### I-03.09 Webhook inbox：先入庫再回應

- **目標**：驗簽成功的事件先寫入 `platform.inbox`，提交後才回 200；以 provider + event id 去重。
- **為什麼**：如果先處理再回應，處理失敗或當機會讓 PSP 重送，處理一半的狀態也會不一致；先入庫就能把接收與處理分開。
- **不做**：處理事件（下一次）。
- **先想清楚**：
  1. inbox 要存整個 payload 嗎？payload 裡有沒有敏感資料？
  2. 為什麼 PSP 的 event id 去重還不夠，之後還要業務層去重？
  3. 回應 PSP 的時間要多短？
- **完成條件**：測試：同一事件送 100 次，inbox 只有一筆；資料庫寫入失敗時回 5xx，讓 PSP 重送。
- **常見陷阱**：在 webhook handler 裡同步呼叫 PSP 查詢或更新多張表。
- **設計模式**：Idempotent consumer（Inbox）
- **Survey**：transactional inbox pattern、Stripe webhook best practices
- **產出**：`internal/platform/inbox`

### I-03.10 Inbox 處理：亂序與過期事件

- **目標**：worker 處理 inbox 事件、驅動狀態機；晚到的舊事件不會讓狀態倒退。
- **為什麼**：webhook 不保證順序；「capture 成功」可能比「授權成功」先到。
- **不做**：outbox（下一次）。
- **先想清楚**：
  1. 怎麼判斷一個事件是過期的：事件時間、PSP 物件版本、還是狀態機本身？
  2. 收到「capture 成功」但我們還在 processing 時，要直接跳過去，還是先向 PSP 查詢？
  3. 處理失敗的事件要重試幾次？最後放到哪裡？
- **完成條件**：故障場景「Capture 事件比 Authorization 事件先到」：最終狀態正確，沒有非法轉移。
- **常見陷阱**：無條件用 webhook 的狀態覆蓋資料庫。
- **Survey**：out-of-order events、event versioning、Stripe event ordering
- **產出**：inbox processor、`docs/evidence/webhook-out-of-order.md`

### I-03.11 Transactional outbox

- **目標**：Intent 狀態改變時，在同一個資料庫交易裡寫入 outbox 事件（例如 `payment.captured`）。
- **為什麼**：「資料庫已提交但事件沒送出」或「事件送出但資料庫回滾」都會讓下游（ledger、商戶通知）與真實狀態不一致。
- **不做**：發送（下一次）。
- **先想清楚**：
  1. 事件 payload 要放完整狀態，還是只放 ID 讓消費者回查？
  2. 事件 ID 怎麼產生，消費者怎麼用它去重？
  3. 哪些狀態變化需要發事件？
- **完成條件**：測試：狀態更新與 outbox 寫入一起成功或一起失敗。
- **常見陷阱**：在交易提交後才另外寫 outbox。
- **設計模式**：Transactional outbox、Repository 與交易邊界
- **Survey**：transactional outbox pattern、microservices.io outbox
- **產出**：`internal/platform/outbox`、事件格式文件

### I-03.12 Outbox dispatcher 與當機測試（含階段檢核）

- **目標**：worker 用 `FOR UPDATE SKIP LOCKED` 讀取未發送的 outbox 事件，交給消費者處理，再標記已發送；在任意時間點 kill worker，事件最終都會被處理，且消費者能容忍重複。
- **為什麼**：這是「至少一次」投遞；正確性來自消費端的去重，而不是期待 exactly-once。
- **不做**：RabbitMQ（P09）。
- **先想清楚**：
  1. 發送成功但標記前當機，會發生什麼？消費者怎麼辦？
  2. 已發送的事件要保留多久？
  3. 怎麼監控 outbox 積壓？
- **完成條件**：故障場景「Dispatcher 發送後、標記前當機」與「Worker 在 ACK 前當機」：重啟後事件被處理，業務效果只有一次。
- **常見陷阱**：宣稱自己做到 exactly-once delivery。
- **Survey**：at-least-once delivery、effectively-once processing
- **產出**：dispatcher、outbox lag metric、`docs/failure-recovery.md`、`docs/notes/p03-review.md`

## 階段檢核

- **面試題**：
  1. PSP 逾時了，你怎麼避免重複扣款？
  2. 你的冪等 key 存在哪？為什麼不只用 Redis？
  3. 同一個 key 送不同的金額會怎樣？
  4. 重複和亂序的 webhook 怎麼處理？
  5. 什麼是 transactional outbox？為什麼不能直接在交易提交後發事件？
- **故障場景**（ProjectRoadmap 的 12 個場景中的 1–7、9）：同 key ×100、同 key 不同 payload、PSP 成功但回應遺失、PSP 回 500、Worker 在 ACK 前當機、同一 webhook ×100、Capture 比 Authorization 先到、Dispatcher 發送後標記前當機。
- **tag**：`p03-reliability-done`
