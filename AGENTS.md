# Threadly Agent Instructions

このファイルはリポジトリ全体に適用する、短いプロジェクト固有ルールです。詳細な手順は各READMEと `.agents/skills/` を参照してください。

## 適用範囲

- ユーザーの明示的な指示を最優先する。
- 下位ディレクトリにより近い `AGENTS.md` がある場合は、その指示を優先する。
- 現行コード、テスト、Migration、OpenAPI定義を確認して判断する。過去の設計や履歴を現行仕様とみなさない。

## プロジェクト構成

- `api/`: Go 1.25系、Gin、GORM、MySQL、AtlasによるAPI。
- `front/`: Node.js 22系、React、TypeScript、ViteによるFront。
- `api/internal/domain/`: ModelとRepository契約。
- `api/internal/usecase/`: 業務ルール、認証・認可、Unit of Work。
- `api/internal/interface/`: HTTP DTOとController。
- `api/internal/infra/`: DB、認証、Middleware、Route、Repository実装。
- `api/migrations/`: Atlas versioned migration。
- `front/src/orval/`、`api/docs/`: 生成物。手編集しない。

## 作業ルール

- 作業開始時に `git status --short --branch` を確認する。
- 既存の未コミット変更を上書きせず、依頼対象外の差分を変更しない。
- 秘密情報（`.env`、JWT secret、password、Cookie、token）を出力・コミット・コメントへ書かない。
- 変更は必要最小限にする。コミット、push、PR、Stack操作は明示的に依頼された場合だけ行う。

## 重要な設計境界

- Middlewareはsession Cookie認証の入口、Usecase/Serviceは業務上の認可・所有者境界を担当する。認証主体のUser IDをrequest bodyから受け取らない。
- Domain Modelを直接HTTP responseへ返さず、DTOで公開フィールドを定義する。内部情報を漏らさない。
- Usecase/Unit of WorkがTransaction境界と複数Repositoryの順序を決める。RepositoryはGORM query、lock、DBエラー変換、永続化操作を担当する。
- Transaction callback内ではcallbackから受け取ったtransaction-bound Repositoryだけを使う。`context.Context` はTransactionを伝播しない。
- Post・Comment・Likeなど複数の永続化操作を一つの成功単位にする場合は、同じUnit of Workで扱う。
- DBスキーマ変更はGORM Modelだけで完了させず、Atlas migrationを生成・確認・適用する。アプリ起動時に自動適用しない。

## 主な検証コマンド

Goコマンドは `api/` で実行する。

```sh
cd api
go test ./...
go test -race ./...
go vet ./...
make build
make vulncheck
```

Frontを変更した場合は `front/` で次を実行する。

```sh
npm run lint
npm run build
```

Swagger変更後は `cd api && make swag`、API契約変更後は続けて `cd front && npm run orval` を実行する。Integration testは専用の `test-db` と `TEST_DATABASE_DSN` を使い、DSNがない場合は未実施と報告する。

## コメント

コメントの追加・レビューは [threadly-code-comments Skill](.agents/skills/threadly-code-comments/SKILL.md) を参照する。このファイルにコメント規約を重複して記載しない。

## 検証結果の報告

- 実際に実行したコマンドだけを成功として報告する。
- `git diff --check` を共通確認とし、生成物・Swagger・Migrationを変更した場合は対応する生成・DB検証も行う。
- CI pending、Docker停止、DSN不足、手動確認未実施は、成功と混同せず明記する。
