import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { validateBrunoYaml } from '../bruno-validator.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const FIXTURES = join(__dirname, 'fixtures/docs/api');

await describe('bruno-validator (ajv)', async () => {
  await describe('Schema-compliance', async () => {
    await it('validates existing magiclink.yml as valid', () => {
      const errors = validateBrunoYaml(join(FIXTURES, 'auth/magiclink.yml'));
      assert.deepEqual(errors, []);
    });

    await it('validates existing payment yml as valid', () => {
      const errors = validateBrunoYaml(join(FIXTURES, 'payments/create-service.yml'));
      assert.deepEqual(errors, []);
    });

    await it('validates folder.yml (type: folder) as valid', () => {
      const errors = validateBrunoYaml(join(FIXTURES, 'auth/folder.yml'));
      assert.deepEqual(errors, []);
    });

    await it('accepts minimal valid HTTP request skeleton', () => {
      const yaml = `info:
  name: "Test Endpoint"
  type: http
  seq: 1
http:
  method: POST
  url: "{{process.env.APP_BASE_URL}}/api/v1/test"
settings:
  encodeUrl: true
  timeout: 0
  followRedirects: true
  maxRedirects: 5
`;
      const errors = validateBrunoYaml(yaml, { isContent: true });
      assert.deepEqual(errors, []);
    });
  });

  await describe('Error reporting', async () => {
    await it('rejects unknown info.type value', () => {
      const errors = validateBrunoYaml(`
info:
  name: Test
  type: invalid_type
  seq: 1
http:
  method: POST
  url: "http://localhost/test"
      `.trim(), { isContent: true });
      assert.ok(errors.length > 0);
      assert.ok(errors.some(e => e.instancePath.includes('info/type')));
    });

    await it('reports missing info section as unknown type', () => {
      const errors = validateBrunoYaml(`
http:
  method: POST
  url: "http://localhost/test"
      `.trim(), { isContent: true });
      assert.ok(errors.length > 0);
    });

    await it('rejects additional properties in info block', () => {
      const errors = validateBrunoYaml(`
info:
  name: Test
  type: http
  seq: 1
  unknownField: should-not-exist
http:
  method: POST
  url: "http://localhost/test"
      `.trim(), { isContent: true });
      assert.ok(errors.length > 0);
      assert.ok(errors.some(e => e.instancePath === '/info'));
    });

    await it('rejects additional properties in http block', () => {
      const errors = validateBrunoYaml(`
info:
  name: Test
  type: http
  seq: 1
http:
  method: POST
  url: "http://localhost/test"
  unknownField: should-not-exist
      `.trim(), { isContent: true });
      assert.ok(errors.length > 0);
      assert.ok(errors.some(e => e.instancePath === '/http'));
    });

    await it('rejects invalid body type', () => {
      const errors = validateBrunoYaml(`
info:
  name: Test
  type: http
  seq: 1
http:
  method: POST
  url: "http://localhost/test"
  body:
    type: invalid
    data: "{}"
      `.trim(), { isContent: true });
      assert.ok(errors.length > 0);
    });

    await it('rejects non-existent file', () => {
      const errors = validateBrunoYaml('/nonexistent/file.yml');
      assert.ok(errors.length > 0);
      assert.ok(errors.some(e => e.message.includes('not found')));
    });
  });
});
