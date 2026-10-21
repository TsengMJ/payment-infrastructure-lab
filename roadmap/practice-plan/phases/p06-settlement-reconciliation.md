# P06 結算、對帳、手續費（14 次迭代）

- **對應模組**：#12 Clearing & Settlement、#13 Reconciliation、#15 Fee & Pricing（手續費部分，FX 在 P14）
- **階段目標**：能匯入 PSP 結算報表與銀行對帳單，找出並分類內外部差異；手續費與平台費由規則計算並正確入帳。
- **前置**：P05。
- **結束時的架構**：`settlement` 模組（`settlement` schema：reports、lines、batches、recon_runs、breaks、pricing_plans）；**只讀** ledger，任何調整都透過 ledger 的 Post API。

架構文件說這幾個模組「最能區分只會串支付 API 和理解支付基礎設施」。這個階段的價值在於故意製造差異並證明你找得到。

---

### I-06.01 結算概念與資料模型

- **目標**：寫出 transaction、settlement、cash 三個時間點的說明，並建立 settlement_reports、settlement_lines、settlement_batches 三張表。
- **為什麼**：請款成功、PSP 結算、錢進銀行是三件不同的事，報表與對帳必須分清楚。
- **不做**：匯入。
- **先想清楚**：
  1. 一筆 PSP 結算款（payout to platform）包含哪些交易、手續費、退款、爭議？
  2. gross 與 net 的差別？
  3. 結算批次與我們內部的哪些紀錄對應？
- **完成條件**：`docs/reconciliation.md` 有三個時間點的流程圖；migration 完成。
- **常見陷阱**：把 PSP payout 到平台的錢，和平台 payout 給商戶的錢混為一談。
- **Survey**：Stripe balance transactions、Stripe payout reconciliation report、Adyen settlement details report
- **產出**：文件、migration

### I-06.02 合成報表產生器與 ground truth

- **目標**：寫一個產生器，從內部資料產生模擬的 PSP 結算報表，並可指定植入哪些差異；同時輸出 ground truth 檔。
- **為什麼**：沒有已知答案，就無法證明對帳引擎找得到所有差異。
- **不做**：真實 Stripe 報表（下一次之後才用）。
- **先想清楚**：
  1. 要能植入哪些差異：漏一筆、多一筆、金額不同、手續費不同、晚一天結算、孤兒退款？
  2. 產生器要可重現（固定 seed）嗎？
  3. 如何產生 100,000 筆而不拖垮資料庫？
- **完成條件**：`make gen-settlement N=100000 SEED=1` 產生報表與 ground truth。
- **常見陷阱**：產生器與對帳引擎共用程式邏輯，結果自己驗證自己。
- **Survey**：synthetic data generation、ground truth testing
- **產出**：`tools/settlementgen`

### I-06.03 報表匯入與正規化

- **目標**：把不同格式的報表（Mock 與 Stripe）匯入成統一的 settlement_lines。
- **為什麼**：每家 PSP 的報表欄位、時區、金額格式都不同；比對前要先正規化。
- **不做**：比對。
- **先想清楚**：
  1. 報表時間是 UTC 還是 PSP 帳號時區？怎麼統一？
  2. 金額是 minor unit 還是十進位字串？負數表示什麼？
  3. 無法解析的行要整檔失敗，還是記錄錯誤繼續？
- **完成條件**：測試：兩種格式匯入後欄位一致；壞掉的行被記錄並計數。
- **常見陷阱**：把時區轉換做錯，導致跨日的交易全部錯位。
- **設計模式**：Pipeline（解析、正規化、驗證、寫入各自可測、可重跑）、Adapter（每家 PSP 一個報表 parser）
- **Survey**：CSV parsing Go、timezone normalization
- **產出**：parser 與 normalizer（每家一個 adapter）

### I-06.04 匯入冪等

- **目標**：同一份報表重複匯入，不會產生重複的 lines。
- **為什麼**：排程重跑、人工重傳都很常見。
- **不做**：報表版本修正（PSP 重新發布修正版）只寫在筆記。
- **先想清楚**：
  1. 用檔案雜湊、PSP 的報表 ID，還是每行的業務 key 去重？
  2. PSP 發布修正版報表時，怎麼處理？
- **完成條件**：測試：同檔匯入兩次，lines 數量不變。
- **常見陷阱**：只用檔名去重。
- **Survey**：idempotent file ingestion
- **產出**：匯入去重

### I-06.05 比對規則一：reference 精確比對

