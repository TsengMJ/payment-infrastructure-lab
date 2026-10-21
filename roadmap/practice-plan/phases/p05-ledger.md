# P05 Ledger（13 次迭代）

- **對應模組**：#11 Ledger & Balance Engine
- **階段目標**：一個複式記帳的帳本：每筆 journal 在同一幣別內借貸平衡、不可修改、不重複過帳；能算出 pending、available、reserved 餘額，並在併發下不會超額預留。
- **前置**：P03（outbox / inbox）、P04（退款事件）。
- **結束時的架構**：`ledger` 模組（`ledger` schema：accounts、journals、postings、holds、balances），是**唯一的帳務寫入者**；worker 消費付款與退款事件並過帳。

如果你之前沒有會計背景，這個階段的前兩次迭代會偏重閱讀。不要跳過，錯的科目設計會讓後面每一步都很痛苦。

---

### I-05.01 會計基礎與科目表

- **目標**：寫出平台的科目表（chart of accounts），每個科目有類型與正常餘額方向，並寫成 seed。
- **為什麼**：帳本的正確性從科目設計開始；PSP 應收、商戶應付、平台收入是三種完全不同性質的錢。
- **不做**：程式邏輯。
- **先想清楚**：
  1. 資產、負債、權益、收入、費用五類科目各自的借方與貸方代表什麼？
  2. 平台收到客戶付款時，這筆錢是平台的收入，還是欠商戶的負債？
  3. 每個商戶要不要有自己的應付帳戶？每個幣別呢？
- **完成條件**：`docs/ledger-design.md` 列出科目表，並用文字寫出「收款 100、平台抽 3、PSP 收 2.9」的分錄。
- **常見陷阱**：把商戶的錢記成平台收入。
- **Survey**：double-entry bookkeeping、normal balance、Modern Treasury ledgers、Square books
- **產出**：科目表文件、ADR 0011「科目表與帳戶粒度」

### I-05.02 Ledger schema

- **目標**：建立 accounts、journals、postings 三張表與必要約束。
- **為什麼**：帳本的不變量要盡可能由資料庫結構保證。
- **不做**：過帳 API。
- **先想清楚**：
  1. posting 的金額用正負號表示借貸，還是用 direction 欄位加正數？
  2. 帳戶要不要綁定幣別？一個帳戶能存多種幣別嗎？
  3. journal 需要哪些欄位：業務來源、描述、生效日、建立時間？
- **完成條件**：migration 完成；CHECK 約束保證金額為正、方向只有兩種值。
- **常見陷阱**：一個帳戶允許多種幣別，之後無法算餘額。
- **Survey**：ledger database schema、immutable ledger Postgres
- **產出**：migration

### I-05.03 Journal 借貸平衡檢查

- **目標**：`ledger.Post(journal)` 在同一交易裡寫入 journal 與所有 postings，提交前檢查每個幣別的借方總和等於貸方總和。
- **為什麼**：這是複式記帳最核心的不變量；不平衡的帳本代表錢憑空出現或消失。
- **不做**：冪等（I-05.05）。
- **先想清楚**：
  1. 平衡檢查放在應用程式還是資料庫（deferred constraint trigger）？兩者能不能都做？
  2. 多幣別的 journal 要怎麼平衡（提示：每個幣別各自平衡，P14 會用到）？
  3. posting 的帳戶幣別與 posting 幣別不同時怎麼辦？
- **完成條件**：測試：平衡的 journal 成功；不平衡、跨幣別混算、帳戶幣別不符都被拒絕且沒有任何資料寫入。
- **常見陷阱**：先寫 journal 再一筆一筆寫 posting，中間失敗留下半筆帳。
- **Survey**：Postgres deferred constraint trigger、journal entry balancing
- **產出**：Post API

### I-05.04 不可變與沖銷

- **目標**：已過帳的 journal 與 posting 不能 UPDATE 或 DELETE；更正一律以新的 reversal journal 完成。
- **為什麼**：帳本是稽核證據；能被改的帳本就沒有可信度。
- **不做**：調整分錄的審核流程（P06）。
- **先想清楚**：
  1. 用資料庫權限、trigger，還是兩者一起保證不可變？
  2. reversal journal 怎麼關聯到原 journal？能不能被沖銷兩次？
  3. 如果真的寫錯了（例如 bug），正確的修正流程是什麼？
- **完成條件**：測試：對 postings 執行 UPDATE / DELETE 失敗；沖銷後兩筆 journal 的淨效果為零；同一 journal 不能被沖銷兩次。
- **常見陷阱**：用 soft delete 欄位「刪除」錯誤分錄。
- **設計模式**：Append-only、沖銷分錄（補償）
- **Survey**：append-only ledger、reversal entry、Postgres REVOKE UPDATE
- **產出**：trigger 或權限設定、Reverse API

### I-05.05 冪等過帳

