# payment-infrastructure-lab

用一連串小迭代實作一套付款系統（Modular Monolith，Go + PostgreSQL）。計劃與每次迭代的內容在 [`roadmap/practice-plan/`](roadmap/practice-plan/00-overview.md)。

## 需要的工具

| 工具 | 版本 | 用途 |
|---|---|---|
| Go | 1.27.2（`go.mod` 的 `toolchain`；較舊的 Go 1.21+ 會自動下載這一版） | 編譯與測試 |
| GNU make | 3.81 以上 | 固定指令入口 |
| Docker + Docker Compose | Compose v2 以上 | 本機 Postgres（`make up`） |
| Atlas | 1.3.0（`deploy/atlas/Dockerfile` 以 `arigaio/atlas:1.3.0-alpine` 建本地 image，不需全域安裝） | schema migration |

其他 Go 工具（例如 lint）由 `go.mod` 釘住版本，透過 `go tool` 執行，不需要全域安裝。

## 指令

| 指令 | 做什麼 |
|---|---|
| `make build` | `go build ./...` |
| `make test` | `go test ./...` |
| `make lint` | 模組 import 邊界檢查 |
| `make up` | 啟動本機 Postgres，等到健康檢查通過 |
| `make down` | 停止本機環境（保留資料） |
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

