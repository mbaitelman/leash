// Spins up a mock API server, opens each page with Playwright, saves PNGs.
// Run: node scripts/screenshots.js  (requires @playwright/test installed)
const http = require('http');
const fs   = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const HTML = fs.readFileSync(path.join(__dirname, '../internal/server/web/index.html'), 'utf8');
const LOGO = fs.readFileSync(path.join(__dirname, '../internal/server/web/logo.svg'));

const NOW = new Date().toISOString();
function ago(min) { return new Date(Date.now() - min * 60000).toISOString(); }

const MOCK = {
  '/api/config': { app_url: 'https://app.datadoghq.com', schedule: '0 6 * * *', dry_run: true },
  '/api/runs': [
    { run_id: 'a1b2c3d4-0000-0000-0000-000000000001', generated_at: ago(3),   dry_run: true,  policy_count: 5, total_matches: 7 },
    { run_id: 'a1b2c3d4-0000-0000-0000-000000000002', generated_at: ago(63),  dry_run: true,  policy_count: 5, total_matches: 3 },
    { run_id: 'a1b2c3d4-0000-0000-0000-000000000003', generated_at: ago(183), dry_run: false, policy_count: 5, total_matches: 0 },
    { run_id: 'a1b2c3d4-0000-0000-0000-000000000004', generated_at: ago(303), dry_run: true,  policy_count: 4, total_matches: 2 },
  ],
  '/api/runs/a1b2c3d4': {
    run_id: 'a1b2c3d4-0000-0000-0000-000000000001',
    generated_at: ago(3), dry_run: true,
    policies: [
      {
        policy_name: 'monitors-must-have-team-tag',
        resource: 'datadog.monitor',
        match_count: 4, pass_count: 18,
        matches: [
          { id: '12345678', properties: { name: 'High error rate – prod API', type: 'metric alert', status: 'Alert', tags: ['env:prod','service:api'], overall_state: 'Alert' } },
          { id: '23456789', properties: { name: 'P99 latency above threshold',  type: 'metric alert', status: 'OK',    tags: ['env:prod'], overall_state: 'OK' } },
          { id: '34567890', properties: { name: 'Disk usage > 90%',             type: 'query alert',  status: 'Warn',  tags: [], overall_state: 'Warn' } },
          { id: '45678901', properties: { name: 'SSL certificate expiry',       type: 'service check', status: 'OK',   tags: ['env:staging'], overall_state: 'OK' } },
        ],
        actions_taken: [
          { resource_id: '12345678', action_type: 'report', dry_run: true },
          { resource_id: '23456789', action_type: 'report', dry_run: true },
        ],
        passing: [
          { id: '56789012', properties: { name: 'CPU utilization', tags: ['env:prod','team:platform'] } },
          { id: '67890123', properties: { name: 'Memory pressure',  tags: ['env:prod','team:infra']    } },
        ],
      },
      {
        policy_name: 'slos-missing-description',
        resource: 'datadog.slo',
        match_count: 3, pass_count: 4,
        matches: [
          { id: 'slo-aabbcc', properties: { name: 'API availability 99.9%', sli_type: 'metric', tags: ['env:prod'] } },
          { id: 'slo-ddeeff', properties: { name: 'Checkout latency SLO',   sli_type: 'metric', tags: ['env:prod','team:checkout'] } },
          { id: 'slo-112233', properties: { name: 'Auth service uptime',     sli_type: 'monitor', tags: [] } },
        ],
        actions_taken: [],
        passing: [],
      },
      {
        policy_name: 'dashboards-must-have-description',
        resource: 'datadog.dashboard',
        match_count: 0, pass_count: 7,
        matches: [],
        actions_taken: [],
        passing: [
          { id: 'abc-123', properties: { title: 'Platform Overview', tags: ['team:platform'] } },
        ],
      },
      {
        policy_name: 'users-must-have-team-tag',
        resource: 'datadog.user',
        match_count: 0, pass_count: 12,
        matches: [],
        actions_taken: [],
        passing: [],
      },
      {
        policy_name: 'synthetics-must-have-team-tag',
        resource: 'datadog.synthetic',
        match_count: 0, pass_count: 6,
        matches: [],
        actions_taken: [],
        passing: [],
      },
    ],
  },
  '/api/policies': [
    { path: 'policies/examples/monitor-tagging.yaml',    size: 412 },
    { path: 'policies/examples/slo-tagging.yaml',        size: 389 },
    { path: 'policies/examples/dashboard-description.yaml', size: 356 },
    { path: 'policies/examples/user-tagging.yaml',       size: 298 },
    { path: 'policies/examples/synthetic-tagging.yaml',  size: 321 },
  ],
  '/api/log-level': { level: 'INFO' },
};

