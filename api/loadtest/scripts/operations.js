import http from 'k6/http';
import execution from 'k6/execution';
import { Counter, Trend } from 'k6/metrics';

const duration = new Trend('lt_http_duration');
const started = new Counter('lt_operations_started');
const completed = new Counter('lt_operations_completed');
const transportErrors = new Counter('lt_transport_errors');

export function executeOperation(config, accounts) {
  const iteration = execution.scenario.iterationInTest;
  const account = accounts[execution.vu.idInTest - 1];
  const tags = { run_id: config.run_id, scenario: config.scenario, phase: config.phase, stage: config.stage };
  started.add(1, tags);
  let success;
  if (config.scenario === 'smoke') {
    const health = request(config, null, tags, 'GET', '/health', 'GET /health', null, 200);
    const unauthorized = request(config, null, tags, 'GET', '/api/posts', 'GET /api/posts', null, 401);
    success = health && unauthorized;
    for (const operation of [0, 1, 2, 16, 17, 18, 19]) {
      const result = performOperation(config, account, tags, operation);
      success = result && success;
    }
  } else {
    success = performOperation(config, account, tags, iteration);
  }
  completed.add(1, { ...tags, outcome: success ? 'success' : 'failure' });
}

function performOperation(config, account, tags, iteration) {
  if (config.scenario === 'login') return login(config, account, tags);
  const fixture = config.fixture;
  const post = fixture.post_ids[iteration % fixture.post_ids.length];
  let success;
  const operation = iteration % 20;
  if (config.scenario === 'browse' || operation < 16) {
    const read = iteration % 3;
    const paths = ['/api/posts', `/api/posts/${post}`, `/api/posts/${post}/comments`];
    const endpoints = ['GET /api/posts', 'GET /api/posts/:id', 'GET /api/posts/:id/comments'];
    const index = iteration % fixture.post_ids.length;
    const validators = [
      (value) => Array.isArray(value) && value.some((item) => item !== null && item.id === post),
      (value) => validSummary(value) && value.id === post && typeof value.title === 'string' && typeof value.content === 'string',
      (value) => validComments(value, fixture.parent_comment_ids[index], fixture.reply_ids[index]),
    ];
    success = request(config, account, tags, 'GET', paths[read], endpoints[read], null, 200, validators[read]);
  } else if (operation === 16) {
    success = request(config, account, tags, 'POST', '/api/posts', 'POST /api/posts', { title: 'loadtest post', content: 'loadtest content' }, 201);
  } else if (operation === 17) {
    success = request(config, account, tags, 'POST', `/api/posts/${post}/comments`, 'POST /api/posts/:id/comments', { content: 'loadtest comment' }, 201);
  } else {
    const index = (execution.vu.idInTest - 1) % fixture.post_ids.length;
    const isPost = operation === 18;
    const target = isPost ? fixture.post_ids[index] : fixture.parent_comment_ids[index];
    const prefix = isPost ? '/api/posts' : '/api/comments';
    const endpoint = isPost ? '/api/posts/:id/like' : '/api/comments/:id/like';
    const path = `${prefix}/${target}/like`;
    success = request(config, account, tags, 'PUT', path, `PUT ${endpoint}`, null, 200,
      (value) => validLike(value, target, true))
      && request(config, account, tags, 'DELETE', path, `DELETE ${endpoint}`, null, 200,
        (value) => validLike(value, target, false));
  }
  return success;
}

function login(config, account, tags) {
  return request(config, null, tags, 'POST', '/api/auth/login', 'POST /api/auth/login',
    { username: account.username, password: account.password }, 200,
    (value, response) => value !== null && value.user !== null && typeof value.user === 'object'
      && value.user.id === account.id && value.user.username === account.username
      && Array.isArray(response.cookies['threadly-session'])
      && response.cookies['threadly-session'].some((cookie) => typeof cookie.value === 'string' && cookie.value.length > 0));
}

function validLike(value, target, liked) {
  return value !== null && value.targetId === target && Number.isInteger(value.likeCount)
    && value.likeCount >= 0 && value.likedByMe === liked;
}

function validSummary(value) {
  return value !== null && typeof value === 'object' && typeof value.id === 'string'
    && value.author !== null && typeof value.author === 'object' && typeof value.author.id === 'string'
    && Number.isInteger(value.likeCount) && value.likeCount >= 0 && typeof value.likedByMe === 'boolean';
}

function validComments(value, parentID, replyID) {
  if (!Array.isArray(value)) return false;
  const parent = value.find((item) => item !== null && item.id === parentID);
  if (!validSummary(parent) || typeof parent.content !== 'string' || !Array.isArray(parent.replies)) return false;
  return parent.replies.some((reply) => validSummary(reply) && reply.id === replyID
    && typeof reply.content === 'string' && Array.isArray(reply.replies) && reply.replies.length === 0);
}

function request(config, account, tags, method, path, endpoint, body, expectedStatus, validate) {
  // 応答のSet-Cookieを次の操作の認証へ流用せず、割当済みCookieだけを送る。
  http.cookieJar().clear(config.base_url);
  const response = http.request(method, config.base_url + path, body === null ? null : JSON.stringify(body), {
    cookies: account === null ? {} : { 'threadly-session': account.cookie },
    headers: { 'Content-Type': 'application/json' },
    redirects: 0,
    timeout: '5s',
    responseCallback: http.expectedStatuses(expectedStatus),
    tags: { ...tags, name: endpoint },
  });
  let success = response.status === expectedStatus;
  // k6のerror_codeにはHTTP 4xx/5xxも含まれるため、接続障害とは分ける。
  if (response.status === 0) {
    transportErrors.add(1, { ...tags, endpoint, kind: response.error_code === 1050 ? 'timeout' : 'transport' });
  }
  if (success && validate) {
    try { success = validate(response.json(), response); } catch (_) { success = false; }
  }
  duration.add(response.timings.duration, { ...tags, endpoint, outcome: success ? 'success' : 'failure' });
  return success;
}
