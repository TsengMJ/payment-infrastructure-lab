# 架構

P00 結束時的形狀。一個 Go module，兩個行程。

## 行程

- `cmd/api`：HTTP。`/healthz` 不查資料庫，`/readyz` 只 ping Postgres，不看 Redis。`/metrics` 在 `METRICS_ADDR`（預設 `:8081`），只接受 loopback、私網與 link-local，不掛在公開埠。
- `cmd/worker`：讀設定、寫一行日誌，收到 SIGTERM 或 SIGINT 就退出。還沒有工作迴圈。

兩個行程都連同一份本機 Postgres。業務模組還沒有表，也還沒有跨模組呼叫。

## 資料

Postgres `payment_dev` 上有兩個 schema，都還沒有表：

| schema | 擁有者 | 表 |
|---|---|---|
| `merchant` | merchant 模組 | 無 |
| `payment` | payment 模組 | 無 |

Migration 由 Atlas 分開套用，行程啟動時不自己遷移。

## 可觀測性

`make up` 只啟動 Postgres。`make up-observability` 才加上這個 profile：

- OpenTelemetry Collector
- Tempo
- Prometheus
- Grafana

API 在宿主機跑。有設 `OTEL_EXPORTER_OTLP_ENDPOINT` 時，span 送到 collector，再進 Tempo。日誌留在標準輸出，`trace_id` 與 Tempo 裡的同一條 trace 相同。沒設 endpoint 時，行程仍產生 span，日誌仍有 `trace_id`，只是不往外送。

Prometheus 抓宿主機的 `:8081/metrics`。公開的 API 埠沒有這個路徑。label 用路由模板（`http.route`）以及 method、status 這類低基數值。不放原始 URL、`payment_id`、`merchant_id`、API key 或 `trace_id`。

日誌進 Loki，以及高流量時的 OTLP 取樣、batcher、中繼 collector、降級，記在 [`optional-todos.md`](../roadmap/practice-plan/optional-todos.md)。尚未做，不擋 `p00-foundation-done`。

## 故障場景

本階段無。
