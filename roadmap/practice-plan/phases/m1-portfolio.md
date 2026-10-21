# M1 作品集整理（4 次迭代）

- **階段目標**：把 P00–P07 的成果整理成第三方能重現、面試官能在 10 分鐘內理解的作品。
- **前置**：P07。
- **為什麼放在這裡**：P00–P07 已涵蓋付款生命週期、可靠性、帳本、對帳、出款，足以回答大部分付款後端面試題。先整理再往下做，可以開始投履歷，也能讓你回頭看清楚哪些地方其實還不懂。

這個階段不加新功能。

---

### I-M1.01 一行啟動與 seed

- **目標**：乾淨的機器上 `make demo-up` 就能啟動所有服務、跑 migration、建立 demo 商戶與 key、產生幾筆示範付款。
- **為什麼**：面試官或審查者不會花一小時設定環境。
- **不做**：雲端部署。
- **先想清楚**：
  1. Stripe sandbox key 要怎麼讓別人提供（不能 commit）？沒有 key 時，能不能全部用 Mock PSP 跑？
  2. seed 資料要包含哪些情境才能支撐 demo？
- **完成條件**：在另一台機器或乾淨的容器裡照 README 操作，10 分鐘內看到 demo 資料。
- **常見陷阱**：README 依賴你本機已安裝的工具。
- **Survey**：developer experience README、make demo
- **產出**：`README.md`（架構圖、啟動步驟、證據索引）、`scripts/seed`

### I-M1.02 故障場景證據

- **目標**：把 ProjectRoadmap 的 12 個故障場景整理成 `docs/evidence/`，每個都有 Inject / Observe / Expected / Actual / Trace，以及一行重跑指令。
- **為什麼**：「我處理過重複扣款」不如「這是重現指令和 trace」。
- **不做**：新的場景（P16 會補更多）。
- **先想清楚**：
  1. 哪些場景已經在前面的階段留過證據？哪些還缺？
  2. 證據要區分自動化測試、sandbox 人工驗證、只有設計三種狀態。
- **完成條件**：`make scenarios` 依序重跑所有自動化場景並輸出摘要表；12 個場景都有文件；對照 [`appendix-survey-keywords.md`](../appendix-survey-keywords.md) 第 4.2 節，架構文件第 8 節的 8 個必測場景都已涵蓋。
- **常見陷阱**：把沒有實際跑過的場景寫成已完成。
- **Survey**：failure injection report、game day
- **產出**：`docs/evidence/README.md`（索引表）

### I-M1.03 Demo 腳本

- **目標**：寫出 5–10 分鐘的 demo 腳本並錄一次影片：結帳與 3DS → 請款與退款 → 注入逾時與重複 webhook → 帳本 100 = 97 + 3 → 對帳差異 → 併發出款。
- **為什麼**：面試時的 demo 要能說出「我在防什麼」，而不只是「我做了什麼」。
- **不做**：剪輯。
- **先想清楚**：
  1. 每一段要讓觀眾看到哪個證據（狀態、trace、分錄、break 報告）？
  2. demo 中途失敗時的備案？
- **完成條件**：`docs/demo-script.md` 與一段錄影。
- **常見陷阱**：demo 只展示成功路徑。
- **Survey**：technical demo script
- **產出**：demo 腳本、錄影連結

### I-M1.04 ADR、面試題與履歷描述

- **目標**：整理所有 ADR 的索引；把 P00–P07 的階段檢核面試題寫成答案；寫出 3–5 條履歷描述。
- **為什麼**：面試時你要能把設計決策與取捨講清楚，ADR 就是你的講稿。
- **不做**：誇大範圍（Sandbox / Mock 要明說）。
- **先想清楚**：
  1. 哪三個設計決策最能代表你對付款系統的理解？
  2. 履歷描述要用可驗證的事實（例如「併發 100 次同 key 請求不重複扣款，附測試」），而不是形容詞。
- **完成條件**：`docs/adr/README.md`；`docs/interview-qa.md`；履歷描述草稿。
- **常見陷阱**：把 Mock 的壓測數字寫成生產等級效能。
- **Survey**：architecture decision records、STAR method
- **產出**：上述文件、tag `m1-portfolio-done`

## 檢核

- 讓一位沒看過這個專案的人照 README 跑起來，記錄他卡住的地方並修正。
- 自己對著錄影講一次，每個設計決策都能說出「放棄了什麼」。
