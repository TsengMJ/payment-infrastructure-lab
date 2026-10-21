# P13 Billing（11 次迭代）

- **對應模組**：#10 Recurring Payments & Billing、#07（已存支付方式）
- **階段目標**：商戶可以建立方案與訂閱；系統依週期產生發票並以已存的支付方式在持卡人不在場時扣款；失敗時重試並進入催收（dunning）。
- **前置**：P09（事件）、P11（token reference）。
- **結束時的架構**：`billing` 模組（`billing` schema：customers、plans、prices、subscriptions、invoices、mandates）；scheduler job 在 worker 裡執行；可注入假時鐘。

---

### I-13.01 付款人與已存支付方式

- **目標**：建立 customer（付款人）；在付款時選擇「保存支付方式」，把 PSP 的 payment method 關聯到 customer。
- **為什麼**：訂閱扣款需要「之後還能用」的支付方式，這需要持卡人的同意與 PSP 端的設定。
- **不做**：扣款。
- **先想清楚**：
  1. customer 是平台的實體，還是商戶的？跨商戶可以共用嗎？
  2. 保存卡片時，PSP 需要什麼設定（setup_future_usage）？
  3. 付款人要求刪除卡片時怎麼處理？
- **完成條件**：測試：保存後可以列出支付方式（只含允許的欄位）；刪除後無法使用。
- **常見陷阱**：未經同意就保存支付方式。
- **Survey**：Stripe Customers、SetupIntent、setup_future_usage
- **產出**：customers 表、支付方式關聯

### I-13.02 CIT、MIT 與授權書

- **目標**：區分持卡人在場的付款（CIT）與商戶發起的付款（MIT）；保存首次付款的 network transaction id 與授權書（mandate）。
- **為什麼**：MIT 不能做 3DS（持卡人不在場），必須能證明之前有授權，否則發卡行會拒絕或之後被爭議。
- **不做**：訂閱邏輯。
- **先想清楚**：
  1. MIT 扣款時要帶哪些資料給 PSP（off_session、network transaction id）？
  2. 授權書要記錄什麼（時間、內容、IP、金額範圍）？
  3. MIT 被要求 3DS 時（SCA 軟拒），要怎麼通知付款者回來驗證？
- **完成條件**：sandbox 測試：用已存卡做 off-session 扣款成功；需要驗證的測試卡進入 requires_action 並發出事件。
- **常見陷阱**：把 MIT 當成一般付款送出，沒有標記 off-session。
- **Survey**：CIT vs MIT、stored credential framework、Stripe off_session payments、Hyperswitch mandate
- **產出**：mandates 表

### I-13.03 方案與價格

- **目標**：建立 plan 與 price（金額、幣別、週期、試用期）；價格不可修改，只能建立新的。
- **為什麼**：已訂閱的客戶必須維持原價，直到明確遷移。
- **不做**：用量計費。
- **先想清楚**：
  1. 為什麼 price 要不可修改？
  2. 一個 plan 可以有多種幣別的 price 嗎？
- **完成條件**：測試：修改價格會建立新 price，舊訂閱不受影響。
- **常見陷阱**：直接修改價格欄位。
- **Survey**：Stripe Products and Prices、Chargebee plans
- **產出**：plans、prices

### I-13.04 訂閱狀態機

- **目標**：訂閱狀態：trialing、active、past_due、canceled、paused、incomplete，以及合法轉移。
- **為什麼**：訂閱的狀態決定是否繼續扣款、是否提供服務，錯了就是多扣或少扣。
- **不做**：扣款排程。
- **先想清楚**：
  1. 首次付款失敗時訂閱是什麼狀態？
  2. 取消是立即生效還是週期結束才生效？
- **完成條件**：表驅動測試。
- **常見陷阱**：用 `is_active` 布林值表示訂閱。
- **Survey**：Stripe subscription statuses
- **產出**：subscriptions、狀態機

### I-13.05 發票

- **目標**：每個計費週期產生一張發票（明細、金額、狀態、唯一編號），發票狀態 draft、open、paid、void、uncollectible。
- **為什麼**：發票是扣款的依據，也是商戶與付款者之間的帳務憑證。
- **不做**：稅額（不在範圍內）。
- **先想清楚**：
  1. 發票編號怎麼保證唯一且連續（部分地區有要求）？
  2. 已開立的發票可以修改嗎？
- **完成條件**：測試：同一週期只會產生一張發票；開立後不能修改金額。
- **常見陷阱**：發票與付款一對一綁死，無法處理部分付款或重試。
- **Survey**：invoice lifecycle、invoice numbering
- **產出**：invoices

### I-13.06 計費週期與按比例計費

- **目標**：處理月底日期（1/31 的下個月）、升降級時的按比例計費（proration）。
- **為什麼**：日期與比例計算是 billing 最常出錯的地方。
- **不做**：多種週期混合。
- **先想清楚**：
  1. 1 月 31 日開始的月訂閱，2 月扣款日是哪天？
  2. 週期中升級，差額立即收還是下期收？
  3. 時區用商戶的還是 UTC？