- **目標**：每個 journal 有 `source_event_id`（或業務操作 ID），唯一約束保證同一業務事件只過帳一次。
- **為什麼**：outbox 是至少一次投遞；沒有過帳冪等，事件重送就是重複入帳。
- **不做**：事件消費（I-05.07）。
- **先想清楚**：
  1. 用 outbox 事件 ID 去重夠嗎？兩個不同事件代表同一個業務操作時怎麼辦（例如 API 回應與 webhook 都說 capture 成功）？
  2. 重複過帳時應該回錯誤，還是回原本的 journal？
- **完成條件**：測試：同一個 source id 送 100 次，只有一筆 journal。
- **常見陷阱**：只用事件 ID 去重，忽略同一業務操作的不同事件。
- **Survey**：idempotent ledger posting、business operation id
- **產出**：唯一約束與 Post 的冪等行為

### I-05.06 過帳規則表

- **目標**：定義業務事件到分錄的規則：capture、refund、（之後）PSP fee、平台費、payout。
- **為什麼**：「什麼事件、記哪些科目、多少錢」應該集中、可審查，而不是散落在各個模組。
- **不做**：消費事件。
- **先想清楚**：
  1. 授權要過帳嗎？還是只有請款才過帳？架構文件第 4 節的會計政策注意事項怎麼說？
  2. 規則寫成程式碼、資料表還是設定？審查與測試哪一種最容易？
  3. 退款的分錄是 capture 分錄的反向，還是不同的科目？
- **完成條件**：`docs/ledger-design.md` 有過帳規則表；每條規則有單元測試，輸出的 journal 平衡。
- **常見陷阱**：把授權記成收入。
- **Survey**：posting rules、accounting events payments
- **產出**：規則程式、規則表文件

### I-05.07 從事件驅動過帳

- **目標**：worker 消費 `payment.captured`、`refund.succeeded` 事件，依規則過帳；用 inbox 去重。
- **為什麼**：Payment 不直接寫帳本，這是架構文件第 5.1 節「掌握狀態，不直接改資金帳」的落地。
- **不做**：事件失敗的人工處理。
- **先想清楚**：
  1. 過帳失敗時（例如規則不存在），事件要怎麼處理？
  2. 過帳延遲多久可以接受？怎麼監控？
  3. Ledger 要不要回頭查 payment 的資料，還是事件 payload 必須足夠？
- **完成條件**：故障場景「Refund 成功但 Ledger 過帳失敗」：修復後重新處理，帳本最終正確且只過帳一次。
- **常見陷阱**：在 payment 的交易裡同步呼叫 ledger，讓兩個模組的寫入綁在一起。
- **Survey**：event-driven ledger、posting lag
- **產出**：ledger consumer、posting lag metric

### I-05.08 餘額計算

- **目標**：能查詢任一帳戶的餘額；比較「每次從 postings 加總」與「維護餘額表」兩種做法，並實作一致性檢查。
- **為什麼**：餘額查詢很頻繁，但快取的餘額可能和明細不一致；兩者都要有。
- **不做**：pending / available 的區分（下一次）。
- **先想清楚**：
  1. 餘額表在過帳時同交易更新，會不會成為熱點？
  2. 怎麼檢查餘額表與明細一致？多久檢查一次？
  3. 查詢某個時間點的歷史餘額要怎麼做？
- **完成條件**：測試：過帳後餘額正確；一致性檢查能抓出被手動改壞的餘額表。
- **常見陷阱**：只有餘額表沒有明細，出錯時無法重建。
- **設計模式**：Snapshot + 增量
- **Survey**：ledger balance computation、running balance
- **產出**：Balance API、一致性檢查 job

### I-05.09 Pending 與 Available 餘額

- **目標**：商戶應付餘額區分 pending（已請款、尚未結算）與 available（可出款）；定義轉換的時機。
- **為什麼**：請款成功不代表錢已經到平台；讓商戶提領還沒結算的錢，平台就在墊錢。
- **不做**：結算資料匯入（P06）。
- **先想清楚**：
  1. pending 與 available 用不同帳戶表示，還是同一帳戶的不同屬性？
  2. 轉換由什麼觸發：固定天數（T+N）、PSP 結算報表、還是兩者？
  3. 退款應該從 pending 扣還是從 available 扣？
- **完成條件**：測試：capture 後增加 pending；轉換事件後 pending 減少、available 增加；總額不變。
- **常見陷阱**：用 UPDATE 改狀態而不是過帳轉換，失去軌跡。
- **Survey**：pending vs available balance、Stripe balance transactions
- **產出**：轉換規則、ADR 0012「pending 轉 available 的觸發條件」

### I-05.10 Hold 與 reservation

