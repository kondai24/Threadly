# 負荷試験の計画と段階集計

現時点で利用できるのは、Dockerへ接続しない `plan` / `report`、専用DBの `prepare` / `reset`、専用Composeの構成定義とk6のHTTP操作です。
`run`、live監視・停止、ディスク監視は未実装です。
VM上限のpreflightも未実装のため、この構成を手動で起動して負荷試験を始めないでください。

## 開発用CLIの確認

`api/` から実行します。以下は計画・保存データの処理だけを確認する開発用コマンドです。測定用APIのbuildや負荷送信は行いません。

```sh
go run ./cmd/loadtest plan --scenario mixed --dataset medium
go run ./cmd/loadtest report --manifest /path/to/manifest.json --points /path/to/points.jsonl
```

`plan` は必須のscenario/datasetを検証してJSONを返します。
browseはsmall/medium/large、mixed/loginはmedium、smokeはsmallを受け付けます。
未知のflagや位置引数、外部URL・DSNは受け付けません。不正入力はstdoutを空にし、stderrへ固定の `loadtest invalid_input`、終了5を返します。

## reportの入力

schema_versionは1です。reportが読むmanifestの部分は次のとおりです。後続の実行器が保存する版・資源・fixture等の追加metadataは集計へ混ぜません。

| フィールド | 意味 |
| --- | --- |
| run_id / scenario / dataset | 生Pointと照合する試験条件 |
| finalized | 結果の保存が完了しているか |
| stages | 実行した段階を時刻順に記録する配列 |
| reasons | 固定分類の停止理由。generator_pressureは終端段階を容量候補から除外する |

stageには `phase`（warmup / measure / smoke）、`stage`（warmup / 1..6 / smoke）、UTCの `start` / `send_end` / `end`、`complete`、任意の `incomplete_windows` を記録します。
`complete` はその段階の結果が確定していることを表し、不完全段階は容量候補にしません。
測定の送信区間は120秒、warmupは60秒、drainは最大10秒です。smokeは送信から終了まで最大30秒で、早期正常終了を許容します。
予定段階を全て終えず停止した場合は理由を残します。理由もなく段階が欠けた結果は不完全です。

