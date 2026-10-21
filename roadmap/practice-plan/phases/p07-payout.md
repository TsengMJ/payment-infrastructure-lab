# P07 Payout（9 次迭代）

- **對應模組**：#14 Payout & Fund Transfer
- **階段目標**：商戶能從 available 餘額提領；資金先原子預留，結果未知時不重送，只有確認失敗才釋放；大額需要審核；可批次出款；提領後的退款有明確的負餘額政策。
- **前置**：P05（hold API）、P06（available 餘額來源）。
- **結束時的架構**：`payout` 模組（`payout` schema：payouts、payout_attempts、bank_accounts、batches）；Mock 銀行 adapter；payout 透過 ledger 的**同步** hold API 預留資金。

這個階段商戶能不能出款，先用手動設定的 `merchant.payouts_enabled` 決定。P08 會改成由 KYB 與銀行帳戶驗證的結果決定。

---

### I-07.01 Payout 模型與狀態機

- **目標**：建立 payouts 與 payout_attempts，定義狀態：pending_approval、approved、processing、unknown、paid、failed、returned、canceled。
- **為什麼**：出款和付款一樣有「意圖」與「每一次執行」之分，失敗後可能需要用新的 attempt 重送。
- **不做**：呼叫銀行。
- **先想清楚**：
  1. 什麼情況下可以為同一個 payout 建立新的 attempt？
  2. `paid` 之後還會變成 `returned` 嗎（銀行退回）？
  3. 為什麼 payout 要單獨一個模組，而且在 P17 要最早被考慮獨立部署之一？
- **完成條件**：轉移表的表驅動測試。
- **常見陷阱**：把 processing 逾時直接當 failed。
- **Survey**：Stripe payouts lifecycle、payout returned
- **產出**：migration、狀態機

### I-07.02 出款帳戶

- **目標**：商戶可以登記出款銀行帳戶；只保存 vendor 或銀行的 reference 與遮罩後的帳號，並有 `verified` 狀態（P08 才真的驗證）。
- **為什麼**：出款帳戶被竄改是常見的詐騙手法；帳號本身也是敏感資料。
- **不做**：驗證流程（P08）。
- **先想清楚**：
  1. 出款帳戶變更時，要不要有冷卻期或額外驗證？
  2. 完整帳號要不要存？存的話怎麼加密、誰能看？
- **完成條件**：測試：API 與日誌只出現遮罩帳號；帳戶變更寫入稽核紀錄。
- **常見陷阱**：出款帳戶可以被任何有 API key 的人改。
- **Survey**：bank account tokenization、payout account change fraud
- **產出**：bank_accounts 表、ADR 0015「出款帳戶資料保存」

### I-07.03 建立 payout 並原子預留

- **目標**：`POST /v1/payouts` 在 ledger 建立 hold；餘額不足時拒絕；hold id 記錄在 payout 上。
- **為什麼**：預留必須是 ledger 的原子操作，否則併發提領會超額出款。
- **不做**：送出銀行指令。
- **先想清楚**：
  1. 先建 payout 再 hold，還是先 hold 再建 payout？中間失敗會留下什麼？
  2. payout 與 hold 不在同一個模組，兩者怎麼保持一致（提示：hold 冪等 key = payout id）？
  3. `payouts_enabled = false` 的商戶要在哪裡擋？
- **完成條件**：故障場景「商戶餘額 100，併發 payout 70 + 70」再跑一次（這次透過 payout API）：只有一筆成功。
- **常見陷阱**：payout 模組自己讀餘額、自己判斷。
- **設計模式**：Reservation、Saga / Process manager（hold → 送出 → 確認或釋放，每一步失敗都有對應的補償）
- **Survey**：funds reservation、double spend payouts
- **產出**：payout API、`docs/evidence/concurrent-payouts.md`

### I-07.04 Mock 銀行轉帳與冪等

- **目標**：Mock 銀行 adapter 送出轉帳，使用 payout attempt 的 ID 作為轉帳冪等 key；Mock 可注入延遲、失敗、逾時、成功但不回應。
- **為什麼**：銀行轉帳一旦送出就很難撤回；重送就是付兩次錢。
- **不做**：真實銀行 API。
- **先想清楚**：
  1. 銀行或出款服務商常見的冪等機制是什麼（end-to-end id、client reference）？
  2. 送出指令應該在 worker 做，還是 API 請求裡同步做？
- **完成條件**：測試：同一 attempt 送兩次，Mock 銀行只記錄一筆轉帳。
- **常見陷阱**：每次重試產生新的 reference。
- **Survey**：ISO 20022 end-to-end id、pacs.008、payout API idempotency
- **產出**：`internal/payout/bank` adapter 與 Mock

### I-07.05 未知結果不重送

