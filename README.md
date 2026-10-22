# payment-infrastructure-lab

用一連串小迭代實作一套付款系統（Modular Monolith，Go + PostgreSQL）。計劃與每次迭代的內容在 [`roadmap/practice-plan/`](roadmap/practice-plan/00-overview.md)。

## 需要的工具

| 工具 | 版本 | 用途 |
|---|---|---|
| Go | 1.27.3（`go.mod` 的 `toolchain`；較舊的 Go 1.21+ 會自動下載這一版） | 編譯與測試 |
| GNU make | 3.81 以上 | 固定指令入口 |
| Docker + Docker Compose | Compose v2 以上 | 本機 Postgres（`make up`） |
| Atlas | 1.3.0（`deploy/atlas/Dockerfile` 以 `arigaio/atlas:1.3.0-alpine` 建本地 image，不需全域安裝） | schema migration |
| gitleaks | `ghcr.io/gitleaks/gitleaks:v8.30.1`（`make secrets` 用 Docker 跑，不需全域安裝） | 掃目前檔案裡的 secret |

其他 Go 工具（例如 lint）由 `go.mod` 釘住版本，透過 `go tool` 執行，不需要全域安裝。

## 指令

| 指令 | 做什麼 |
|---|---|
| `make build` | `go build ./...` |
| `make test` / `make test-unit` | 單元測試。不需要 Docker |
| `make test-integration` | 用 testcontainers 起 Postgres、跑 migration、再跑整合測試 |
| `make lint` | import 邊界，再加上 govet、staticcheck、errcheck、unused、ineffassign |
| `make vuln` | `govulncheck ./...` |
| `make gosec` | gosec。有發現就失敗 |
| `make secrets` | gitleaks `v8.30.1` 只掃目前檔案，不掃 git 歷史 |
| `make up` | 啟動本機 Postgres，等到健康檢查通過 |
| `make up-observability` | 啟動 Postgres，再加上 OpenTelemetry Collector、Tempo、Prometheus、Grafana |
| `make down` | 停止本機環境（含 observability profile，保留資料） |
| `make migrate-up` | 套用 `merchant`、`payment` 的 migration。已套用過再跑會成功結束 |
| `make migrate-down` | 撤銷這兩個目錄裡已套用的 migration。只允許連到 `127.0.0.1`；I-17.03 會刪掉這個指令 |
| `make migrate-diff MODULE=<module> NAME=<change>` | 依 `schema/<module>.hcl` 產生下一支 migration。`MODULE` 是 `merchant` 或 `payment` |

## 本機資料庫

第一次使用前複製本機設定（已在 `.gitignore`，不會被 commit）：

```sh
cp deploy/compose/env.example deploy/compose/.env
make up
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env \
  exec postgres psql -U payment -d payment_dev -c 'select 1'
```

Postgres 只綁 `127.0.0.1:5432`（可用 `POSTGRES_PORT` 改）。要連資料一起刪除時，手動執行上面的 `docker compose ... down -v`。

## Migration

`make up` 之後：

```sh
make migrate-up
```

每個模組的 desired state 在 `schema/<module>.hcl`，產生出來的 SQL 在 `migrations/<module>/`。改 schema 時改 hcl，再執行 `make migrate-diff`，不要手改已經提交的 SQL。版本表在 `atlas_merchant`、`atlas_payment`，不在業務 schema 裡。

`make migrate-diff` 和 `make migrate-down` 會用 Docker 起一個拋棄式的 Postgres 18 來計算 plan，所以需要 Docker socket。應用程式啟動時不跑 migration。

## 應用程式設定

`.env` 不會被 commit。`DATABASE_URL` 沒有預設值，缺了就無法啟動，錯誤會指出變數名稱。範例裡的密碼 `local-dev-only` 只給本機 compose 用。

日誌是 JSON。`card_number`、`authorization` 這類欄位在寫出時會變成 `***`，不要把整份 HTTP request 或外部回應 body 丟進日誌。

已存在的環境變數優先於 `.env`。沒有 `.env` 也可以，只要環境裡有 `DATABASE_URL`。`/healthz` 不查資料庫；`/readyz` 會 ping，資料庫不通時回 503。`cmd/worker` 目前只會啟動、寫一行日誌，並在 SIGTERM 或 SIGINT 時退出。

## 本機追蹤與指標

`make up` 不會啟動 Grafana。要看 trace 與請求數：

```sh
make up-observability
```

Grafana 在 `http://127.0.0.1:3000`，Prometheus 與 Tempo 的 datasource 已由檔案寫入，不用在畫面上新增。Prometheus 對宿主機開在 `http://127.0.0.1:9091`（容器內仍是 9090；宿主機的 9090 常被本機 proxy 佔用）。API 仍在宿主機跑。Prometheus 抓 `host.docker.internal:8081/metrics`。公開的 API 埠不提供 `/metrics`。把 `.env` 裡的 `OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317` 打開後再啟動 API，trace 才會送到 collector。沒設這個變數時，行程仍會產生 span，日誌裡仍有 `trace_id`。日誌留在標準輸出，這次不送進 Loki。請求數在 Grafana 查 `http_server_request_duration_seconds_count`，label 用 `http_route`，不要用原始路徑。


```sh
cp .env.example .env
go run ./cmd/api
```

另一個終端機：

```sh
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/healthz
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/readyz
go run ./cmd/worker
```