- **目標**：以 PSP reference 把 settlement_lines 對到內部的 capture 與 refund。
- **為什麼**：大部分交易能用 reference 直接對上；先處理簡單的，剩下的才是需要調查的。
- **不做**：模糊比對。
- **先想清楚**：
  1. 內部要用哪個 reference：Stripe charge id、balance transaction id、還是 metadata 裡的 attempt id？
  2. 一對多（一筆請款拆成多筆結算）怎麼處理？
- **完成條件**：在沒有植入差異的資料上，100% 配對成功。
- **常見陷阱**：用金額當比對 key。
- **設計模式**：Strategy / 規則鏈（比對規則依序套用，I-06.06 再加一條）
- **Survey**：transaction matching、reconciliation reference
- **產出**：matcher

### I-06.06 比對規則二：金額、幣別、時間窗與營業日

- **目標**：reference 對上後，檢查金額與幣別；用時間窗與營業日判斷「晚到」與「缺漏」的差別。
- **為什麼**：週末、假日、時區會讓結算晚幾天；不能把正常延遲誤判成短款。
- **不做**：分類（下一次）。
- **先想清楚**：
  1. 時間窗要多大？依 PSP 的結算週期（T+2、T+7）決定嗎？
  2. 營業日行事曆從哪裡來？
  3. 超過時間窗仍未出現，才算缺漏？
- **完成條件**：測試：晚 1 個營業日結算的交易被標為 pending，不是 missing。
- **常見陷阱**：只用日曆日計算。
- **Survey**：settlement timing、business day calendar、T+N settlement
- **產出**：比對條件

### I-06.07 差異分類

- **目標**：把未對上的項目分類為 missing_internal、missing_external、duplicate、amount_mismatch、fee_mismatch、timing、orphan_refund。
- **為什麼**：不同類型的差異有不同的根因與處理方式。
- **不做**：處理（I-06.09）。
- **先想清楚**：
  1. 每一類最可能的根因是什麼？
  2. 一筆差異能不能同時屬於兩類？
  3. 哪幾類需要立即告警？
- **完成條件**：對 100,000 筆含植入差異的資料，所有植入的差異都被找到並分類正確（與 ground truth 比對）。
- **常見陷阱**：把無法分類的項目默默丟掉。
- **Survey**：reconciliation break types、exception management
- **產出**：break 分類、`docs/evidence/recon-ground-truth.md`

### I-06.08 對帳 run 與重跑冪等

- **目標**：每次對帳是一個 run（有 run_id、輸入範圍、結果快照）；重跑同一範圍不會產生重複的 breaks 或重複補帳。
- **為什麼**：對帳常常要重跑（報表晚到、修正規則）；重跑不能製造新的錯。
- **不做**：排程。
- **先想清楚**：
  1. 新 run 的結果與舊 run 不同時，舊的 breaks 怎麼處理？
  2. 已經人工處理過的 break，重跑後要保留處理結果嗎？
- **完成條件**：測試：同範圍重跑三次，breaks 數量與狀態不變。
- **常見陷阱**：每次重跑都產生新的 break 紀錄，人工處理的進度全部遺失。
- **Survey**：reconciliation run idempotency
- **產出**：recon_runs、break 的穩定 key

### I-06.09 人工覆核與調整分錄

- **目標**：break 可被指派、註記、核准處理；需要調帳時，產生調整 journal（透過 ledger API），並留下稽核軌跡。
- **為什麼**：對帳的目的不是自動改帳，而是讓差異有人負責並留下證據。架構文件明確說「不是自動偷偷修改 Ledger」。
- **不做**：UI（P15）。
- **先想清楚**：
  1. 誰可以核准調帳？需要兩個人嗎（maker-checker）？
  2. 調整分錄用哪些科目（例如對帳差異暫記帳戶）？
  3. 調整後，break 怎麼關閉？下次重跑會不會又出現？
- **完成條件**：測試：fee_mismatch break 經核准後產生一筆平衡的調整 journal，break 狀態為 resolved；未核准不能調帳。
- **常見陷阱**：對帳程式直接寫 ledger 表。
- **設計模式**：Maker-checker、沖銷分錄（補償）
- **Survey**：maker-checker、suspense account、audit trail
- **產出**：break 處理 API

### I-06.10 PSP 手續費入帳

- **目標**：從結算報表取得 PSP 手續費，依 gross / net 過帳到手續費費用科目。
- **為什麼**：PSP 給你的是扣完手續費的淨額；不記手續費，帳本就永遠對不上銀行。
- **不做**：預估手續費。
- **先想清楚**：
  1. 手續費應在請款時預估入帳，還是結算時實際入帳？兩者差異怎麼處理？
  2. 手續費由平台負擔還是轉嫁商戶？這會改變哪些分錄？