生データは[k6のJSON出力](https://grafana.com/docs/k6/latest/results-output/real-time/json/)のMetric / Point形式です。
`lt_http_duration` は1HTTPにつき1個のms値で、必須タグは `run_id` / `phase` / `scenario` / `stage` / `endpoint` / `outcome` です。
endpointは `GET /api/posts/:id` など固定のroute名とし、実際のUUIDやURLを含めません。outcomeはsuccess / failureです。内容不正もfailureへ数えます。

`lt_operations_started` / `lt_operations_completed` は業務操作のCounterで、完了にはoutcomeを付けます。
標準の `http_reqs` / `iterations` / `dropped_iterations` と照合し、欠落や未完了を検出します。
`lt_transport_errors` があればconnectivityとして扱います。Cookie・認証情報・response bodyは入力へ保存しません。

分位点はnearest rankです。30秒ごとの非重複窓は20HTTP以上で評価し、小さい窓はprovisionalにします。
容量候補は段階全体100HTTP以上、操作結果確定、dropなし、品質合格の段階です。warmup・smokeを容量へ含めません。
drain中の応答は開始段階へ含め、HTTP/秒の分母は120秒に固定します。操作/秒も別に返します。

Pointのtimeは採取時刻です。後の時刻のPointより遅れて記録された値は破棄せず、該当窓を不完全にします。
採取時刻だけでは受信遅延を全て復元できないため、監視器で検出した窓欠損はstageの `incomplete_windows`（0始まり）へ保存します。
live監視器と実k6の結合は未実装です。

| 終了コード | 意味 |
| --- | --- |
| 0 | 全予定段階合格で範囲内では限界未到達、またはsmoke正常 |
| 2 | 品質不合格 |
| 3 | 生成不足・drop・generator pressure |
| 4 | 環境・接続・監視障害 |
| 5 | 入力不正・結果不完全 |
| 130 | 利用者中断 |

停止理由を全て保持し、環境障害、結果不完全、生成不足、品質の順で主理由を決めます。中断は130です。

## 専用構成の検証

通常の `api/docker-compose.yml` は変更していません。専用projectはthreadly-loadtest、DBはthreadly_loadtestです。
DBとCollectorはホストへ公開せず、API/Grafana/Tempoは127.0.0.1の18080/13000/13200へ限定しています。
常駐+k6の合計上限は3.5CPU・3840MiB。各サービスのmemswap_limitはmem_limitと同値で、restartはnoです。
[Composeの制限指定](https://docs.docker.com/reference/compose-file/services/)は設定値であり、実効制限の確認は別途必要です。

builderは通常の `go build ./cmd/api` とCLIを1CPU・1536MiBの専用コンテナで作る構成です。
Go imageは1.26.10-bookworm、k6は2.3.0、その他は既存構成の明示tagを継承します。imageの取得・digest・architecture・起動互換性は未確認です。
builder / prepare用tool / k6は別profileです。測定前のbuild、Migration、fixture、ready確認と測定中の一時tool停止を制御するCLIは未実装です。

構成テストは検証専用の値を子processへ渡してComposeのJSONを読み、資源上限、volume分離、公開先、profile、Tempo専用datasourceを意味として検証します。
実際のローカル設定や秘密値を表示する必要はありません。

```sh
# api/ から。Docker CLIとComposeは必要ですが、daemonへの接続・コンテナ起動は行いません。
go test ./cmd/loadtest -run TestCompose -count=1

# Docker CLIを使わないplan/reportの確認
go test ./cmd/loadtest -run 'Test(Plan|Report)' -count=1

go test ./...
go test -race ./...
go vet ./...
make build
make vulncheck
```

CLIのテストは一時ディレクトリへbuildした実行ファイルのstdout・stderr・終了コードを観測します。
fixtureの期待値は固定の手計算例です。実MySQL・VM/swap・実効制限・Trace画面・全負荷試験の検証を含みません。

## k6の少量機能テスト

k6 2.3.0のローカル実行ファイルを指定します。テストが一時HTTPサーバーをloopbackへ起動し、実k6で少量の操作を送ります。コンテナ・DB・Threadly APIは使用しません。

```sh
# api/ から。ローカルに取得した実行ファイルへの絶対パスを指定します。
K6_BINARY=/path/to/k6 go test ./cmd/loadtest -run TestK6 -count=1
```

`K6_BINARY` 未指定ではk6テストをskipします。指定済みで実行できない場合は失敗します。
CIの `k6-functional-test` は公式2.3.0のLinux binaryを固定checksumで確認し、このテストを実行します。
検証対象はHTTP受信順・件数・JSON body・Cookie、生Point、秘密非出力です。5秒timeoutの検証も含みます。

`scripts/operations.js` はsmoke、閲覧、混在、loginのHTTP操作とPoint出力を担当します。
混在はglobal iterationの剰余20で16閲覧・4書き込み、Like/Unlikeを含む22HTTPを送ります。201の空bodyを許容し、200でも内容不正なら失敗です。
CookieはVUごとの割当を明示し、応答Cookieを次の操作へ流用しません。loginは毎iterationに1回だけ送り、認証応答のUserとCookieを検証します。

`scripts/scenario.js` は後続のrun制御が段階ごとに起動する入口です。専用 `http://api:8080` のみを許容し、`LOADTEST_CONFIG` / `LOADTEST_ACCOUNTS` で渡したJSONファイルを読みます。
configはschema_version=1、run_id、scenario、dataset、phase、stage、base_urlとfixtureを持ちます。
fixtureは同じindexで対応する `post_ids` / `parent_comment_ids` / `reply_ids` のUUID配列です。loginではfixtureを使いません。
accountsはVU順の配列で、重複しないid/usernameと、閲覧・混在・smoke用のcookieまたはlogin用のpasswordを保持します。browse/mixedは32件、loginは8件、smokeは1件です。

入口のoptionsはwarmup60秒、測定120秒、固定到着率・固定VU上限、drain最大10秒、smoke最大30秒です。
設定の `k6 inspect` 確認は送信時間・資源上限の実測を意味しません。
認証情報の取得・残存期限確認、0600一時ファイルの生成/削除、段階間の排他、900秒watchdog、停止理由の保存は未実装のrun制御が担当します。スクリプト単独で本試験を開始しないでください。

## 専用DBの準備と初期化

`prepare` / `reset` は `api/` から実行し、専用project `threadly-loadtest` だけを操作します。
秘密値は追跡しない `loadtest/secrets/loadtest.env`（0600推奨）から読み、開発用 `.env` や任意のDSNへフォールバックしません。

```sh
# loadtest/secrets/loadtest.env（値は各自で生成し、共有・コミットしない）
LOADTEST_DB_PASSWORD=...
LOADTEST_JWT_SECRET=...      # 32バイト以上
LOADTEST_USER_PASSWORD=...   # fixture Userの共通password

go run ./cmd/loadtest prepare --dataset small
go run ./cmd/loadtest reset --confirm-project threadly-loadtest
go run ./cmd/loadtest reset --reset-traces --confirm-project threadly-loadtest
```

`prepare` は次の順で準備し、成功時だけ `loadtest/state/fixture.json` へ未使用fixtureとして記録します。

1. 専用DB volumeが存在する場合、Composeのproject/volume labelが専用対象と一致するかを照合する。
2. `builder`（1CPU・1536MiB）で通常の `go build ./cmd/api` とCLIを作る。
3. `loadtest-db` を起動し、`migrate`（Atlas 1.3.5 community）で既存migrationを明示適用する。
4. `tool` containerの `seed` が、空のDBへ固定fixture v1を1つのtransactionで投入する。DBから読み戻したchecksumが生成値と一致した場合だけcommitする。
5. ホスト側でdataset・件数・checksumを再生成値と照合する。

準備済み・秘密設定なし・別projectのvolume・checksum不一致は終了5、Docker操作の失敗は終了4で、状態を書き換えません。全体の上限は10分です。
`reset` は確認値が `threadly-loadtest` と一致し、volumeの所有を照合できた場合だけ、`down --remove-orphans` の後に専用DB volumeを名前指定で削除します。
Trace volumeは `--reset-traces` の明示時だけ削除します。`down -v` は使いません。prepare / reset / runは `loadtest/state/lock` で排他し、実行中なら何もせず終了5です。

fixtureは件数表どおりのUser・Post・親Comment5件と返信5件・Post/CommentごとのLike2件です。
IDは種別と連番から作る固定UUID、時刻は2026-01-01T00:00:00Zからの連番秒です。password hashはランダムsaltのためchecksumの対象外です。

### 検証範囲

```sh
# 生成規則とchecksum（DB不要）
go test ./internal/loadtest -run 'Fixture' -count=1

# 実MySQLへの投入・rollback。TEST_DATABASE_DSNのserverへ使い捨てschemaを作り、終了時に削除します。
# 既存test-dbのschemaやデータには触れません。
TEST_DATABASE_DSN='root:password@tcp(127.0.0.1:3307)/threadly_test?parseTime=True' \
  go test -tags=integration ./internal/loadtest -run LoadFixture -count=1

# prepare / reset / seedのCLI（PATHへ置いた代替dockerで、Docker VMを使わずに検証）
go test ./cmd/loadtest -run 'Test(Prepare|Reset|Seed)' -count=1
```

代替dockerのテストは、呼び出し順・拒否時に破壊的操作を送らないこと・状態ファイル・秘密値の非出力を確認します。
実際の専用containerでのbuild、Atlas適用、`loadtest-db` へのseed、tool containerの256MiB上限内でのlarge投入は未検証です。

## 続きの実装条件

次はrun制御と、資源・Trace・ディスク監視です。開発DBや既存test-dbを初期化対象へ代用しません。
artifacts / results / secrets / .envはGit対象外です。通常のDB・Trace volumeを削除する手順はありません。
