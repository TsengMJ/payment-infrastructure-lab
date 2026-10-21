# P16 安全與可靠性收尾（9 次迭代）

- **對應模組**：#18 Security, Compliance & Reliability
- **階段目標**：系統有威脅模型、secret 與金鑰管理、敏感資料自動掃描、每模組最小權限、可用的告警、可封存的大表、驗證過的備份還原、故障演練與壓測報告。
- **前置**：P00–P15。
- **結束時的架構**：每個模組一個 DB role（只對自己的 schema 有寫入權）；Vault 或 SOPS 管理 secret；告警與 dashboard；ledger 與事件表分區。

前面每個階段都已經做了一部分安全（驗簽、隔離、遮罩、PCI 邊界）。這個階段把它們系統化，並補上「出事時能恢復」的部分。也是 P17 抽出服務前的最後準備：沒有每模組 DB role，就不算真的有資料所有權。

---

### I-16.01 威脅模型

- **目標**：用 STRIDE 對整個系統做威脅模型，列出攻擊面、現有控制、缺口與優先順序。
- **為什麼**：安全工作要從「哪裡最可能出事」開始排序，而不是想到什麼做什麼。
- **不做**：滲透測試。
- **先想清楚**：
  1. 資料流圖上有哪些信任邊界（商戶、付款者、PSP、vendor、內部人員）？
  2. 最有價值的攻擊目標是什麼（出款帳戶、API key、退款）？
- **完成條件**：`docs/threat-model.md`，至少 15 個威脅，每個有對應控制或待辦。
- **常見陷阱**：只列技術漏洞，忽略內部人員與商業流程濫用。
- **Survey**：STRIDE、OWASP threat modeling、payment fraud vectors
- **產出**：威脅模型

### I-16.02 Secret 管理與金鑰加密

- **目標**：PSP 金鑰、webhook secret、資料庫密碼由 Vault（或 SOPS）管理；個資欄位改用信封加密（資料金鑰由 KMS 保護），可輪替。
- **為什麼**：環境變數裡的 secret 很容易外洩；欄位加密的金鑰如果和資料放在一起就沒有意義。
- **不做**：HSM。
- **先想清楚**：
  1. 信封加密是什麼？輪替主金鑰時要不要重新加密所有資料？
  2. 應用程式啟動時拿不到 secret，要怎麼失敗？
- **完成條件**：測試：輪替資料金鑰後新舊資料都能解密；資料庫中只有密文。
- **常見陷阱**：把 KMS 金鑰 ID 與加密資料當作安全邊界，卻讓所有服務都有解密權限。
- **Survey**：envelope encryption、HashiCorp Vault transit、SOPS、LocalStack KMS
- **產出**：secret 載入、加密元件

### I-16.03 敏感資料掃描自動化

- **目標**：CI 與定期 job 掃描日誌、資料庫、trace、錯誤回應、測試 fixtures，偵測卡號、CVV、email、證件號碼、secret。
- **為什麼**：把 I-11.07 的 PAN 掃描擴大到所有敏感資料，並持續執行。
- **不做**：DLP 產品。
- **先想清楚**：
  1. 每種敏感資料的偵測規則？誤報怎麼處理？
  2. 偵測到時的處理流程？
- **完成條件**：故意寫入一筆含 email 的日誌，掃描失敗；修正後通過。
- **常見陷阱**：只掃描資料庫，不掃描日誌與 trace。
- **Survey**：PII detection、gitleaks、trufflehog
- **產出**：掃描 job

### I-16.04 每模組 DB role 與最小權限

- **目標**：每個模組使用自己的資料庫角色，只對自己的 schema 有寫入權；ledger 的 postings 只有 INSERT 權限。
- **為什麼**：把 Monolith 的資料所有權從「約定」變成「權限」；也是 P17 抽出的前置條件。
- **不做**：獨立資料庫實例。
- **先想清楚**：
  1. 一個行程內有多個模組，怎麼讓每個模組使用不同的連線角色？
  2. migration 要用哪個角色執行？
  3. 報表 read model 需要哪些讀權限？
- **完成條件**：測試：payment 模組的連線嘗試寫入 ledger 表時被資料庫拒絕。
- **常見陷阱**：所有模組共用 superuser 連線。
- **Survey**：Postgres roles and privileges、least privilege
- **產出**：DB role migration、連線池設定、ADR 0026「每模組 DB role」

### I-16.05 告警與 dashboard

- **目標**：定義 SLO 與告警：授權成功率、PSP 錯誤率、UNKNOWN 數量、webhook 積壓、ledger 過帳延遲、對帳差異率、出款失敗率；建立對應的 Grafana dashboard。
- **為什麼**：架構文件第 7 節列出的可觀測性指標；沒有告警，故障就只能靠商戶來回報。
- **不做**：on-call 輪值。
- **先想清楚**：
  1. 哪些指標是症狀（影響商戶），哪些是原因？告警應該針對哪一種？
  2. 每個告警都要有 runbook 嗎？
