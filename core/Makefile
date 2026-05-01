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

lint: ## go vet によるリント (golangci-lint 導入は M0.2 以降)
	go vet ./...

clean: ## ビルド成果物を削除
	rm -rf $(BIN_DIR) coverage.txt
