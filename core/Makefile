.PHONY: help build run test test-cover lint clean

# 出力先
BIN_DIR := bin
BIN := $(BIN_DIR)/core

help: ## ヘルプを表示
	@awk 'BEGIN {FS = ":.*##"} /^([a-zA-Z0-9_-]+):.*##/ { printf "\033[36m%-14s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## バイナリをビルド (bin/core)
	go build -o $(BIN) ./cmd/core

run: build ## ビルドして起動 (CORE_PORT などは環境変数で指定)
	./$(BIN)

test: ## ユニットテスト実行 (-race)
	go test -race ./...

test-cover: ## カバレッジ計測
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt | tail -1

lint: ## go vet + プロジェクト固有禁止チェック (log.Fatal*)
	@echo "==> go vet"
	@go vet ./...
	@echo "==> log.Fatal* check (project policy: forbidden in core/)"
	@grep -rEn --include='*.go' --exclude='*_test.go' '\blog\.Fatal[a-zA-Z]*\(' . ; rc=$$?; \
	case $$rc in \
		0) echo "ERROR: log.Fatal* is forbidden in core/. Use logger.Error + os.Exit(1) instead."; exit 1 ;; \
		1) : ;; \
		*) echo "ERROR: grep exited with $$rc (unexpected)"; exit 1 ;; \
	esac

clean: ## ビルド成果物を削除
	rm -rf $(BIN_DIR) coverage.txt