- **目標**：銀行逾時時 payout attempt 進入 unknown，以原 reference 查詢收斂；只有確認失敗才釋放 hold。
- **為什麼**：架構文件第 5.5 節：「網路斷線時不可直接重複送出一筆新轉帳」。
- **不做**：人工介面。
- **先想清楚**：
  1. 銀行查詢 API 本身也失敗時怎麼辦？
  2. unknown 狀態超過多久要轉人工？
  3. 確認失敗後，要自動重送新的 attempt，還是等人工？
- **完成條件**：故障場景「Payout 送出後網路斷線」：hold 維持、沒有第二筆轉帳；查詢確認成功後才進入 paid。
- **常見陷阱**：unknown 時釋放 hold，讓商戶可以再提領一次。
- **設計模式**：逾時即未知、先查再做（和 P03 是同一套，只是對象換成銀行）
- **Survey**：unknown outcome payout、reconciliation of payouts
- **產出**：payout recovery job、`docs/evidence/payout-network-loss.md`

### I-07.06 成功、失敗與退回的分錄

- **目標**：paid 時 hold 轉為實際扣款分錄；failed 時釋放 hold；returned（成功後被銀行退回）時以新分錄把錢加回商戶。
- **為什麼**：出款的每一個結果都要有對應的帳務，而且不能修改舊分錄。
- **不做**：退回手續費政策（寫在筆記）。
- **先想清楚**：
  1. returned 時要不要重新開放提領？要不要通知商戶更新帳戶？
  2. paid 的分錄記到哪些科目（商戶應付減少、平台銀行存款減少）？
- **完成條件**：測試：三種結果的分錄都平衡；returned 不修改原分錄。
- **常見陷阱**：returned 時刪除原本的扣款分錄。
- **Survey**：ACH return codes、payout reversal accounting
- **產出**：payout 過帳規則

### I-07.07 審核流程

- **目標**：超過門檻的 payout 進入 pending_approval；需要一位（或兩位不同的）內部人員核准才會送出。
- **為什麼**：大額出款與異常出款是詐騙損失的主要來源。
- **不做**：審核 UI（P15）。
- **先想清楚**：
  1. 門檻依商戶、幣別還是風險等級決定？
  2. 提出者與核准者可以是同一人嗎？怎麼在程式層級保證？
  3. 待審核期間，hold 要不要保留？被拒絕後呢？
- **完成條件**：測試：大額 payout 未核准不會送出；同一人不能自我核准；拒絕後 hold 釋放。
- **常見陷阱**：只在 UI 上隱藏按鈕，API 仍可直接核准。
- **Survey**：four-eyes principle、maker-checker
- **產出**：審核 API、稽核紀錄

### I-07.08 Payout 批次

- **目標**：每天依排程把所有到期的出款切成批次送出；批次中部分失敗時只重試失敗的項目。
- **為什麼**：實務上出款常以批次方式送往銀行；批次讓成本與作業可控，但也帶來部分失敗的問題。
- **不做**：真實銀行批次檔格式（筆記比較即可）。
- **先想清楚**：
  1. 排程重跑時，怎麼保證同一筆 payout 不會進入兩個批次？
  2. 批次的狀態與單筆 payout 的狀態是什麼關係？
  3. 切批時間點與時區？
- **完成條件**：測試：排程重跑不重複切批；批次中一筆失敗，其他筆不受影響且失敗筆可單獨重試。
- **常見陷阱**：批次層級重試，把成功的項目又送一次。
- **Survey**：payout batching、NACHA file、SEPA credit transfer batch
- **產出**：batch job

### I-07.09 提領後退款與負餘額（含階段檢核）

- **目標**：商戶已提領後才發生退款時，依政策處理：先用 reserve、再從未來收入扣、最後產生負餘額並限制出款。
- **為什麼**：平台已經把錢給商戶了，退款的錢從哪裡來，必須有規則，否則平台會默默承擔損失。
- **不做**：催收流程。
- **先想清楚**：
  1. Rolling reserve 怎麼運作？保留比例與期間？
  2. 負餘額的商戶還能收款嗎？還能出款嗎？
  3. 這些規則在 ledger 上用哪些科目表示？
- **完成條件**：測試：提領後全額退款，ledger 平衡；商戶餘額為負時新的 payout 被拒絕；後續收款優先抵銷負餘額。
- **常見陷阱**：讓退款失敗，因為商戶 available 不足。
- **Survey**：rolling reserve、negative balance marketplace、Stripe Connect negative balances
- **產出**：ADR 0016「負餘額與 reserve 政策」、`docs/settlement-payout.md`、`docs/notes/p07-review.md`

## 階段檢核

- **面試題**：
  1. 兩筆併發出款同時要用同一筆餘額，你怎麼保證只有一筆成功？
  2. 出款送出後網路斷了，你會重送嗎？
  3. Settlement 與 Payout 的差別？
  4. 商戶已經提領後客戶才退款，錢從哪裡來？
  5. 為什麼大額出款需要兩個人核准？
- **故障場景**（ProjectRoadmap 場景 10，架構文件場景 7、8）：併發 payout 70 + 70；Payout 送出後網路斷線；批次部分失敗；提領後退款。
- **tag**：`p07-payout-done`
