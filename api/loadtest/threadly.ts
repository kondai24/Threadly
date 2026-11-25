// Threadly API の負荷試験。手順は loadtest/README.md を参照する。
import http from 'k6/http';
import { check, fail } from 'k6';
import exec from 'k6/execution';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:18080';
// 各段階を一定に保つ時間。動作確認だけなら `-e STAGE_DURATION=10s` のように短くする。
const STAGE_DURATION = __ENV.STAGE_DURATION || '2m';
const RATES = [1, 2, 5, 10, 20, 40];
const VUS = 32;
const SEED_POSTS = 10;
const PASSWORD = 'loadtest-password';
const SESSION_COOKIE = 'threadly-session';

type SetupData = { cookies: string[]; postIds: string[] };

export const options = {
  setupTimeout: '3m',
  scenarios: {
    mixed: {
      executor: 'ramping-arrival-rate',
      startRate: RATES[0],
      timeUnit: '1s',
      preAllocatedVUs: VUS,
      maxVUs: VUS,
      // 各段階へ5秒で切り替えたあと、STAGE_DURATION の間は同じ到着率を保つ。
      stages: RATES.flatMap((rate) => [
        { target: rate, duration: '5s' },
        { target: rate, duration: STAGE_DURATION },
      ]),
    },
  },
  thresholds: {
    // 失敗が増えた段階より先は測っても意味がないため、その時点で打ち切る。
    'http_req_failed{scenario:mixed}': [{ threshold: 'rate<0.01', abortOnFail: true, delayAbortEval: '10s' }],
    'http_req_duration{scenario:mixed}': ['p(95)<500'],
    checks: ['rate>0.99'],
    // VU が足りず予定どおりに送れなかった操作。API が到着率に追いつけていないことを示す。
    dropped_iterations: ['count<1'],
  },
};

function params(cookie: string | null, name: string) {
  return {
    headers: { 'Content-Type': 'application/json' },
    // Cookie jar に残った別ユーザーの Cookie ではなく、割り当てたユーザーの Cookie を送る。
    cookies: cookie === null ? {} : { [SESSION_COOKIE]: { value: cookie, replace: true } },
    tags: { name },
  };
}

// テスト開始前に1回だけ実行し、戻り値を全 VU へ渡す。
export function setup(): SetupData {
  // 空でない DB で再実行しても username が重複しないよう、実行ごとに接頭辞を変える。
  const prefix = `lt${Date.now()}`;
  const cookies: string[] = [];
  for (let i = 0; i < VUS; i++) {
    const res = http.post(`${BASE_URL}/api/auth/register`,
      JSON.stringify({ username: `${prefix}_${i}`, password: PASSWORD }),
      params(null, 'POST /api/auth/register'));
    const cookie = res.cookies[SESSION_COOKIE]?.[0]?.value;
    if (res.status !== 201 || !cookie) fail(`register failed: status ${res.status}`);
    cookies.push(cookie);
  }

  for (let i = 0; i < SEED_POSTS; i++) {
    const res = http.post(`${BASE_URL}/api/posts`,
      JSON.stringify({ title: `seed post ${i}`, content: 'seed content' }),
      params(cookies[0], 'POST /api/posts'));
    if (res.status !== 201) fail(`create post failed: status ${res.status}`);
  }
  // 投稿作成は 201 で body を返さないため、一覧から ID を集める。
  const list = http.get(`${BASE_URL}/api/posts`, params(cookies[0], 'GET /api/posts'));
  if (list.status !== 200) fail(`list posts failed: status ${list.status}`);
  const postIds = (list.json() as Array<{ id: string }>).map((post) => post.id);
  if (postIds.length === 0) fail('no posts');
  return { cookies, postIds };
}

// 1 iteration = 1操作。閲覧8割、書き込み2割で送る。
export default function (data: SetupData) {
  // VU 番号は 1〜VUS なので、VU ごとに別ユーザーとして振る舞う。
  const cookie = data.cookies[exec.vu.idInTest - 1];
  const postId = data.postIds[exec.scenario.iterationInTest % data.postIds.length];
  const roll = Math.random();

  if (roll < 0.3) {
    const res = http.get(`${BASE_URL}/api/posts`, params(cookie, 'GET /api/posts'));
    check(res, { 'list posts 200': (r) => r.status === 200 });
  } else if (roll < 0.6) {
    const res = http.get(`${BASE_URL}/api/posts/${postId}`, params(cookie, 'GET /api/posts/:id'));
    check(res, { 'get post 200': (r) => r.status === 200 });
  } else if (roll < 0.8) {
    const res = http.get(`${BASE_URL}/api/posts/${postId}/comments`, params(cookie, 'GET /api/posts/:id/comments'));
    check(res, { 'list comments 200': (r) => r.status === 200 });
  } else if (roll < 0.9) {
    const res = http.post(`${BASE_URL}/api/posts/${postId}/comments`,
      JSON.stringify({ content: 'loadtest comment' }), params(cookie, 'POST /api/posts/:id/comments'));
    check(res, { 'create comment 201': (r) => r.status === 201 });
  } else {
    const like = http.put(`${BASE_URL}/api/posts/${postId}/like`, null, params(cookie, 'PUT /api/posts/:id/like'));
    if (!check(like, { 'like 200': (r) => r.status === 200 })) return;
    // Like したまま残すと次回以降の Like が同じ状態にならないため、直後に取り消す。
    const unlike = http.del(`${BASE_URL}/api/posts/${postId}/like`, null, params(cookie, 'DELETE /api/posts/:id/like'));
    check(unlike, { 'unlike 200': (r) => r.status === 200 });
  }
}
