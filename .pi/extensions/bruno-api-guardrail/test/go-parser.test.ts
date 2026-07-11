import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseGoRoutes, type ParsedRoute } from '../go-parser.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const FIXTURES = join(__dirname, 'fixtures/routers');

function loadFixture(name: string): string {
  return readFileSync(join(FIXTURES, name), 'utf-8');
}

await describe('go-parser', async () => {
  await describe('parseGoRoutes', async () => {
    await it('parses auth routes with middleware and handler', () => {
      const source = loadFixture('auth_router.go');
      const routes = parseGoRoutes(source, 'auth_router.go');

      assert.equal(routes.length, 5);
      assert.equal(routes[0].method, 'POST');
      assert.equal(routes[0].path, '/magiclink');
      assert.equal(routes[0].handler, 'CreateMagicLink');
      assert.equal(routes[0].file, 'auth_router.go');
    });

    await it('parses all HTTP methods from chi patterns', () => {
      const source = loadFixture('auth_router.go');
      const routes = parseGoRoutes(source, 'auth_router.go');

      const methods = routes.map(r => r.method);
      assert.ok(methods.includes('POST'));
      assert.ok(methods.includes('GET'));
      assert.ok(methods.includes('PATCH'));
      assert.ok(!methods.includes('DELETE'));
    });

    await it('parses payment routes', () => {
      const source = loadFixture('payment_router.go');
      const routes = parseGoRoutes(source, 'payment_router.go');

      assert.equal(routes.length, 3);
      assert.equal(routes[1].method, 'POST');
      assert.equal(routes[1].path, '/pay/service');
      assert.equal(routes[1].handler, 'CreatePay');
    });

    await it('parses webhook routes with path parameters', () => {
      const source = loadFixture('webhook_router.go');
      const routes = parseGoRoutes(source, 'webhook_router.go');

      assert.equal(routes.length, 4);
      assert.equal(routes[0].method, 'POST');
      assert.equal(routes[0].path, '/hook/synchronous/{service}');
      assert.equal(routes[0].handler, 'HandleSynchronousWebHook');
      assert.equal(routes[3].method, 'GET');
      assert.equal(routes[3].path, '/service-request/{id}/result');
      assert.equal(routes[3].handler, 'HandleGetAsyncServiceResult');
    });

    await it('parses schedule routes', () => {
      const source = loadFixture('schedule_router.go');
      const routes = parseGoRoutes(source, 'schedule_router.go');

      assert.equal(routes.length, 1);
      assert.equal(routes[0].method, 'POST');
      assert.equal(routes[0].path, '/schedule/unavailable');
      assert.equal(routes[0].handler, 'SetUnavailable');
    });

    await it('parses organization routes with path parameters', () => {
      const source = loadFixture('organization_router.go');
      const routes = parseGoRoutes(source, 'organization_router.go');

      assert.equal(routes.length, 1);
      assert.equal(routes[0].method, 'POST');
      assert.equal(routes[0].path, '/organizations/{organizationId}/roles');
      assert.equal(routes[0].handler, 'RegisterPractitionerRole');
    });

    await it('only extracts actual route registrations, not middleware calls', () => {
      const source = loadFixture('auth_router.go');
      const routes = parseGoRoutes(source, 'auth_router.go');

      // Every parsed route must be a real endpoint registration
      assert.equal(routes.length, 5);
      const handlerNames = routes.map(r => r.handler);
      assert.ok(handlerNames.includes('CreateMagicLink'));
      assert.ok(handlerNames.includes('CreateAnonymousSession'));
      assert.ok(handlerNames.includes('ClaimAnonymousResources'));
      assert.ok(handlerNames.includes('PasswordlessEmailExists'));
      assert.ok(handlerNames.includes('SetActiveRole'));
    });

    await it('returns empty array for empty source', () => {
      const routes = parseGoRoutes('package routers', 'empty.go');
      assert.deepEqual(routes, []);
    });

    await it('returns empty array for source with no route registrations', () => {
      const source = `package routers
func someHelper() string { return "ok" }`;
      const routes = parseGoRoutes(source, 'helper.go');
      assert.deepEqual(routes, []);
    });
  });
});