- **目標**：提供 `Hold(account, amount)`、`Release(hold)`、`Capture(hold)`；hold 會減少 available，但還不是真正扣款。
- **為什麼**：出款送出到銀行確認之間有時間差，錢必須先被鎖住，否則會被重複使用。
- **不做**：payout 本身（P07）。
- **先想清楚**：
  1. hold 要用 posting 表示（轉到 reserved 帳戶），還是獨立的 holds 表？
  2. hold 要不要有到期時間？過期自動釋放安全嗎？
  3. Capture hold 時，金額可以和 hold 不同嗎？
- **完成條件**：測試：hold 後 available 減少；release 後恢復；capture 後變成實際扣款；同一 hold 不能 release 又 capture。
- **常見陷阱**：hold 自動過期釋放，但出款其實已經成功。
- **設計模式**：Reservation（Hold）
- **Survey**：ledger holds、authorization holds、funds reservation
- **產出**：Hold API

### I-05.11 併發控制：不能超額預留

- **目標**：100 個併發 hold 請求不會讓 available 變成負數。
- **為什麼**：「兩個請求都看到餘額 100、各自扣 70」是最經典的雙花問題。
- **不做**：分散式鎖。
- **先想清楚**：
  1. 選哪一種：對帳戶列加 `FOR UPDATE`、Serializable 隔離等級、還是帶條件的 UPDATE？各自的衝突與重試行為？
  2. 熱門帳戶（大商戶）會不會變成瓶頸？
  3. 死鎖可能發生在哪裡？鎖的順序怎麼固定？
- **完成條件**：故障場景「同一商戶餘額 100，併發 payout 70 + 70」：只有一個 hold 成功；100 次併發測試穩定通過。
- **常見陷阱**：在應用程式裡先讀餘額再判斷。
- **設計模式**：悲觀鎖 / 條件更新、Reservation（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 3.3 節）
- **Survey**：Postgres row locking、serializable snapshot isolation、hot account problem
- **產出**：ADR 0013「Ledger 併發控制策略」、`tests/failure/concurrent_hold_test.go`

### I-05.11a 熱點帳戶

- **目標**：量測同一個帳戶（例如平台手續費收入帳戶）在高併發過帳下的鎖等待與吞吐，實作一種緩解方式，再用同一個腳本量一次。
- **為什麼**：每一筆付款都會記一筆平台收入。如果所有交易都鎖同一列餘額，整個 ledger 的吞吐會被一個帳戶卡住。這是 ledger 系統設計面試最常見的追問之一。
- **不做**：分散式 ledger、改用 TigerBeetle。
- **先想清楚**：
  1. 哪些帳戶會變成熱點？為什麼最嚴重的通常是平台帳戶，而不是單一商戶帳戶？
  2. 緩解選項有哪些：拆成多個子帳戶（sharded sub-accounts）、過帳時不更新餘額列而改成事後彙總、批次過帳？各自對「即時餘額」與「不能超額預留」的影響？
  3. 哪些帳戶需要即時準確的 available（會被 hold 的），哪些只需要最終正確？
- **完成條件**：`docs/evidence/hot-account.md` 有前後對照的量測（同一腳本、同一資料量、同一機器）與選擇理由；I-05.03 到 I-05.11 的所有不變量測試仍然通過。
- **常見陷阱**：為了吞吐，把需要預留的商戶帳戶也改成事後彙總，失去超額保護。
- **設計模式**：Sharded counter、Snapshot + 增量（見 [`03-design-patterns.md`](../03-design-patterns.md) 第 3.5 節）
- **Survey**：hot account problem ledger、sharded counters、pg_stat_activity lock waits、pgbench custom scripts
- **產出**：量測腳本、`docs/evidence/hot-account.md`

### I-05.12 Property-based test（含階段檢核）

- **目標**：用 property-based test 產生至少 1,000 組隨機的資金操作序列，驗證不變量。
- **為什麼**：手寫的測試只涵蓋你想得到的情況；隨機序列會找到你沒想到的。
- **不做**：效能測試。
- **先想清楚**：
  1. 要驗證哪些不變量：每個 journal 平衡、全帳本總和為零、available 不為負、退款不超過請款？
  2. 失敗時怎麼縮小（shrink）到最小可重現的序列？
- **完成條件**：property test 在 CI 中執行並通過；故意引入一個 bug 時能被抓到。
- **常見陷阱**：產生器只產生合法序列，從不測試非法操作會被拒絕。
- **Survey**：property-based testing Go、rapid、gopter、invariants
- **產出**：`tests/property/ledger_test.go`、`docs/notes/p05-review.md`

## 階段檢核

- **面試題**：
  1. 用複式記帳表示：客戶付 100，平台抽 3。
  2. 為什麼帳本不能修改？寫錯了怎麼辦？
  3. 你怎麼防止同一筆事件被過帳兩次？
  4. 兩筆併發出款同時要用同一筆餘額，你怎麼處理？
  5. Pending 與 available 餘額有什麼不同？
- **故障場景**：Refund 成功但 Ledger 過帳失敗；同一商戶餘額 100，併發 70 + 70；重複事件 ×100。
- **tag**：`p05-ledger-done`
