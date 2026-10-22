import { executeOperation } from './operations.js';

const config = readInput(__ENV.LOADTEST_CONFIG);
const accounts = readInput(__ENV.LOADTEST_ACCOUNTS);
validateInput(config, accounts);
const smoke = config.scenario === 'smoke';
const maxVUs = config.scenario === 'login' ? 8 : 32;
const rates = config.scenario === 'login' ? [1, 2, 3, 4, 5, 6] : [1, 2, 5, 10, 20, 40];
const stage = smoke ? {
  executor: 'shared-iterations', vus: 1, iterations: 1, maxDuration: '30s', gracefulStop: '0s',
} : {
  executor: 'constant-arrival-rate',
  rate: config.phase === 'warmup' ? 1 : rates[Number(config.stage) - 1],
  timeUnit: '1s',
  duration: config.phase === 'warmup' ? '60s' : '120s',
  preAllocatedVUs: maxVUs,
  maxVUs,
  gracefulStop: '10s',
};

export const options = {
  scenarios: { stage },
  // 生Pointへ実URL・UUIDを出さず、business scenarioのタグをexecutor名で上書きしない。
  systemTags: ['status', 'method', 'name'],
  tags: { run_id: config.run_id, scenario: config.scenario, phase: config.phase, stage: config.stage },
};

export default function() {
  executeOperation(config, accounts);
}

function readInput(path) {
  try { return JSON.parse(open(path)); } catch (_) { throw new Error('loadtest invalid_input'); }
}

function validateInput(config, accounts) {
  const invalid = () => { throw new Error('loadtest invalid_input'); };
  if (!config || config.schema_version !== 1 || config.base_url !== 'http://api:8080'
    || typeof config.run_id !== 'string' || !/^[a-zA-Z0-9-]{1,64}$/.test(config.run_id)
    || !['browse', 'mixed', 'login', 'smoke'].includes(config.scenario)) invalid();
  const smoke = config.scenario === 'smoke';
  const datasets = config.scenario === 'browse' ? ['small', 'medium', 'large'] : [smoke ? 'small' : 'medium'];
  if (!datasets.includes(config.dataset)) invalid();
  if (smoke) {
    if (config.phase !== 'smoke' || config.stage !== 'smoke') invalid();
  } else if (!((config.phase === 'warmup' && config.stage === 'warmup')
    || (config.phase === 'measure' && typeof config.stage === 'string' && /^[1-6]$/.test(config.stage)))) invalid();
  const vus = smoke ? 1 : config.scenario === 'login' ? 8 : 32;
  if (!Array.isArray(accounts) || accounts.length !== vus) invalid();
  const ids = new Set();
  const usernames = new Set();
  for (const account of accounts) {
    if (!account || !uuid(account.id) || typeof account.username !== 'string' || !account.username
      || ids.has(account.id) || usernames.has(account.username)) invalid();
    ids.add(account.id);
    usernames.add(account.username);
    if (config.scenario === 'login') {
      if (typeof account.password !== 'string' || !account.password) invalid();
    } else if (typeof account.cookie !== 'string' || !account.cookie || /[\r\n;]/.test(account.cookie)) invalid();
  }
  if (config.scenario !== 'login') {
    const fixture = config.fixture;
    if (!fixture || !Array.isArray(fixture.post_ids) || fixture.post_ids.length === 0) invalid();
    for (const key of ['post_ids', 'parent_comment_ids', 'reply_ids']) {
      if (!Array.isArray(fixture[key]) || fixture[key].length !== fixture.post_ids.length
        || !fixture[key].every(uuid)) invalid();
    }
  }
}

function uuid(value) {
  return typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
}