const server = http.createServer((req, res) => {
  const url = req.url.split('?')[0];

  if (url === '/') {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    return res.end(HTML);
  }
  if (url === '/logo.svg') {
    res.writeHead(200, { 'Content-Type': 'image/svg+xml' });
    return res.end(LOGO);
  }
  if (url === '/api/logs/stream') {
    res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' });
    const entries = [
      { time: ago(0.1), level: 'INFO',  msg: 'server started',       attrs: [{ k: 'port', v: '8080' }] },
      { time: ago(0.08), level: 'INFO', msg: 'run triggered',         attrs: [{ k: 'dry_run', v: 'true' }] },
      { time: ago(0.06), level: 'INFO', msg: 'listing monitors',      attrs: [{ k: 'count', v: '22' }] },
      { time: ago(0.04), level: 'WARN', msg: 'rate limit approached', attrs: [{ k: 'remaining', v: '10' }] },
      { time: ago(0.03), level: 'INFO', msg: 'policy evaluated',      attrs: [{ k: 'policy', v: 'monitor-tagging' }, { k: 'matches', v: '4' }] },
      { time: ago(0.01), level: 'INFO', msg: 'run complete',          attrs: [{ k: 'total_matches', v: '7' }] },
    ];
    entries.forEach(e => res.write('data: ' + JSON.stringify(e) + '\n\n'));
    return; // keep open
  }

  // prefix match for /api/runs/:id
  const runMatch = url.match(/^\/api\/runs\/([^/]+)/);
  if (runMatch) {
    const key = '/api/runs/' + runMatch[1].slice(0, 8);
    const data = MOCK[key];
    if (data) { res.writeHead(200, { 'Content-Type': 'application/json' }); return res.end(JSON.stringify(data)); }
  }

  if (MOCK[url]) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    return res.end(JSON.stringify(MOCK[url]));
  }

  res.writeHead(404); res.end('not found');
});

const OUT = path.join(__dirname, '../docs/screenshots');
fs.mkdirSync(OUT, { recursive: true });

(async () => {
  await new Promise(r => server.listen(3001, r));
  console.log('Mock server listening on :3001');

  const browser = await chromium.launch();
  const pages = [
    { name: 'overview',    hash: '#/',          wait: '#ov-stats .stat-card' },
    { name: 'runs-list',   hash: '#/runs',      wait: '#runs-body tr.clickable' },
    { name: 'run-detail',  hash: '#/runs/a1b2c3d4-0000-0000-0000-000000000001', wait: '.run-meta' },
    { name: 'policies',    hash: '#/policies',  wait: '#pol-body tr.clickable' },
    { name: 'logs',        hash: '#/logs',      wait: '.log-box .log-entry' },
  ];

  for (const { name, hash, wait } of pages) {
    for (const dark of [false, true]) {
      const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
      const page = await ctx.newPage();
      if (dark) await page.addInitScript(() => localStorage.setItem('leash-theme', 'dark'));
      await page.goto('http://localhost:3001/' + hash);
      await page.waitForSelector(wait, { timeout: 8000 }).catch(() => {});
      await page.waitForTimeout(800);
      const file = path.join(OUT, name + (dark ? '-dark' : '-light') + '.png');
      await page.screenshot({ path: file });
      console.log('saved', path.basename(file));
      await ctx.close();
    }
  }

  await browser.close();
  server.close();
  console.log('Done. Screenshots in docs/screenshots/');
})();
