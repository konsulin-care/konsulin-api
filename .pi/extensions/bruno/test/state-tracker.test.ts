import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { type ParsedRoute } from '../go-parser.ts';
import {
  type RouteState,
  type RouteDiff,
  computeDiff,
  buildRouteKey,
  shouldTrackFile,
} from '../state-tracker.ts';

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

await describe('state-tracker', async () => {
  await describe('buildRouteKey', async () => {
    await it('creates a composite key from method and path', () => {
      const route = makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink' });
      assert.equal(buildRouteKey(route), 'POST /magiclink');
    });

    await it('handles paths with parameters', () => {
      const route = makeRoute({ method: 'GET', path: '/hook/{service}', handler: 'HandleWebhook' });
      assert.equal(buildRouteKey(route), 'GET /hook/{service}');
    });
  });

  await describe('shouldTrackFile', async () => {
    await it('tracks router files', () => {
      assert.ok(shouldTrackFile('auth_router.go'));
      assert.ok(shouldTrackFile('payment_router.go'));
    });

    await it('tracks controller files', () => {
      assert.ok(shouldTrackFile('auth_controller.go'));
      assert.ok(shouldTrackFile('payment_controller.go'));
    });

    await it('rejects non-Go files', () => {
      assert.equal(shouldTrackFile('main.go'), false);
      assert.equal(shouldTrackFile('types.ts'), false);
    });

    await it('rejects Bruno doc files', () => {
      assert.equal(shouldTrackFile('/docs/api/auth/magiclink.yml'), false);
    });
  });

  await describe('computeDiff', async () => {
    await it('detects new routes', () => {
      const oldRoutes: ParsedRoute[] = [];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
      ];

      const diff = computeDiff(oldRoutes, newRoutes);
      assert.equal(diff.new.length, 1);
      assert.equal(diff.deleted.length, 0);
      assert.equal(diff.modified.length, 0);
      assert.equal(diff.stale.length, 0);
      assert.equal(diff.new[0].handler, 'CreateMagicLink');
    });

    await it('detects deleted routes', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
      ];
      const newRoutes: ParsedRoute[] = [];

      const diff = computeDiff(oldRoutes, newRoutes);
      assert.equal(diff.new.length, 0);
      assert.equal(diff.deleted.length, 1);
      assert.equal(diff.modified.length, 0);
      assert.equal(diff.deleted[0].handler, 'CreateMagicLink');
    });

    await it('treats changed method as delete + create (key changed)', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'GET', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
      ];

      const diff = computeDiff(oldRoutes, newRoutes);
      // Key 'POST /magiclink' is deleted, 'GET /magiclink' is new
      assert.equal(diff.new.length, 1);
      assert.equal(diff.new[0].method, 'GET');
      assert.equal(diff.deleted.length, 1);
      assert.equal(diff.deleted[0].method, 'POST');
      assert.equal(diff.modified.length, 0);
    });

    await it('treats changed path as delete + create (key changed)', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/old-path', handler: 'MyHandler', file: 'r.go' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/new-path', handler: 'MyHandler', file: 'r.go' }),
      ];

      const diff = computeDiff(oldRoutes, newRoutes);
      assert.equal(diff.new.length, 1);
      assert.equal(diff.new[0].path, '/new-path');
      assert.equal(diff.deleted.length, 1);
      assert.equal(diff.deleted[0].path, '/old-path');
      assert.equal(diff.modified.length, 0);
    });

    await it('detects stale handlers (same key, different handler name)', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/magiclink', handler: 'OldHandler', file: 'auth_router.go' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/magiclink', handler: 'NewHandler', file: 'auth_router.go' }),
      ];

      const diff = computeDiff(oldRoutes, newRoutes);
      assert.equal(diff.stale.length, 1);
      assert.equal(diff.modified.length, 0);
      assert.equal(diff.stale[0].key, 'POST /magiclink');
      assert.equal(diff.stale[0].from.handler, 'OldHandler');
      assert.equal(diff.stale[0].to.handler, 'NewHandler');
    });

    await it('returns empty diff for identical routes', () => {
      const routes = [
        makeRoute({ method: 'POST', path: '/a', handler: 'A' }),
        makeRoute({ method: 'GET', path: '/b', handler: 'B' }),
      ];

      const diff = computeDiff(routes, routes);
      assert.equal(diff.new.length, 0);
      assert.equal(diff.deleted.length, 0);
      assert.equal(diff.modified.length, 0);
      assert.equal(diff.stale.length, 0);
    });

    await it('handles mixed changes simultaneously', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/a', handler: 'A', file: 'r1.go' }),
        makeRoute({ method: 'GET', path: '/b', handler: 'B', file: 'r1.go' }),
        makeRoute({ method: 'POST', path: '/c', handler: 'C', file: 'r1.go' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/a', handler: 'A', file: 'r1.go' }),  // unchanged
        makeRoute({ method: 'POST', path: '/c', handler: 'CNew', file: 'r1.go' }), // stale handler
        makeRoute({ method: 'POST', path: '/d', handler: 'D', file: 'r1.go' }),    // new
      ];

      const diff = computeDiff(oldRoutes, newRoutes);
      assert.equal(diff.new.length, 1);
      assert.equal(diff.new[0].handler, 'D');
      assert.equal(diff.deleted.length, 1);
      assert.equal(diff.deleted[0].handler, 'B');
      assert.equal(diff.stale.length, 1);
      assert.equal(diff.stale[0].key, 'POST /c');
      assert.equal(diff.modified.length, 0);
    });

    await it('consideres path change as modification', () => {
      const oldRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/old-path', handler: 'MyHandler', file: 'r.go' }),
      ];
      const newRoutes: ParsedRoute[] = [
        makeRoute({ method: 'POST', path: '/old-path/changed', handler: 'MyHandler', file: 'r.go' }),
      ];

      const diff = computeDiff(oldRoutes, newRoutes);
      assert.equal(diff.new.length, 1); // old key doesn't exist, new key doesn't exist in old
      assert.equal(diff.deleted.length, 1); // old key removed
      assert.equal(diff.modified.length, 0);
    });
  });
});
