# 負荷試験の計画と段階集計

現時点で利用できるのは、Dockerへ接続しない `plan` / `report`、専用DBの `prepare` / `reset`、段階制御・監視付きの `run`、専用Composeの構成定義とk6のHTTP操作です。
資源時系列・generator pressure・ディスク監視とmanifestの環境metadataは未実装です。
この構成を `docker compose up` で手動起動して負荷試験を始めず、`run` を使ってください。

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
認証情報の取得・残存期限確認、0600一時ファイルの生成/削除、段階間の排他、900秒watchdog、停止理由の保存は `run` が担当します。スクリプト単独で本試験を開始しないでください。

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

1. 解決済みComposeのproject名・DB名・volume名と、既存の `loadtest-db` container・DB volumeのComposeラベルが専用対象と一致するかを照合する。
2. `builder`（1CPU・1536MiB）で通常の `go build ./cmd/api` とCLIを作る。
3. `loadtest-db` を起動し、`migrate`（Atlas 1.3.5 community）で既存migrationを明示適用する。
4. `tool` containerの `seed` が、空のDBへ固定fixture v1を1つのtransactionで投入する。DBから読み戻したchecksumが生成値と一致した場合だけcommitする。
5. ホスト側でdataset・件数・checksumを再生成値と照合する。

準備済み・秘密設定なし・対象不一致・seedの拒否（初期化済みDBや読み戻し不一致）・checksum不一致は終了5、Docker操作の失敗は終了4で、状態を書き換えません。
build / migrate / seedは名前付きの一時containerで実行し、失敗やtimeout（全体10分）では排他を解放する前に名前指定で削除します。
`reset` は確認値が `threadly-loadtest` と一致し、volumeの所有を照合できた場合だけ、`down --remove-orphans` の後に専用DB volumeを名前指定で削除します。
Trace volumeは `--reset-traces` の明示時だけ削除します。`down -v` は使いません。prepare / reset / runは `loadtest/state/lock` で排他し、実行中なら何もせず終了5です。

fixtureは件数表どおりのUser・Post・親Comment5件と返信5件・Post/CommentごとのLike2件です。
IDは種別と連番から作る固定UUID、時刻は2026-01-01T00:00:00Zからの連番秒です。password hashはランダムsaltのためchecksumの対象外です。

### 検証範囲

```sh
# 生成規則とchecksum（DB不要）
go test ./internal/loadtest -run 'Fixture' -count=1

# 実MySQLへの投入・rollback・unique/FK制約。TEST_DATABASE_DSNのserverへ使い捨てschemaを作り、
# atlas CLIで既存migrationを適用してから検証し、終了時に削除します。既存test-dbのschemaやデータには触れません。
TEST_DATABASE_DSN='root:password@tcp(127.0.0.1:3307)/threadly_test?parseTime=True' \
  go test -tags=integration ./internal/loadtest -run LoadFixture -count=1

# prepare / reset / seedのCLI（PATHへ置いた代替dockerで、Docker VMを使わずに検証）
go test ./cmd/loadtest -run 'Test(Prepare|Reset|Seed)' -count=1
```

代替dockerのテストは、呼び出し順・拒否時に破壊的操作を送らないこと・状態ファイル・秘密値の非出力を確認します。
実際の専用containerでのbuild、Atlas適用、`loadtest-db` へのseed、tool containerの256MiB上限内でのlarge投入は未検証です。

## run

```sh
go run ./cmd/loadtest run --scenario smoke --dataset small
go run ./cmd/loadtest reset --confirm-project threadly-loadtest
go run ./cmd/loadtest prepare --dataset medium
go run ./cmd/loadtest run --scenario mixed --dataset medium
```

`run` は未使用のfixtureだけを受け付け、送信開始前に使用済みへ変えます。次のrunの前には `reset` と `prepare` が必要です。
smokeが失敗したら他シナリオを開始しないでください。smokeと本試験の間もresetします。

準備（全体10分以内）:

1. `docker info` でVMのCPUが4・メモリが6GiB以下で、上限合計＋CPU 0.5・メモリ1GiBの余裕があること、swap制限に対応していることを確認する。
2. `docker ps` で専用project以外のcontainerが動いていれば、名前を示して拒否する（自動停止しない）。
3. 解決済みComposeと既存DB containerの専用対象を照合し、`api` / `otel-collector` / `tempo` / `grafana` を起動する。
4. API `127.0.0.1:18080/health`・Tempo `127.0.0.1:13200/ready`・Grafana `127.0.0.1:13000/api/health` の応答を待つ。
   Collectorは公開portを持たないため、常駐サービスの稼働監視で代替する。
5. `tool verify` で専用DBの件数とchecksumを読み戻し、`prepare` 時の記録と一致しなければ送信せず `result_incomplete` とする。
6. browse / mixed / smokeでは割り当てUserを直列に事前ログインし、先頭Userの `/api/me` で認証付きAPIを確認する。
   Cookieは `loadtest/runtime/<run_id>/accounts.json`（0600、k6へread-only mount）だけに置き、終了時に削除する。loginシナリオは事前ログインせずpasswordだけを渡す。

準備中の失敗・中断でも `manifest.json` と `report.json` を保存し、理由（`environment` / `result_incomplete` / `interrupted`）を残します。

測定:

- 段階ごとに `threadly-loadtest-k6-<stage>` の一時containerでk6を1つだけ起動し、生Pointを `results/<run_id>/points-<stage>.jsonl` へ出す。
- ホスト側は1秒ごとに生Pointを読み、30秒窓（20HTTP以上）の品質違反・transport error・10秒以上のHTTP停滞・起動後30秒のPoint欠如を判定する。
  段階の開始時刻は最初に採取されたPointの時刻を基準とし、窓は境界から5秒待って確定する。確定後に届いたPointの窓は不完全として記録する。
  5秒ごとに常駐サービスのOOM・停止・再起動とAPI health（timeout 2秒）を確認する。Docker eventsは購読せず、この `docker inspect` の定期確認で代替する。
- 停止理由を検出したらk6へSIGINTを送り、最大10秒のdrain後は強制停止する。drain内に終えた段階は `stopped` として記録し、停止理由で判定する。
- 段階終了ごとにreportと同じ規則で評価し、品質・生成不足・不完全があれば次段階を開始しない。
- SIGINT後に停止できなければ `docker kill`・`rm -f` で強制停止し、確認できない場合は `monitoring_failed` を記録する。
- 最初の送信から890秒で停止要求し（895秒で強制停止の保険）、残り時間で送信120秒＋drain＋余裕を確保できない段階は短縮せずに開始しない。
- Ctrl-C / SIGTERMでも同じ停止と保存を行い、終了130を返す。

結果は新規の `results/<run_id>/` に `manifest.json` / `points.jsonl` / `report.json` / 段階別の生Pointとk6設定を保存し、reportを標準出力へ出します。
終了コードはreportと同じです。保存済み結果は `report` で同じ規則により再生成できます。

### runの検証範囲

```sh
# 代替dockerとloopbackのAPI fixture serverで、Docker VMを使わずに実行する（約1分）。
go test ./cmd/loadtest -run TestRun -count=1
# 30秒窓の品質停止を実時間で待つケースを除く場合
go test ./cmd/loadtest -run TestRun -short -count=1
```

代替dockerは `docker info` / `ps` / `inspect` / `tool verify` の状態fixture、k6の生Point出力（遅れて書かれるPointを含む）、SIGINTの受信を再現します。
Tempo・Grafanaの準備完了応答はAPI fixture serverで代替します（`LOADTEST_TEMPO_URL` / `LOADTEST_GRAFANA_URL`、loopbackのみ）。
`verify` の実DB照合は `ReadFixtureSummary` のIntegration testで確認します。
多段の正常完走は実時間で約15分かかるため自動テストでは扱わず、smokeの正常系と各停止理由で検証します。
実Docker VMでの起動・事前認証・k6 container（uid）からの0600ファイル読み込み・実OOM検出は未検証です。

## 資源・保存量の監視と比較条件

測定中は5秒ごとに次を確認します（段階開始直後にも1回採取します）。サービス状態の確認と資源採取は並行に行います。

- `docker stats --no-stream` で常駐サービスとk6（最初のPoint観測後）のCPU・メモリを採取し、`results/<run_id>/resources.jsonl` へ追記する。
  CPUは各containerの上限に対する比率（`cpu_quota_ratio`）、メモリは上限に対する比率（`memory_ratio`）も残す。
  対象containerの値が欠けた場合は `monitoring_failed` とする。ただしk6が正常終了する際の `compose run --rm` による削除中（最大2秒待って終了を確認）の失敗は除く。
- k6のメモリが上限の90%以上、またはCPUが上限の90%以上を30秒続けた場合は `generator_pressure` で停止する（生成側の限界をAPIの限界と混同しない）。
- `results/<run_id>/` が512MiB以上、または専用Trace volumeが1GiB以上になれば `disk_limit` で停止する。
  Trace volumeは `docker system df -v`（VM上の全volume・imageを走査するため30秒ごと）で確認し、採取できない・volumeが見つからなければ `monitoring_failed`。

Tempoは専用の `loadtest/tempo.yaml` でblock保持期間を24時間にします。Trace volumeは `reset --reset-traces` 以外では削除しません。

`manifest.json` には比較条件として、plan・fixture要約・Trace設定（sampler、exporter）・開始/測定開始/終了時刻・Docker/Composeの版・Gitのcommitと未コミット有無・
各imageとdigest・起動後の常駐containerへ実際に適用された上限・監視プロセスと子process（docker / git）のCPU時間と最大RSS・測定開始時と終了時のTrace volume量を記録します。
k6へ適用された上限は段階ごとに `k6-limits-<stage>.json` へ保存します。secret・Cookie・passwordは記録しません。
送信前にTrace volume量を計測できなければ送信せず `monitoring_failed` とします。

段階別の集計は `report.json` に加えて `report.csv` にも同じ値で保存します。列は段階・要求rate・予定/開始/完了/失敗操作数・drop数・HTTP数・HTTP/秒・操作/秒・p95・p99・エラー率・
窓の判定別件数（passed / quality_failed / provisional / incomplete）・合否・drain秒です。CSV・reportの保存に失敗した場合は結果不完全（終了5）です。

### 監視の検証範囲

`docker stats` / `system df` / `version` / `image inspect` の出力は代替dockerのfixtureで再現します。CPU 90%が30秒続くケースは実時間待ちのため `-short` では除外します。
実Docker VMでのcgroup上限値の取得、`docker system df` の実測値、Tempoの24時間保持の実挙動は未検証です。

artifacts / results / runtime / secrets / state / .envはGit対象外です。開発DBや既存test-dbを初期化対象へ代用しません。通常のDB・Trace volumeを削除する手順はありません。
