import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { type ParsedRoute } from '../go-parser.ts';
import {
  runSync,
  type SyncResult,
  type SyncError,
  type ChainInfo,
} from '../index.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROUTER_FIXTURES = join(__dirname, 'fixtures/routers');
const DOCS_FIXTURES = join(__dirname, 'fixtures/docs');

function makeRoute(overrides: Partial<ParsedRoute> & { handler: string }): ParsedRoute {
  return {
    method: 'POST',
    path: '/test',
    handler: overrides.handler,
    file: 'test_router.go',
    middlewares: [],
    ...overrides,
  };
}

await describe('bruno-sync', async () => {
  await describe('runSync', async () => {
    await it('generates new Bruno doc for new route', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');
      const authDir = join(docsDir, 'auth');
      const paymentsDir = join(docsDir, 'payments');
      rmSync(authDir, { recursive: true, force: true });
      rmSync(paymentsDir, { recursive: true, force: true });

      const oldRoutes: ParsedRoute[] = [];
      const newRoutes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/magiclink',
          handler: 'CreateMagicLink',
          file: 'auth_router.go',
        }),
      ];

      const results = runSync(oldRoutes, newRoutes, docsDir, '/api/v1');

      assert.equal(results.length, 1);
      assert.equal(results[0].action, 'created');
      assert.equal(results[0].success, true);
      // Check file was created
      const docPath = join(authDir, 'magiclink.yml');
      assert.ok(existsSync(docPath));
      const content = readFileSync(docPath, 'utf-8');
      assert.ok(content.includes('name: "Create Magic Link"'));
      assert.ok(content.includes('/api/v1/magiclink'));

      rmSync(tmpDir, { recursive: true, force: true });
    });

    await it('deletes Bruno doc for deleted route', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');
      const authDir = join(docsDir, 'auth');
      rmSync(authDir, { recursive: true, force: true });

      const oldRoutes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/magiclink',
          handler: 'CreateMagicLink',
          file: 'auth_router.go',
        }),
      ];

      // First create the doc
      const createResults = runSync([], oldRoutes, docsDir, '/api/v1');
      assert.equal(createResults.length, 1);
      assert.ok(existsSync(join(authDir, 'magiclink.yml')));

      // Then delete
      const deleteResults = runSync(oldRoutes, [], docsDir, '/api/v1');
      const deletes = deleteResults.filter(r => r.action === 'deleted');
      assert.equal(deletes.length, 1);
      assert.equal(deletes[0].success, true);

      rmSync(tmpDir, { recursive: true, force: true });
    });

    await it('marks stale routes with handler change', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');
      const authDir = join(docsDir, 'auth');

      const oldRoutes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/magiclink',
          handler: 'OldHandler',
          file: 'auth_router.go',
        }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/magiclink',
          handler: 'NewHandler',
          file: 'auth_router.go',
        }),
      ];

      const results = runSync(oldRoutes, newRoutes, docsDir, '/api/v1');
      const stale = results.filter(r => r.action === 'stale');
      assert.equal(stale.length, 1);
      assert.equal(stale[0].handler, 'NewHandler');

      rmSync(tmpDir, { recursive: true, force: true });
    });

    await it('skips unchanged routes', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');

      const routes = [
        makeRoute({ method: 'POST', path: '/a', handler: 'A', file: 'r.go' }),
        makeRoute({ method: 'GET', path: '/b', handler: 'B', file: 'r.go' }),
      ];

      const results = runSync(routes, routes, docsDir, '/api/v1');
      const actions = results.map(r => r.action);
      assert.equal(actions.includes('created'), false);
      assert.equal(actions.includes('deleted'), false);
      assert.equal(actions.includes('stale'), false);

      rmSync(tmpDir, { recursive: true, force: true });
    });

    await it('provides sync summary with file paths and actions', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');

      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/old', handler: 'Old', file: 'r.go' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/new', handler: 'New', file: 'r.go' }),
      ];

      const results = runSync(oldRoutes, newRoutes, docsDir, '/api/v1');
      assert.equal(results.length, 2); // 1 delete + 1 create

      const created = results.find(r => r.action === 'created');
      const deleted = results.find(r => r.action === 'deleted');
      assert.ok(created);
      assert.ok(deleted);
      assert.ok(created.docPath);
      assert.ok(deleted.docPath);

      rmSync(tmpDir, { recursive: true, force: true });
    });

    await it('uses chainLookup to write confirmed chain script in new doc', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');
      const authDir = join(docsDir, 'auth');

      const oldRoutes: ParsedRoute[] = [];
      const newRoutes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/magiclink',
          handler: 'CreateMagicLink',
          file: 'auth_router.go',
        }),
      ];

      const chainLookup = new Map<string, ChainInfo>();
      chainLookup.set('POST /magiclink', {
        nextRequestName: 'Check Email Exists',
      });

      const results = runSync(oldRoutes, newRoutes, docsDir, '/api/v1', chainLookup);

      assert.equal(results.length, 1);
      assert.equal(results[0].action, 'created');
      assert.equal(results[0].success, true);

      const docPath = join(authDir, 'magiclink.yml');
      assert.ok(existsSync(docPath));
      const content = readFileSync(docPath, 'utf-8');
      assert.ok(content.includes('setNextRequest("Check Email Exists")'),
        'chainLookup with nextRequestName should write confirmed chain script');
      assert.ok(!content.includes('TODO: set chain'),
        'chainLookup with nextRequestName should not have generic chain TODO');

      rmSync(tmpDir, { recursive: true, force: true });
    });

    await it('uses chainLookup to write detected deps comment in new doc', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-sync-'));
      const docsDir = join(tmpDir, 'docs/api');
      const webhookDir = join(docsDir, 'webhooks');

      const oldRoutes: ParsedRoute[] = [];
      const newRoutes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/hook/{service}',
          handler: 'HandleEnqueueWebHook',
          file: 'webhook_router.go',
        }),
      ];

      const chainLookup = new Map<string, ChainInfo>();
      chainLookup.set('POST /hook/{service}', {
        downstreamDeps: ['POST /callback/service-request', 'GET /service-request/{id}/result'],
      });

      const results = runSync(oldRoutes, newRoutes, docsDir, '/api/v1', chainLookup);

      assert.equal(results.length, 1);
      assert.equal(results[0].action, 'created');
      assert.equal(results[0].success, true);

      const docPath = join(webhookDir, 'hook-service.yml');
      assert.ok(existsSync(docPath));
      const content = readFileSync(docPath, 'utf-8');
      assert.ok(content.includes('Detected downstream: POST /callback/service-request'),
        'chainLookup with downstreamDeps should write dep comment');
      assert.ok(content.includes('TODO: confirm chain order'),
        'chainLookup with downstreamDeps should still have confirm TODO');

      rmSync(tmpDir, { recursive: true, force: true });
    });
  });
});
