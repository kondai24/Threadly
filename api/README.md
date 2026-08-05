# Threadly API

## 起動

MySQL を起動したうえで、API を実行します。

```sh
cd api
docker compose up -d db

export DB_USER=root
export DB_PASSWORD=password
export DB_HOST=127.0.0.1
export DB_PORT=3306
export DB=go_post
export JWT_SECRET="32バイト以上のランダムな秘密値"

go run ./cmd/api
```

`JWT_SECRET` は必須です。HS256 の署名鍵として利用するため、32バイト以上の秘密値を設定してください。設定項目は `.env.sample` にも記載しています。

## 認証

パスワードは Argon2id でハッシュ化して保存します。登録またはログインが成功すると、1時間有効な Bearer JWT が返ります。

```sh
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"correct-horse-battery"}'

curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"correct-horse-battery"}'
```

以降の `/api/me` と `/api/posts` へのアクセスでは、レスポンスの `token` を使用します。

```sh
export TOKEN='ログインで返されたJWT'

curl http://localhost:8080/api/me \
  -H "Authorization: Bearer ${TOKEN}"

curl -X POST http://localhost:8080/api/posts \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"title":"hello","content":"world"}'
```

Post の一覧取得・詳細取得は、JWTで認証されたUserが全Postを閲覧できます。作成時の投稿者はJWTのUser IDから設定され、更新・削除は投稿者本人に限定されます。レスポンスには公開User情報として `author.id` と `author.username` を含めます。`authorId` はリクエストから受け取りません。

詳細なリクエスト・レスポンスは `/swagger/index.html` を参照してください。

## テスト

通常のUnit・route testは、APIディレクトリで実行します。

```sh
go test ./...
go test -race ./...
go vet ./...
```

`UserRepository` のIntegration testは、既存の開発用DBとは分離したテスト用MySQLを用意し、DSNを指定して実行します。

```sh
export TEST_DATABASE_DSN='root:password@tcp(127.0.0.1:3306)/threadly_test?parseTime=True'
go test -tags=integration ./internal/infra/database/repositories
```
