# パス解決ルール

このリポジトリは**モノレポ**。すべてのパスは**リポジトリルートからの相対パス**で解決する。

## 禁止

- 個人 PC の絶対パス (`/Users/xxx/...`, `/home/xxx/...`) のハードコード
- リポジトリ外への参照 (このリポジトリは自己完結することが前提)

## モノレポ内の主要パス

| 用途                                         | パス                                   |
| -------------------------------------------- | -------------------------------------- |
| OIDC OP 本体 (Go)                            | `core/`                                |
| サンプル A バックエンド (Go)                 | `examples/go-react/backend/`           |
| サンプル A フロントエンド (React)            | `examples/go-react/frontend/`          |
| サンプル B バックエンド (Kotlin/Spring Boot) | `examples/kotlin-nextjs/backend/`      |
| サンプル B フロントエンド (Next.js)          | `examples/kotlin-nextjs/frontend/`     |
| ドキュメント                                 | `docs/`                                |
| コンテキスト集約                             | `docs/context/`                        |
| 認可マトリクス正本                           | `docs/context/authorization/matrix.md` |
| 要求文書                                     | `docs/requirements/{N}/`               |
| 設計書                                       | `docs/specs/{N}/`                      |
| ADR                                          | `docs/adr/`                            |
| テンプレート                                 | `docs/templates/`                      |
| 開発環境                                     | `docker/`                              |
| ルール・スキル正本                           | `.rulesync/`                           |

## スキル / プロンプトでの参照

スキルの SKILL.md やプロンプト内では、上記パスをリポジトリルートからの相対パスで直書きしてよい。
ghq による解決やシェル変数展開は**不要**。

例:

```markdown
- 既存マイグレーション: `core/db/migrations/*.up.sql`
- 認可マスター: `docs/context/authorization/matrix.md`
- サンプル backend (Go): `examples/go-react/backend/internal/`
```

## 実装プロンプト (`docs/specs/{N}/prompts/`) での扱い

実装プロンプトは**自己完結型**とする。設計書からの内容コピーを基本とし、他ファイルへの参照は最小限にとどめる。
他のリポジトリ (このリポジトリ外) への参照は禁止。

## アーカイブ/ について

`アーカイブ/` 配下は **参考資料**で、`.gitignore` でコミット対象外。
スキル・ルール・コードがアーカイブを直接参照することは禁止 (スキルが汎用的に動作するため)。
ユーザーが明示的に参照を求めた場合のみ、Read で内容確認する。
