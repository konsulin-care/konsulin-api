import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { type ParsedRoute } from '../go-parser.ts';
import {
  generateBrunoSkeleton,
  buildBrunoPath,
  findNextSeq,
  deleteBrunoDoc,
  type BrunoDocEntry,
} from '../bruno-generator.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const DOCS_FIXTURES = join(__dirname, 'fixtures/docs/api');

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

await describe('bruno-generator', async () => {
  await describe('findNextSeq', async () => {
    await it('returns 1 for empty directory', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-test-'));
      try {
        const seq = findNextSeq(tmpDir);
        assert.equal(seq, 1);
      } finally {
        rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    await it('returns next seq after existing files', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-test-'));
      try {
        writeFileSync(join(tmpDir, 'file1.yml'), 'info:\n  name: A\n  seq: 1\n');
        writeFileSync(join(tmpDir, 'file2.yml'), 'info:\n  name: B\n  seq: 3\n');
        writeFileSync(join(tmpDir, 'file3.yml'), 'info:\n  name: C\n  seq: 2\n');
        const seq = findNextSeq(tmpDir);
        assert.equal(seq, 4);
      } finally {
        rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    await it('handles files without seq field', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-test-'));
      try {
        writeFileSync(join(tmpDir, 'file1.yml'), 'info:\n  name: A\n');
        const seq = findNextSeq(tmpDir);
        assert.equal(seq, 1);
      } finally {
        rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    await it('reads seq from existing auth folder', () => {
      const seq = findNextSeq(join(DOCS_FIXTURES, 'auth'));
      assert.equal(seq, 4); // existing seq: 1, 2, 3
    });

    await it('reads seq from existing payments folder', () => {
      const seq = findNextSeq(join(DOCS_FIXTURES, 'payments'));
      assert.equal(seq, 4); // existing seq: 1, 2, 3
    });
  });

  await describe('buildBrunoPath', async () => {
    await it('builds path for auth route', () => {
      const route = makeRoute({
        method: 'POST',
        path: '/magiclink',
        handler: 'CreateMagicLink',
        file: 'auth_router.go',
      });
      const path = buildBrunoPath(route, DOCS_FIXTURES);
      assert.ok(path.endsWith('/auth/magiclink.yml'));
    });

    await it('builds path for payment route', () => {
      const route = makeRoute({
        method: 'POST',
        path: '/pay/service',
        handler: 'CreatePay',
        file: 'payment_router.go',
      });
      const path = buildBrunoPath(route, DOCS_FIXTURES);
      assert.ok(path.endsWith('/payments/pay-service.yml'));
    });

    await it('builds path for nested path route', () => {
      const route = makeRoute({
        method: 'GET',
        path: '/passwordless/email/exists',
        handler: 'PasswordlessEmailExists',
        file: 'auth_router.go',
      });
      const path = buildBrunoPath(route, DOCS_FIXTURES);
      assert.ok(path.endsWith('/auth/passwordless-email-exists.yml'));
    });
  });

  await describe('generateBrunoSkeleton', async () => {
    await it('generates valid YAML skeleton for a new route', () => {
      const route = makeRoute({
        method: 'POST',
        path: '/magiclink',
        handler: 'CreateMagicLink',
        file: 'auth_router.go',
      });
      const yaml = generateBrunoSkeleton(route, 1, '/api/v1');

      assert.ok(yaml.includes('name: "Create Magic Link"'));
      assert.ok(yaml.includes('seq: 1'));
      assert.ok(yaml.includes('method: POST'));
      assert.ok(yaml.includes('/api/v1/magiclink'));
      assert.ok(yaml.includes('encodeUrl: true'));
      assert.ok(yaml.includes('type: http'));
    });

    await it('includes tags placeholder in skeleton', () => {
      const route = makeRoute({
        method: 'POST',
        path: '/pay/service',
        handler: 'CreatePay',
        file: 'payment_router.go',
      });
      const yaml = generateBrunoSkeleton(route, 1, '/api/v1');

      assert.ok(yaml.includes('tags:'));
      assert.ok(yaml.includes('after-response'));
      assert.ok(yaml.includes('setNextRequest'));
    });

    await it('generates skeleton for GET endpoint', () => {
      const route = makeRoute({
        method: 'GET',
        path: '/passwordless/email/exists',
        handler: 'PasswordlessEmailExists',
        file: 'auth_router.go',
      });
      const yaml = generateBrunoSkeleton(route, 2, '/api/v1');

      assert.ok(yaml.includes('method: GET'));
      assert.ok(yaml.includes('/api/v1/passwordless/email/exists'));
      assert.ok(yaml.includes('seq: 2'));
    });

    await it('generates skeleton for PATCH endpoint', () => {
      const route = makeRoute({
        method: 'PATCH',
        path: '/anonymous/claim',
        handler: 'ClaimAnonymousResources',
        file: 'auth_router.go',
      });
      const yaml = generateBrunoSkeleton(route, 5, '/api/v1');

      assert.ok(yaml.includes('method: PATCH'));
      assert.ok(yaml.includes('seq: 5'));
    });
  });

  await describe('deleteBrunoDoc', async () => {
    await it('deletes existing Bruno doc file', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-test-'));
      try {
        const docPath = join(tmpDir, 'test.yml');
        writeFileSync(docPath, 'info:\n  name: Test\n');
        assert.ok(existsSync(docPath));

        const result = deleteBrunoDoc(docPath, tmpDir);
        assert.equal(result, true);
        assert.equal(existsSync(docPath), false);
      } finally {
        rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    await it('returns false for non-existent file', () => {
      const tmpDir = mkdtempSync(join(tmpdir(), 'bruno-test-'));
      try {
        const result = deleteBrunoDoc(join(tmpDir, 'nonexistent.yml'), tmpDir);
        assert.equal(result, false);
      } finally {
        rmSync(tmpDir, { recursive: true, force: true });
      }
    });
  });

  await describe('end-to-end: auth route generation', async () => {
    await it('generates a skeleton matching the existing magiclink.yml structure', () => {
      const route = makeRoute({
        method: 'POST',
        path: '/magiclink',
        handler: 'CreateMagicLink',
        file: 'auth_router.go',
      });
      const yaml = generateBrunoSkeleton(route, 1, '/api/v1');

      // Should contain the same structural elements as the existing doc
      assert.ok(yaml.includes('info:'));
      assert.ok(yaml.includes('http:'));
      assert.ok(yaml.includes('runtime:'));
      assert.ok(yaml.includes('settings:'));
      assert.ok(yaml.includes('examples:'));
      assert.ok(yaml.includes('docs: |'));
    });

    await it('generates skeleton with correct URL template', () => {
      const route = makeRoute({
        method: 'POST',
        path: '/pay/service',
        handler: 'CreatePay',
        file: 'payment_router.go',
      });
      const yaml = generateBrunoSkeleton(route, 1, '/api/v1');

      assert.ok(yaml.includes("url: \"{{process.env.APP_BASE_URL}}/api/v1/pay/service\""));
    });
  });
});