- **完成條件**：表驅動測試涵蓋月底、閏年、升級、降級。
- **常見陷阱**：用 30 天當一個月。
- **Survey**：billing anchor、proration、Stripe billing cycle anchor
- **產出**：週期計算函式

### I-13.07 排程扣款

- **目標**：scheduler 在扣款日對到期發票發起 MIT 扣款；每張發票每次嘗試有冪等 key。
- **為什麼**：排程重跑、多個 worker、當機都可能讓同一張發票被扣兩次。
- **不做**：失敗重試（下一次）。
- **先想清楚**：
  1. 冪等 key 用 invoice id + attempt 序號？
  2. 排程要用自己的 job 表（像 Hyperswitch 的 process_tracker），還是外部排程器？
- **完成條件**：測試：排程同時在兩個 worker 執行，同一張發票只扣一次。
- **常見陷阱**：用「今天有沒有扣過」來判斷，跨時區就錯了。
- **設計模式**：單一執行者排程、Idempotency key（每個計費週期一個 key）
- **Survey**：Hyperswitch scheduler process_tracker、job scheduling SKIP LOCKED
- **產出**：billing job

### I-13.08 失敗重試策略

- **目標**：扣款失敗時依拒絕類型重試（soft decline 重試，hard decline 不重試），間隔遞增，有次數上限。
- **為什麼**：大部分訂閱流失是被動的（卡過期、額度不足），合理的重試能救回來。
- **不做**：智慧重試。
- **先想清楚**：
  1. 重用 I-12.07 的 soft / hard 分類？
  2. 重試時間要不要選在發薪日之後？（讀 Hyperswitch revenue recovery 的設計）
- **完成條件**：測試：soft decline 依時程重試；hard decline 不重試並通知。
- **常見陷阱**：每天重試直到成功。
- **Survey**：smart retries、revenue recovery、Hyperswitch revenue_recovery
- **產出**：重試政策

### I-13.09 Dunning

- **目標**：扣款失敗後通知付款者更新支付方式；寬限期後依政策暫停或取消訂閱。
- **為什麼**：重試之外，最有效的挽回方式是讓付款者自己更新卡片。
- **不做**：多語系郵件。
- **先想清楚**：
  1. 寬限期內要不要繼續提供服務？
  2. 付款者更新卡片後，要立即重扣未付發票嗎？
- **完成條件**：測試：失敗→通知→更新卡片→重扣成功→訂閱回到 active；寬限期滿→取消。
- **常見陷阱**：付款者更新卡片後，舊的重試排程仍在跑，造成重複扣款。
- **Survey**：dunning management、account updater
- **產出**：dunning 流程、通知事件

### I-13.10 訂閱與帳務

- **目標**：確認訂閱扣款走與一般付款相同的過帳規則；發票、付款、ledger 之間有可追溯的關聯。
- **為什麼**：billing 不應該有自己的一套帳；但需要能從帳本追溯到發票。
- **不做**：遞延收入（那是商戶的會計，不是平台的）。
- **先想清楚**：
  1. 平台帳本需要知道發票嗎？還是只需要 payment？
  2. 發票作廢（void）時，若已付款，要如何處理？
- **完成條件**：測試：訂閱扣款的 journal 可以追溯到 invoice id；P06 對帳測試加入訂閱資料仍通過。
- **常見陷阱**：為 billing 另外寫一套過帳。
- **Survey**：subscription accounting platform vs merchant
- **產出**：關聯欄位

### I-13.11 假時鐘測試（含階段檢核）

- **目標**：用可注入的時鐘，模擬一年的訂閱生命週期（試用、扣款、失敗、重試、升級、取消），在幾秒內跑完。
- **為什麼**：billing 的 bug 常常要過幾個月才出現；時間必須能被測試控制。
- **不做**：真實時間的長時間測試。
- **先想清楚**：
  1. 程式裡有多少地方直接呼叫 `time.Now()`？要怎麼改？
  2. 外部 PSP 的 sandbox 也有測試時鐘嗎？
- **完成條件**：一年模擬測試通過，發票數量與金額符合預期。
- **常見陷阱**：只有 billing 用假時鐘，其他模組仍讀真實時間，導致不一致。
- **設計模式**：Clock 注入（沿用 I-00.08 的 Clock 介面）
- **Survey**：Stripe test clocks、clock injection Go
- **產出**：`internal/platform/clock`、`docs/notes/p13-review.md`

## 階段檢核

- **面試題**：
  1. CIT 與 MIT 的差別？為什麼 MIT 不能做 3DS？
  2. 怎麼保證同一張發票不會被扣兩次？
  3. 1 月 31 日開始的月訂閱，下一次什麼時候扣？
  4. 訂閱扣款失敗後你怎麼做？
- **故障場景**：排程重跑；付款者更新卡片時的重複扣款；MIT 被要求 3DS。
- **tag**：`p13-billing-done`