- **完成條件**：測試：gross 100、fee 3.2、net 96.8 的分錄平衡，PSP 應收清零。
- **常見陷阱**：把 net 當成 gross 入帳，手續費消失。
- **Survey**：interchange++ pricing、blended pricing、Stripe fees balance transaction
- **產出**：fee 過帳規則

### I-06.11 商戶費率方案

- **目標**：建立 pricing_plans（百分比 + 固定費、依支付方式或幣別區分），並有生效時間；每個商戶綁定一個方案。
- **為什麼**：平台的收入來自費率；不同商戶的費率不同，而且會變。
- **不做**：分潤（revenue share）。
- **先想清楚**：
  1. 費率變更時，已經發生但未結算的交易用舊費率還是新費率？
  2. 費率方案要不要版本化、不可修改？
  3. 費率計算的四捨五入規則？
- **完成條件**：測試：費率變更前後的交易使用各自的費率；歷史方案不能被修改。
- **常見陷阱**：直接修改方案的費率，讓歷史交易的費用無法重算。
- **設計模式**：Strategy（不同計費方式）、Effective-dated records
- **Survey**：payment pricing models、effective dated records
- **產出**：pricing_plans、商戶與方案的關聯

### I-06.12 平台費計算與分錄

- **目標**：每筆 capture 依商戶費率計算平台費，過帳成「商戶應付 97、平台收入 3」。
- **為什麼**：抽成必須由 ledger 規則決定，不是 UI 上的數字。
- **不做**：退款時是否退還平台費以外的複雜政策（寫成 ADR 即可）。
- **先想清楚**：
  1. 退款時要退還平台費嗎？全額與部分退款各怎麼算？
  2. 四捨五入的差額歸誰？
  3. 平台費在請款時就認列，還是結算時？
- **完成條件**：測試：100 = 97 + 3；部分退款 50 時的分錄依 ADR 規則正確；四捨五入不產生不平衡。
- **常見陷阱**：先算淨額再反推費用，造成分錢誤差。
- **Survey**：marketplace fees accounting、application fee Stripe Connect
- **產出**：平台費過帳規則、ADR 0014「退款時的平台費政策」

### I-06.13 第三方對帳一：銀行對帳單匯入

- **目標**：產生並匯入 Mock 銀行對帳單（平台銀行帳戶的入帳與出帳）。
- **為什麼**：PSP 說它付了錢，不代表錢真的進了銀行；三方對帳的第三方就是銀行。
- **不做**：真實銀行格式解析（可在筆記比較 CAMT.053、BAI2、MT940）。
- **先想清楚**：
  1. 銀行對帳單的一筆入帳，通常對應 PSP 的什麼？
  2. 銀行的描述欄位常常不完整，怎麼找到對應的 PSP payout？
- **完成條件**：匯入測試通過；同檔重匯不重複。
- **常見陷阱**：以為銀行入帳會帶完整的 reference。
- **Survey**：CAMT.053、BAI2、MT940、bank statement reconciliation
- **產出**：銀行對帳單 adapter

### I-06.14 第三方對帳二：PSP 撥款對銀行入帳（含階段檢核）

- **目標**：把 PSP 撥款（PSP 給平台的 payout）對到銀行入帳；確認「PSP 應收」與「在途資金」科目在對帳後歸零。
- **為什麼**：這是資金閉環的最後一段，證明帳本、PSP、銀行三方一致。
- **不做**：商戶出款的銀行對帳（P07 之後回頭補一個測試即可）。
- **先想清楚**：
  1. 在途資金（cash in transit）科目怎麼設計？
  2. PSP 撥款與銀行入帳差了一天，怎麼表達？
  3. 金額對不上（銀行扣了匯費）怎麼分類？
- **完成條件**：故障場景「Provider settlement report 少了一筆 capture」能被定位；三方對帳報告列出預期、實際、差異、分類、處理狀態。
- **常見陷阱**：只做 ledger 對 PSP，從未確認錢真的到銀行。
- **Survey**：three-way reconciliation、cash in transit、clearing account
- **產出**：三方對帳報告、`docs/notes/p06-review.md`

## 階段檢核

- **面試題**：
  1. Authorization、Capture、Settlement、Payout 分別是什麼時間點？
  2. 你的對帳能找出哪些差異？怎麼證明都找得到？
  3. 正常的結算延遲與真正的短款怎麼區分？
  4. 對帳發現差異之後，你會直接改帳嗎？
  5. 平台抽成在帳本上怎麼記？退款時呢？
- **故障場景**：Provider 報表少一筆 capture；手續費不符；重跑對帳不重複補帳；晚結算不誤判。
- **tag**：`p06-settlement-done`
