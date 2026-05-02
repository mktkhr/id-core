.PHONY: help build run test test-cover test-integration lint clean \
        migrate-install migrate-create migrate-up migrate-up-one \
        migrate-down migrate-down-all migrate-force migrate-version migrate-status

# 出力先
BIN_DIR := bin
BIN := $(BIN_DIR)/core

# golang-migrate CLI バージョン (M0.3 Q2 で確定)
MIGRATE_VERSION := v4.19.1

# DB_URL は CORE_DB_* 個別 env から組み立てる (M0.3 Q7)。
# DB_URL を直接 export することで上書き可能 (CI / 統合テスト等)。
DB_URL ?= postgres://$(CORE_DB_USER):$(CORE_DB_PASSWORD)@$(CORE_DB_HOST):$(CORE_DB_PORT)/$(CORE_DB_NAME)?sslmode=$(CORE_DB_SSLMODE)

# 統合テスト用 DB の DSN。テスト用 DB (id_core_test) を別建てにし、開発 DB との衝突を避ける。
TEST_DATABASE_URL ?= postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable

help: ## ヘルプを表示
	@awk 'BEGIN {FS = ":.*##"} /^([a-zA-Z0-9_-]+):.*##/ { printf "\033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## バイナリをビルド (bin/core)
	go build -o $(BIN) ./cmd/core

run: build ## ビルドして起動 (CORE_PORT などは環境変数で指定)
	./$(BIN)

test: ## ユニットテスト実行 (-race)
	go test -race ./...

test-cover: ## カバレッジ計測
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt | tail -1

test-integration: ## 統合テスト (DB 必要、build tag = integration)
	@echo "==> テスト用 DB を初期化 ($(TEST_DATABASE_URL))"
	@migrate -path db/migrations -database "$(TEST_DATABASE_URL)" drop -f
	@migrate -path db/migrations -database "$(TEST_DATABASE_URL)" up
	@echo "==> 統合テスト実行 (build tag = integration)"
	@TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_DB_REQUIRED=1 \
		go test -p 1 -race -v -tags integration ./...

lint: ## go vet + プロジェクト固有禁止チェック (log.Fatal* + マイグレーション連番衝突)
	@echo "==> go vet"
	@go vet ./...
	@echo "==> log.Fatal* check (project policy: forbidden in core/)"
	@grep -rEn --include='*.go' --exclude='*_test.go' '\blog\.Fatal[a-zA-Z]*\(' . ; rc=$$?; \
	case $$rc in \
		0) echo "ERROR: log.Fatal* is forbidden in core/. Use logger.Error + os.Exit(1) instead."; exit 1 ;; \
		1) : ;; \
		*) echo "ERROR: grep exited with $$rc (unexpected)"; exit 1 ;; \
	esac
	@echo "==> migration sequence collision check"
	@if [ -d db/migrations ]; then \
		dups=$$(ls db/migrations/*.up.sql 2>/dev/null | xargs -n1 basename | awk -F_ '{print $$1}' | sort | uniq -d); \
		if [ -n "$$dups" ]; then \
			echo "ERROR: duplicate migration sequence numbers detected: $$dups"; \
			exit 1; \
		fi; \
	fi

clean: ## ビルド成果物を削除
	rm -rf $(BIN_DIR) coverage.txt

# ----------------------------------------------------------------------------
# マイグレーション (golang-migrate v4)
# ----------------------------------------------------------------------------

migrate-install: ## golang-migrate CLI を $(go env GOPATH)/bin/migrate にインストール
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

migrate-create: ## 新規マイグレーションファイル雛形を生成 (NAME=<slug>)
	@if [ -z "$(NAME)" ]; then \
		echo "ERROR: NAME=<slug> を指定してください (例: make migrate-create NAME=add_users_table)"; \
		exit 1; \
	fi
	migrate create -ext sql -dir db/migrations -seq -digits 8 $(NAME)

migrate-up: ## 全 pending マイグレーションを適用 (DB_URL 必須)
	migrate -path db/migrations -database "$(DB_URL)" up

migrate-up-one: ## 次の 1 件のみ適用 (DB_URL 必須)
	migrate -path db/migrations -database "$(DB_URL)" up 1

migrate-down: ## 直近の 1 件をロールバック (DB_URL 必須)
	migrate -path db/migrations -database "$(DB_URL)" down 1

migrate-down-all: ## 全マイグレーションをロールバック (危険、DB_URL 必須)
	@echo "WARNING: 全マイグレーションをロールバックします。schema_smoke を含む全テーブルが削除されます。"
	migrate -path db/migrations -database "$(DB_URL)" down -all

migrate-force: ## dirty 状態の強制リセット (VERSION=<n> 必須)
	@if [ -z "$(VERSION)" ]; then \
		echo "ERROR: VERSION=<n> を指定してください (例: make migrate-force VERSION=1)"; \
		exit 1; \
	fi
	migrate -path db/migrations -database "$(DB_URL)" force $(VERSION)

migrate-version: ## 現在の schema_migrations.version を表示
	migrate -path db/migrations -database "$(DB_URL)" version

migrate-status: ## マイグレーション状態を表示 (no-version は graceful、接続/認証エラーは通常通り fail)
	@out=$$(migrate -path db/migrations -database "$(DB_URL)" version 2>&1); rc=$$?; \
	echo "$$out"; \
	if [ "$$rc" -ne 0 ]; then \
		case "$$out" in \
			*"no migration"*|*"NilVersion"*) exit 0 ;; \
			*) exit "$$rc" ;; \
		esac; \
	fi
