# P00 基礎建設（10 次迭代）

- **對應模組**：#18 的基礎部分（可觀測性、CI、安全掃描）
- **階段目標**：一個什麼業務都還沒有、但邊界規則、測試、可觀測性、CI 都已就位的 repo。
- **前置**：無。
- **結束時的架構**：`cmd/api`、`cmd/worker` 能啟動；Postgres、OTel collector、Prometheus、Grafana 在 Docker Compose 裡；CI 綠燈。

為什麼要先花 10 次迭代做這些：付款系統的除錯幾乎都靠日誌、trace 和可重跑的測試。等到 P03 做故障注入時才補，會變成一邊修 bug 一邊搭工具。

---

### I-00.01 Repo 骨架與 Go module

- **目標**：`go build ./...` 與 `make test` 在空專案上成功。
- **為什麼**：之後每一次迭代都靠同一組指令驗證，指令要先固定。
- **不做**：任何業務程式、資料庫。
- **先想清楚**：
  1. Go module path 怎麼取？之後 P17 改成多 module 時會受什麼影響？
  2. Makefile 要提供哪些固定 target（`build`、`test`、`lint`、`up`、`down`）？
  3. Go 版本怎麼釘住（`go.mod` 的 `go` 與 `toolchain`）？
- **完成條件**：`make build`、`make test` 在乾淨 clone 上成功；README 寫明需要的工具版本。
- **常見陷阱**：把 Makefile 寫成只在自己機器能跑（寫死路徑、依賴全域安裝的工具）。
- **Survey**：Go modules、toolchain directive、Makefile phony targets
- **產出**：`go.mod`、`Makefile`、`README.md`

### I-00.02 目錄結構與 import 邊界檢查

- **目標**：建立 `cmd/`、`internal/<module>` 目錄，並讓 CI 擋下違規的跨模組 import。
- **為什麼**：Modular Monolith 的邊界如果只靠自律，幾個月後一定會被打破；P17 抽服務時會付出代價。
- **不做**：模組裡的實際程式。
- **先想清楚**：
  1. 模組對外公開什麼？`internal/<module>/api.go` 與 `internal/<module>/internal/` 的分工？
  2. 用 depguard 還是 go-arch-lint？兩者能表達的規則差在哪？
  3. `internal/platform` 可以放什麼、不可以放什麼？
- **完成條件**：故意寫一個 `merchant` import `ledger/internal` 的檔案，`make lint` 失敗；移除後通過。
- **常見陷阱**：把 platform 變成垃圾桶，業務邏輯流進共用套件。
- **設計模式**：Modular Monolith、Ports & Adapters
- **Survey**：Go internal packages、modular monolith Go、depguard、go-arch-lint
- **產出**：目錄、lint 設定、ADR 0001「模組邊界規則」

### I-00.03 Docker Compose 本機環境

- **目標**：`make up` 啟動 Postgres，並有健康檢查。
- **為什麼**：所有整合測試與 demo 都依賴可重現的本機環境。
- **不做**：Redis、RabbitMQ（之後需要時才加）。
- **先想清楚**：
  1. 資料卷要不要持久化？測試和開發要不要用不同資料庫？
  2. 映像版本要釘到哪個層級（major 還是 patch）？
  3. 用 compose profile 區分「最小」與「含監控」嗎？
- **完成條件**：`make up && make down` 可重複執行；`psql` 能連上。
- **常見陷阱**：使用 `latest` 標籤；把密碼寫進 compose 檔後 commit。
- **Survey**：docker compose healthcheck、compose profiles
- **產出**：`deploy/compose/docker-compose.yml`

### I-00.04 Migration 與每個模組一個 schema

- **目標**：migration 工具能為每個模組建立自己的 Postgres schema，並可以 up / down。
- **為什麼**：資料所有權從第一天就要清楚，這是之後抽出服務的前提。
- **不做**：業務資料表。
- **先想清楚**：
  1. golang-migrate、goose、atlas 各自怎麼處理多個目錄？
  2. migration 要不要寫 down？在付款系統裡 down 是否真的安全？
  3. 誰執行 migration：應用程式啟動時，還是獨立指令？
