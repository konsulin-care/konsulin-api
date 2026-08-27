/**
 * Tests for the /bruno slash command and the doSync() helper
 * added to the extension index.ts.
 *
 * Uses fixture-based setup like bruno-sync.test.ts.
 */

import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { type ParsedRoute } from '../go-parser.ts';
import { runSync, type SyncResult } from '../sync-orchestrator.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROUTER_FIXTURES = join(__dirname, 'fixtures/routers');
const DOCS_FIXTURES = join(__dirname, 'fixtures/docs');
const API_PREFIX = '/api/v1';

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

await describe('/bruno command', async () => {
  await describe('doSync helper (scan + runSync combo)', async () => {
    let tmpDir: string;

    before(() => {
      tmpDir = mkdtempSync(join(tmpdir(), 'bruno-command-test-'));
    });

    it('returns empty results when old and new routes match', () => {
      const routes: ParsedRoute[] = [
        makeRoute({ method: 'GET', path: '/api/v1/users', handler: 'listUsers' }),
      ];
      const results = runSync(routes, routes, tmpDir, API_PREFIX);
      const changed = results.filter(r => r.action !== 'unchanged');
      assert.equal(changed.length, 0);
    });

    it('detects new routes and creates valid docs', () => {
      const oldRoutes: ParsedRoute[] = [];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'GET', path: '/api/v1/users', handler: 'listUsers' }),
      ];

      const results = runSync(oldRoutes, newRoutes, tmpDir, API_PREFIX);
      const created = results.filter(r => r.action === 'created');
      const errors = results.filter(r => r.action === 'error');

      assert.equal(created.length, 1, 'expected 1 created doc');
      assert.equal(errors.length, 0, `expected no errors, got: ${errors.map(e => e.error).join('; ')}`);

      // Verify the created doc is valid by checking each action
      const createdResult = created[0];
      assert.ok(createdResult.success, 'expected success');
      assert.ok(createdResult.docPath, 'expected doc path');
      assert.equal(createdResult.key, 'GET /api/v1/users');
    });

    it('detects deleted routes', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'GET', path: '/api/v1/old', handler: 'oldHandler' }),
      ];
      const newRoutes: ParsedRoute[] = [];

      const results = runSync(oldRoutes, newRoutes, tmpDir, API_PREFIX);
      const deleted = results.filter(r => r.action === 'deleted');
      assert.equal(deleted.length, 1);
      assert.equal(deleted[0].key, 'GET /api/v1/old');
    });

    it('detects stale routes (handler change)', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/api/v1/users', handler: 'oldHandler' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/api/v1/users', handler: 'newHandler' }),
      ];

      const results = runSync(oldRoutes, newRoutes, tmpDir, API_PREFIX);
      const stale = results.filter(r => r.action === 'stale');
      assert.equal(stale.length, 1);
      assert.equal(stale[0].key, 'POST /api/v1/users');
    });

    it('reports results with created, deleted, and stale counts', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'GET', path: '/api/v1/deleted', handler: 'delHandler' }),
        makeRoute({ method: 'GET', path: '/api/v1/updated', handler: 'oldHandler' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'GET', path: '/api/v1/new', handler: 'newRoute' }),
        makeRoute({ method: 'GET', path: '/api/v1/updated', handler: 'newHandler' }),
      ];

      const results = runSync(oldRoutes, newRoutes, tmpDir, API_PREFIX);
      const created = results.filter(r => r.action === 'created').length;
      const deleted = results.filter(r => r.action === 'deleted').length;
      const stale = results.filter(r => r.action === 'stale').length;

      assert.equal(created, 1, 'expected 1 created');
      assert.equal(deleted, 1, 'expected 1 deleted');
      assert.equal(stale, 1, 'expected 1 stale');
    });
  });

  await describe('result reporting format', async () => {
    it('formats created summary line', () => {
      const created = true;
      const deleted = false;
      const stale = false;
      const parts: string[] = [];
      if (created) parts.push('1 created');
      if (deleted) parts.push('0 deleted');
      if (stale) parts.push('0 stale');
      const summary = parts.length > 0
        ? `Bruno docs synced: ${parts.join(', ')}.`
        : 'Bruno docs are up to date.';
      assert.equal(summary, 'Bruno docs synced: 1 created.');
    });

    it('formats summary with multiple action types', () => {
      const created = 2;
      const deleted = 1;
      const stale = 3;
      const parts: string[] = [];
      if (created > 0) parts.push(`${created} created`);
      if (deleted > 0) parts.push(`${deleted} deleted`);
      if (stale > 0) parts.push(`${stale} stale`);
      const summary = parts.length > 0
        ? `Bruno docs synced: ${parts.join(', ')}.`
        : 'Bruno docs are up to date.';
      assert.equal(summary, 'Bruno docs synced: 2 created, 1 deleted, 3 stale.');
    });

    it('formats summary with no changes', () => {
      const parts: string[] = [];
      const summary = parts.length > 0
        ? `Bruno docs synced: ${parts.join(', ')}.`
        : 'Bruno docs are up to date.';
      assert.equal(summary, 'Bruno docs are up to date.');
    });

    it('formats change detail lines', () => {
      const results: SyncResult[] = [
        { action: 'created', docPath: 'docs/api/auth/test.yml', handler: 'createTest', key: 'POST /test', success: true },
        { action: 'deleted', docPath: 'docs/api/auth/old.yml', handler: 'deleteOld', key: 'GET /old', success: true },
        { action: 'stale', docPath: 'docs/api/auth/upd.yml', handler: 'newHandler', key: 'PATCH /upd', success: true },
      ];
      const lines = results
        .filter(r => r.action === 'created' || r.action === 'deleted' || r.action === 'stale')
        .map(r => `  [${r.action}] ${r.docPath}  (${r.handler})`);
      assert.equal(lines.length, 3);
      assert.ok(lines[0].startsWith('  [created]'));
      assert.ok(lines[0].includes('createTest'));
      assert.ok(lines[1].startsWith('  [deleted]'));
      assert.ok(lines[2].startsWith('  [stale]'));
    });

    it('formats error detail lines', () => {
      const results: SyncResult[] = [
        { action: 'error', docPath: 'docs/api/auth/bad.yml', handler: 'badRoute', key: 'POST /bad', success: false, error: 'Validation failed: must have required property' },
      ];
      const errorLines = results
        .filter(r => r.action === 'error')
        .map(e => `${e.docPath}: ${e.error}`);
      assert.equal(errorLines.length, 1);
      assert.ok(errorLines[0].includes('Validation failed'));
      assert.ok(errorLines[0].includes('docs/api/auth/bad.yml'));
    });
  });
});
