# 負荷試験

CPU・メモリに上限を付けた専用環境の API へ、k6 で段階的に負荷をかけます。

| ファイル | 役割 |
| --- | --- |
| `threadly.ts` | k6 のシナリオ。テスト用ユーザーと投稿の準備、負荷の段階、合否の基準を定義します |
| `compose.yaml` | 専用環境（project `threadly-loadtest`）。DB・migration・API を CPU 1 / メモリ 1GiB ずつで起動します |
| `Dockerfile` | 専用環境の API image |

## 実行

Docker と k6（`brew install k6`）が必要です。`api/` から実行します。

```sh
# 1. 専用環境を起動する（API は http://localhost:18080）
docker compose -f loadtest/compose.yaml up -d --build --wait api

# 2. 負荷をかける（約13分）
k6 run loadtest/threadly.ts

# 動作確認だけなら、各段階を短くする（約1分半）
k6 run -e STAGE_DURATION=10s loadtest/threadly.ts

# 3. 片付ける（DB も消すため、次回は空の DB から始まる）
docker compose -f loadtest/compose.yaml down -v
```

k6 をインストールしない場合は、公式 image で実行できます。Linux では `--add-host=host.docker.internal:host-gateway` も指定してください。

```sh
docker run --rm -v "$PWD/loadtest:/scripts:ro" -e BASE_URL=http://host.docker.internal:18080 \
  grafana/k6:2.3.0 run /scripts/threadly.ts
```

`BASE_URL` は専用環境以外へ向けないでください。テスト用のユーザー・投稿・コメントを書き込みます。

## 負荷の内容

`setup()` が最初に1回だけ、テスト用ユーザー32人の登録と投稿10件の作成を行います。

その後、毎秒 1 → 2 → 5 → 10 → 20 → 40 回の操作を、各段階 `STAGE_DURATION`（既定2分）ずつ送ります。32人の VU がそれぞれ別のユーザーとして操作します。

| 割合 | 操作 |
| --- | --- |
| 30% | `GET /api/posts` |
| 30% | `GET /api/posts/:id` |
| 20% | `GET /api/posts/:id/comments` |
| 10% | `POST /api/posts/:id/comments` |
| 10% | `PUT /api/posts/:id/like` のあと `DELETE /api/posts/:id/like` |

## 結果の見方

終了時のサマリーに、次の基準を満たしたかが表示されます。満たさない基準があると、k6 は終了コード 99 で終わります。

| 基準 | 意味 |
| --- | --- |
| `http_req_failed{scenario:mixed}` rate<1% | 失敗率。超えた時点で試験を打ち切ります |
| `http_req_duration{scenario:mixed}` p(95)<500ms | 95%の応答が500ms未満 |
| `checks` rate>99% | ステータスコードの確認が通った割合 |
| `dropped_iterations` count<1 | VU が足りず送れなかった操作。API が到着率に追いついていないことを示します |

途中で打ち切られた場合は、経過時間からどの段階で失敗が増えたかが分かります。試験中の CPU・メモリは `docker stats` で確認できます。