- **完成條件**：`make migrate-up` 建出 `merchant`、`payment` 兩個空 schema；重跑不報錯。
- **常見陷阱**：應用程式啟動時自動跑 migration，多個副本同時啟動會互相衝突。
- **Survey**：golang-migrate、goose、Postgres schema、expand contract migration
- **產出**：`migrations/`、`make migrate-up`、ADR 0002「migration 工具與執行時機」

### I-00.05 設定與 secret

- **目標**：設定從環境變數載入並驗證；缺少必要設定時啟動失敗並說明缺了什麼。
- **為什麼**：PSP 金鑰、webhook secret 一旦進 repo 就很難收回。
- **不做**：Vault 或 KMS（P16）。
- **先想清楚**：
  1. 哪些設定是 secret、哪些不是？要不要分開載入？
  2. 設定物件印到日誌時，secret 怎麼遮罩？
  3. `.env` 檔要不要 commit？範例檔怎麼放？
- **完成條件**：缺 `DATABASE_URL` 時啟動失敗且訊息清楚；把設定印出來時 secret 顯示為 `***`。
- **常見陷阱**：用預設值掩蓋缺失的 secret，導致在錯誤環境下靜默運作。
- **Survey**：twelve-factor config、Go struct validation、secret redaction
- **產出**：`internal/platform/config`、`.env.example`、`.gitignore`

### I-00.06 結構化日誌與敏感欄位遮罩

- **目標**：用 `log/slog` 輸出 JSON 日誌，並能遮罩指定欄位。
- **為什麼**：付款系統的日誌常被拿去查交易，也常意外寫進卡號、email、token。
- **不做**：集中收集日誌（Loki 等）。之後若要做，見選做的 I-00.09a，不計入這 10 次。
- **先想清楚**：
  1. 每行日誌必備哪些欄位（時間、層級、服務、request_id、trace_id）？
  2. 遮罩是在寫入時做，還是靠呼叫端自律？哪一個比較可靠？
  3. 錯誤要記錄多少細節才夠除錯，又不洩漏資料？
- **完成條件**：測試證明含有 `card_number`、`authorization` header 的物件寫入日誌後被遮罩。
- **常見陷阱**：直接 `log` 整個 HTTP request 或外部回應 body。
- **Survey**：Go slog、log redaction、PCI DSS logging
- **產出**：`internal/platform/log`

### I-00.07 HTTP server 骨架與健康檢查

- **目標**：`cmd/api` 啟動 HTTP server，提供 `/healthz` 與 `/readyz`，收到 SIGTERM 時優雅關閉。
- **為什麼**：未來 worker 與 API 都需要正確關閉，否則會留下處理到一半的付款。
- **不做**：路由框架之外的 middleware。
- **先想清楚**：
  1. liveness 與 readiness 的差別？readiness 要不要檢查資料庫？
  2. 優雅關閉時，正在處理的請求給多少時間？
  3. 用標準庫 `net/http` 還是 chi 之類的 router？理由寫進筆記。
- **完成條件**：整合測試：送出一個慢請求後發 SIGTERM，請求仍然完成，server 才結束。
- **常見陷阱**：readiness 永遠回 200，資料庫斷了還繼續接流量。
- **設計模式**：Composition root（在 `cmd/api` 組裝依賴）、Middleware chain、優雅關閉、Health check
- **Survey**：Go http.Server Shutdown、liveness vs readiness probe
- **產出**：`cmd/api/main.go`、`cmd/worker/main.go`（只會啟動與關閉）

### I-00.08 測試骨架：單元與整合測試

- **目標**：整合測試透過 testcontainers 啟動真的 Postgres 並跑 migration。
- **為什麼**：付款系統的關鍵不變量（唯一約束、鎖、交易）只有真的資料庫才測得出來，mock 資料庫會給你錯誤的安全感。
- **不做**：效能測試。
- **先想清楚**：
  1. 單元測試與整合測試怎麼分（build tag、目錄、命名）？
  2. 每個測試用獨立資料庫、獨立 schema，還是 transaction rollback？各自的取捨？
  3. 測試能不能平行跑？
