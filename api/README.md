# Threadly API

## 起動

MySQLを起動し、Atlasでmigrationを適用したうえでAPIを実行します。

```sh
cd api
docker compose up -d db

# api/.env に DB_USER、DB_PASSWORD、DB_HOST、DB_PORT、DB、JWT_SECRET を設定
make tools
make migrate-apply

make dev
```

`make tools` は mockgen、swag、Air、Atlas CLIをインストールします。Atlas CLIはmacOSのHomebrewを利用します。`make dev` は Air を起動し、`cmd/`・`internal/`・`docs/` 配下の変更を検知すると API を再ビルド・再起動します。Air は Go 1.25 と互換性のある固定バージョンをインストールします。

`JWT_SECRET` は必須です。HS256 の署名鍵として利用するため、32バイト以上の秘密値を設定してください。設定項目は `.env.sample` にも記載しています。

`api/.env` がある場合、Makefileが `DB_USER`・`DB_PASSWORD`・`DB_HOST`・`DB_PORT`・`DB` から `DATABASE_URL` を自動生成します。そのため、ローカル開発では `DATABASE_URL` を毎回設定する必要はありません。別のDBを対象にする場合は、従来どおり `DATABASE_URL='mysql://...' make migrate-apply` のように明示指定できます。

## 認証

パスワードは Argon2id でハッシュ化して保存します。登録またはログインが成功すると、1時間有効な `HttpOnly; SameSite=Lax` セッションCookieが設定されます。HTTPS環境では `Secure` も有効になり、Cookie名は `__Host-threadly-session` です。JWT本体はレスポンスボディやブラウザJavaScriptへ返しません。

```sh
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -c threadly.cookies \
  -d '{"username":"alice","password":"correct-horse-battery"}'

curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -c threadly.cookies \
  -d '{"username":"alice","password":"correct-horse-battery"}'
```

以降の `/api/me` と `/api/posts` へのアクセスでは、Cookie jarを使用します。

```sh
curl http://localhost:8080/api/me \
  -b threadly.cookies

curl -X POST http://localhost:8080/api/posts \
  -b threadly.cookies \
  -H 'Content-Type: application/json' \
  -d '{"title":"hello","content":"world"}'

curl -X POST http://localhost:8080/api/auth/logout \
  -b threadly.cookies \
  -c threadly.cookies
```

`Secure` CookieはHTTPS接続でのみ送信されます。HTTPでローカル開発する場合は `api/.env` に `COOKIE_SECURE=false` を設定してください。この場合、`__Host-` 接頭辞は使えないためCookie名は `threadly-session` になります。本番環境では `COOKIE_SECURE=true` を維持し、`__Host-threadly-session` を使用してください。

Post の一覧取得・詳細取得は、JWTで認証されたUserが全Postを閲覧できます。作成時の投稿者はJWTのUser IDから設定され、更新・削除は投稿者本人に限定されます。レスポンスには公開User情報として `author.id` と `author.username` を含めます。`authorId` はリクエストから受け取りません。

UserとPostのIDは連番ではなくUUID文字列です。`/api/posts/{id}` の `id` にもUUIDを指定します。

詳細なリクエスト・レスポンスは `/swagger/index.html` を参照してください。

## テスト

通常のUnit・route testは、APIディレクトリで実行します。

```sh
go test ./...
go test -race ./...
go vet ./...
```

Integration testは、既存の開発用DBとは分離した `test-db` を起動し、DSNを指定して実行します。

```sh
docker compose up -d test-db
export TEST_DATABASE_DSN='root:password@tcp(127.0.0.1:3307)/threadly_test?parseTime=True'
DATABASE_URL='mysql://root:password@127.0.0.1:3307/threadly_test' make integration-test
```

Integration testが触るのは専用の `threadly_test` DBだけです。`make integration-test` がAtlasのversioned migrationを適用してからテストを実行します。テストデータの変更はtransactionをrollbackし、開発用の `go_post` DBやそのvolumeは削除しません。

## Migration

GORMのモデルは `internal/domain/models` に定義し、DBスキーマの変更履歴は `migrations/` と `migrations/atlas.sum` で管理します。Atlasのmigrationはアプリケーション起動時には実行されません。

Atlas CLIが必要です。macOSでは `make tools` が `brew install ariga/tap/atlas` を実行します。Homebrewを使わない環境では、Atlas公式のインストーラーを利用してください。

```sh
# 現在のGORMモデルとの差分からmigrationを生成する
make migrate-diff

# 適用済みバージョンと保留中のmigrationを確認する
make migrate-status
```

新しい空のDBへは `make migrate-apply` を実行して、コミット済みのmigrationを先頭から適用します。既存DB向けのbaselineや既存migrationとの互換対応はこの変更の対象外です。
