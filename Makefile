.PHONY: help dev-init dev-up dev-down dev-logs rulesync format format-check hooks-install hooks-uninstall

# rulesync: バージョン固定
RULESYNC_VERSION ?= 8.11.0
RULESYNC_FEATURES ?= rules,ignore,mcp,subagents,commands,skills,hooks,permissions
RULESYNC_TARGETS ?= claudecode,codexcli

# prettier: バージョン固定
PRETTIER_VERSION ?= 3.8.3

help: ## ヘルプ
	@awk 'BEGIN {FS = ":.*##"} /^([a-zA-Z0-9_-]+):.*##/ { printf "\033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

dev-init: ## docker/.env を docker/.env.sample からコピー (初回のみ)
	@test -f docker/.env || (cp docker/.env.sample docker/.env && echo "✓ docker/.env を作成しました (必要なら編集してください)")

dev-up: ## 開発環境を起動 (docker compose)
	@test -f docker/.env || (echo "✗ docker/.env がありません。先に 'make dev-init' を実行してください" && exit 1)
	docker compose -f docker/compose.yaml --env-file docker/.env up -d

dev-down: ## 開発環境を停止
	docker compose -f docker/compose.yaml --env-file docker/.env down

dev-logs: ## コンテナログを追従
	docker compose -f docker/compose.yaml --env-file docker/.env logs -f

rulesync: ## rulesync: .rulesync/ から各 AI ツール向けに生成 (.rulesync/ → .claude/ 等)
	npx -y rulesync@$(RULESYNC_VERSION) generate --targets $(RULESYNC_TARGETS) --features $(RULESYNC_FEATURES)

format: ## prettier: 対象ファイルを整形 (.prettierignore で除外管理)
	npx -y prettier@$(PRETTIER_VERSION) --write .

format-check: ## prettier: 整形差分をチェックのみ (CI / 事前検証用)
	npx -y prettier@$(PRETTIER_VERSION) --check .

hooks-install: ## git pre-commit hook を有効化 (.githooks/ を使う)
	chmod +x .githooks/pre-commit
	git config core.hooksPath .githooks
	@echo "✓ pre-commit hook を有効化しました (.githooks/pre-commit)"

hooks-uninstall: ## git hooks 設定を解除 (デフォルトの .git/hooks に戻す)
	git config --unset core.hooksPath
	@echo "✓ git hooks 設定を解除しました"