- **完成條件**：`make test-unit` 不需要 Docker；`make test-integration` 啟動 Postgres、跑 migration、跑一個查詢測試。
- **常見陷阱**：測試之間共用資料導致順序相依、偶發失敗。
- **設計模式**：Fake 與 Mock 的區分、Clock 注入（現在就定一個 Clock 介面，P02 的授權過期、P06 的時間窗、P13 的計費週期都會用到）
- **Survey**：testcontainers-go、Go build tags、table-driven tests
- **產出**：`internal/platform/testutil`

### I-00.09 OpenTelemetry trace 與 metrics 基線

- **目標**：一個 HTTP 請求在 Grafana 裡能看到 trace，`/metrics` 能被 Prometheus 抓到。
- **為什麼**：P03 的故障注入、P17 的跨服務呼叫都要靠 trace 才能看懂；越晚接越難補。
- **不做**：告警規則（P16）、業務指標、把日誌送進 Loki（選做見 I-00.09a）。
- **先想清楚**：
  1. trace_id 怎麼寫進日誌，讓日誌與 trace 能互相跳轉？
  2. 資料庫查詢要不要建 span？會不會把 SQL 參數（可能含個資）寫進 span？
  3. metrics 用 OTel 匯出，還是直接用 Prometheus client？
- **完成條件**：`make up-observability` 後，打一次 `/healthz`，Grafana 看得到 trace 與請求數指標；日誌裡有相同的 trace_id。
- **常見陷阱**：把高基數值（payment_id、merchant_id）當成 metric label。
- **Survey**：OpenTelemetry Go、OTel collector、Prometheus cardinality、Grafana Tempo
- **產出**：`internal/platform/otel`、compose 的 observability profile

### I-00.09a 選做：把 JSON 日誌送進 Grafana Loki

- **狀態**：選做。現在不做。不計入 P00 的 10 次迭代，也不擋 `p00-foundation-done`。
- **為什麼先不做**：I-00.06 明確排除集中收集。I-00.09 要證明的是標準輸出裡的 `trace_id` 能對上 Grafana 的 trace，日誌本身留在標準輸出。
- **之後再做的前提**：JSON 日誌與遮罩已經穩定。收集器把標準輸出送進 Loki，`trace_id` 做成可查欄位，Grafana 能從同一條 trace 跳到日誌。
- **現在做的代價**：在 P11、P16 的敏感資料掃描守住日誌之前，先開一個長期日誌副本。
- **完成條件（將來做的時候）**：`make up-observability` 後，`/healthz` 的那一行日誌在 Grafana 裡查得到，而且 `trace_id` 與 Tempo 的 trace 相同。

### I-00.10 CI 與安全掃描（含階段檢核）

- **目標**：每個 PR 自動跑 lint、單元與整合測試、govulncheck、gosec、secret 掃描。
- **為什麼**：之後每次迭代都靠 CI 證明「沒有弄壞之前的東西」。
- **不做**：部署流程。
- **先想清楚**：
  1. 哪些檢查失敗要擋 PR，哪些只警告？
  2. secret 掃描誤報時怎麼處理（允許清單要不要審查）？
  3. 整合測試在 CI 裡怎麼啟動 Docker？
- **完成條件**：故意 commit 一個假 API key，CI 失敗；移除後通過。
- **常見陷阱**：為了讓 CI 綠燈而關掉檢查。
- **Survey**：GitHub Actions、golangci-lint、govulncheck、gosec、gitleaks
- **產出**：`.github/workflows/ci.yml`、`docs/notes/p00-review.md`

## 階段檢核

- **面試題**：
  1. 為什麼付款系統一開始選 Modular Monolith，而不是微服務？
  2. 你怎麼防止模組之間的依賴在幾個月後失控？
  3. 為什麼整合測試要用真的 Postgres？
  4. 你如何避免卡號或 token 出現在日誌裡？
- **故障場景**：無（本階段只準備工具）。
- **tag**：`p00-foundation-done`