- **完成條件**：每個告警都能用故障注入觸發；每個告警有對應的 runbook。
- **常見陷阱**：告警太多，最後沒人看。
- **Survey**：SLO burn rate alerts、Google SRE workbook alerting、runbook
- **產出**：告警規則、dashboard、`docs/runbooks/`

### I-16.06 大表分區與封存

- **目標**：ledger postings、inbox、outbox、webhook attempts 依時間分區；舊分區可封存或刪除（依保存政策）。
- **為什麼**：這些表只增不減；沒有分區，查詢、備份、索引都會越來越慢。
- **不做**：分片（sharding）。
- **先想清楚**：
  1. 哪些表可以刪舊資料（outbox），哪些必須長期保存（ledger）？
  2. 分區鍵選什麼？跨分區的唯一約束怎麼處理（冪等 key）？
  3. 把既有大表改成分區表的遷移步驟？
- **完成條件**：測試：新資料寫入正確分區；舊分區封存後查詢仍正確；冪等約束仍有效。
- **常見陷阱**：分區後唯一約束失效，冪等被破壞。
- **Survey**：Postgres declarative partitioning、pg_partman、unique constraints partitioned tables
- **產出**：分區 migration、封存 job

### I-16.07 備份與還原演練

- **目標**：設定 Postgres 時間點還原（PITR）；實際還原到另一個環境並驗證帳本完整；寫出 RPO 與 RTO。
- **為什麼**：沒有演練過的備份不能算有備份。
- **不做**：跨區域災難復原。
- **先想清楚**：
  1. 還原到某個時間點後，PSP 那邊已經發生的交易怎麼辦？（提示：對帳）
  2. 還原後要驗證哪些不變量？
- **完成條件**：還原演練報告：還原耗時、資料遺失範圍、還原後 ledger 平衡檢查與對帳結果。
- **常見陷阱**：只驗證備份檔存在。
- **Survey**：Postgres PITR、pgBackRest、WAL-G、RPO RTO
- **產出**：`docs/runbooks/restore.md`、演練報告

### I-16.08 故障演練

- **目標**：在整個系統上執行故障演練：kill worker、Postgres 重啟、RabbitMQ 停止、Redis 停止、PSP 全面逾時、vendor 停止；記錄每個場景的觀察與恢復。
- **為什麼**：前面每個階段都測過單點故障；這次看整體是否在組合故障下仍守住資金不變量。
- **不做**：生產環境演練。
- **先想清楚**：
  1. 每個場景的預期行為是什麼？哪些功能應降級、哪些應停止？
  2. 演練後要檢查哪些不變量？
- **完成條件**：至少 8 個場景的演練紀錄（沿用 `01-iteration-template.md` 的證據格式），演練後帳本平衡、無重複扣款、無超額出款。
- **常見陷阱**：演練只看服務有沒有恢復，不檢查資金正確性。
- **Survey**：chaos engineering、game day、toxiproxy
- **產出**：`docs/evidence/game-day-*.md`

### I-16.09 k6 壓測與報告（含階段檢核）

- **目標**：用 k6 與 Mock PSP 測量建立付款、ledger 過帳、webhook 消費、對帳的吞吐與延遲，並寫報告。
- **為什麼**：知道系統的瓶頸在哪，也知道你的數字在什麼條件下才成立。
- **不做**：對真實 PSP 壓測。
- **先想清楚**：
  1. 測試環境要記錄哪些資訊（CPU、記憶體、資料量、Mock 延遲）？
  2. 瓶頸出現時，你怎麼判斷是資料庫鎖、連線池還是應用程式？
- **完成條件**：`docs/benchmark-report.md`，包含 ProjectRoadmap 的 lab 目標表格與實際結果，以及至少一個瓶頸分析。
- **常見陷阱**：把 Mock 的數字寫成 PSP 的效能。
- **設計模式**：Backpressure / load shedding（壓到超過容量時，系統是有控制地拒絕，還是全部變慢？）；api 跑兩個實例，確認沒有依賴行程內狀態
- **Survey**：k6、Postgres lock contention、pg_stat_statements
- **產出**：k6 腳本、報告、`docs/notes/p16-review.md`

## 階段檢核

- **面試題**：
  1. 你的系統最主要的威脅是什麼？
  2. 個資欄位怎麼加密？金鑰怎麼輪替？
  3. 你怎麼知道備份可以還原？還原後 PSP 那邊的交易怎麼辦？
  4. 你的系統瓶頸在哪？怎麼找到的？
- **故障場景**：組合故障演練、還原演練。
- **tag**：`p16-security-done`
