# P04 退款與取消（6 次迭代）

- **對應模組**：#09 Refund / Void / Dispute 的第一部分（Dispute 在 P14）
- **階段目標**：全額與部分退款正確、可冪等、不會超退；退款的非同步與未知結果能收斂。
- **前置**：P03（冪等、inbox、recovery 都會重用）。
- **結束時的架構**：`refund` 模組（`refund` schema），透過 payment 的公開介面讀取可退款金額，退款狀態改變寫 outbox。

---

### I-04.01 Refund 模型與前置條件

- **目標**：建立 `refund.refunds`；定義哪些付款狀態可以退款，並把 Void 與 Refund 的適用時機寫成規則。
- **為什麼**：未請款的授權應該 Void，已請款才能 Refund；用錯會讓持卡人看到奇怪的帳單或讓退款失敗。
- **不做**：呼叫 PSP。
- **先想清楚**：
  1. Refund 關聯到 Intent 還是 Capture？部分請款、多次請款時差別是什麼？
  2. 為什麼把 Refund 放在獨立模組，而不是 payment 模組裡？代價是什麼？
  3. 商戶對未請款的付款呼叫退款時，要自動改成 Void，還是回錯誤？
- **完成條件**：狀態機測試：只有已請款的付款可以退款；退款狀態的合法轉移表。
- **常見陷阱**：退款成功後把 Intent 狀態改成 refunded，混淆付款與退款兩個生命週期。
- **Survey**：Stripe refunds、void vs refund
- **產出**：migration、ADR 0010「Refund 的歸屬模組與關聯對象」

### I-04.02 全額退款

- **目標**：`POST /v1/payments/{id}/refunds` 全額退款，經 Stripe 執行，保存 PSP refund id。
- **為什麼**：最基本的路徑先打通，下一次才處理部分退款的累計問題。
- **不做**：部分退款。
- **先想清楚**：
  1. 先寫 pending 紀錄、提交、再呼叫 PSP，還是先呼叫 PSP？為什麼？
  2. 退款請求的 PSP 冪等 key 怎麼產生？
- **完成條件**：sandbox 測試成功；資料庫有 PSP refund id；outbox 產生 `refund.created`。
- **常見陷阱**：在資料庫交易內呼叫 PSP。
- **Survey**：Stripe create refund
- **產出**：refund API、connector 的 Refund 實作

### I-04.03 部分退款與累計上限

- **目標**：支援多次部分退款；累計退款不超過已請款金額，即使多個退款請求同時到達。
- **為什麼**：併發退款是常見的超額退款來源，錢會直接流失。
- **不做**：跨幣別退款（P14）。
- **先想清楚**：
  1. 累計上限要在哪裡檢查：應用程式、資料庫約束、還是鎖？
  2. pending 中的退款要不要算進已用額度？失敗的退款呢？
  3. PSP 本身也會擋超額退款，那我們為什麼還要檢查？
- **完成條件**：測試：請款 100，併發送出 10 筆 20 的退款，成功的總額不超過 100。
- **常見陷阱**：只算 succeeded 的退款，pending 的被忽略，導致超退。
- **Survey**：SELECT FOR UPDATE、concurrent refunds
- **產出**：鎖策略、`tests/failure/concurrent_refunds_test.go`

### I-04.04 退款冪等

- **目標**：退款 API 支援商戶的 Idempotency-Key；對 PSP 使用 refund 級的冪等 key。
- **為什麼**：重送退款和重送付款一樣危險，而且退出去的錢更難追回。
- **不做**：新的冪等機制（重用 P03）。
- **先想清楚**：
  1. 商戶沒帶冪等 key 時，同一個金額的兩次退款要視為兩筆嗎？
  2. P03 的冪等元件能直接套用嗎？需要調整什麼？
- **完成條件**：測試：同 key 送 50 次只產生一筆退款。
- **常見陷阱**：把「同金額」當成重複，誤擋了商戶刻意的兩次退款。
- **Survey**：idempotency key per operation
- **產出**：冪等元件套用到 refund

### I-04.05 退款的非同步與未知結果

- **目標**：退款可能是 pending；逾時進 unknown；recovery worker 能收斂退款狀態。
- **為什麼**：某些支付方式的退款需要幾天；PSP 也可能在回應前斷線。
- **不做**：人工處理介面。
- **先想清楚**：
  1. 退款「成功後又失敗」是可能的嗎？要怎麼表示？
  2. unknown 的退款要不要佔用可退款額度？
  3. 可以重用 I-03.07 的 recovery worker 嗎？要怎麼抽象？
- **完成條件**：Mock PSP 注入逾時：退款進 unknown，recovery 收斂；期間不會產生第二筆退款。
- **常見陷阱**：退款逾時就重送一筆新的。
- **Survey**：Stripe refund failed after succeeded、refund status
- **產出**：recovery 擴充

### I-04.06 退款 webhook 與狀態收斂（含階段檢核）

- **目標**：處理 Stripe 的退款事件，透過 inbox 驅動退款狀態；與 API 回應、recovery 的結果互不衝突。
- **為什麼**：同一個退款的狀態可能從三個地方來（API 回應、webhook、recovery 查詢），必須收斂到同一個結果。
- **不做**：Ledger 分錄（P05 會補上）。
- **先想清楚**：
  1. 三個來源結果不一致時，以誰為準？
  2. 收到一個我們沒有紀錄的退款事件（例如商戶直接在 Stripe 後台退款）時怎麼辦？
- **完成條件**：測試：webhook 先到、API 回應後到，最終狀態一致；未知退款事件被記錄並告警。
- **常見陷阱**：忽略「在 PSP 後台直接操作」的情況，這會在對帳時變成 orphan refund。
- **Survey**：Stripe charge.refunded、refund.updated events
- **產出**：refund webhook 處理、`docs/notes/p04-review.md`

## 階段檢核

- **面試題**：
  1. Void 與 Refund 的差別？什麼時候用哪一個？
  2. 你怎麼防止併發的部分退款超過請款金額？
  3. 退款逾時了怎麼辦？
  4. 有人直接在 PSP 後台退款，你的系統會發生什麼？
- **故障場景**：併發部分退款、退款逾時、未知退款事件。
- **tag**：`p04-refund-done`
