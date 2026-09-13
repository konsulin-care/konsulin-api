/**
 * Tests for doc-inventory.ts — Bruno doc inventory scanning and classification.
 *
 * Verifies that scanDocInventory correctly classifies Bruno YAML files as
 * managed, external, or unrecognized based on Go route definitions and
 * known external route/domain patterns.
 */

import { describe, it, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, mkdirSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { type ParsedRoute } from '../go-parser.ts';
import {
  scanDocInventory,
  EXTERNAL_ROUTES,
  EXTERNAL_DOMAINS,
  formatInventoryReport,
  type DocInventory,
  type DocInventoryEntry,
} from '../doc-inventory.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
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

function makeDoc(name: string, method: string, urlPath: string): string {
  return `info:
  name: "${name}"
  type: http
  seq: 1

http:
  method: ${method}
  url: "{{process.env.APP_BASE_URL}}${urlPath}"
  headers:
    - name: content-type
      value: application/json
  body:
    type: json
    data: "{}"
`;
}

function makeExternalDoc(name: string, method: string, url: string): string {
  return `info:
  name: "${name}"
  type: http
  seq: 1

http:
  method: ${method}
  url: "${url}"
  body:
    type: json
    data: "{}"
`;
}

/** Create a fresh temp docs/api/ directory and return its path. */
function freshDocsDir(): string {
  const tmp = mkdtempSync(join(tmpdir(), 'doc-inventory-test-'));
  const dir = join(tmp, 'docs', 'api');
  mkdirSync(dir, { recursive: true });
  return dir;
}

/** Clean up a temp directory. */
function cleanup(dir: string): void {
  rmSync(dir, { recursive: true, force: true });
}

await describe('DocInventory', async () => {
  await describe('EXTERNAL_ROUTES constant', async () => {
    it('contains SuperTokens SDK signout route', () => {
      assert.ok(EXTERNAL_ROUTES.has('POST /auth/signout'));
    });
    it('contains SuperTokens SDK code consume route', () => {
      assert.ok(EXTERNAL_ROUTES.has('POST /auth/signinup/code/consume'));
    });
    it('has exactly 2 known external routes', () => {
      assert.equal(EXTERNAL_ROUTES.size, 2);
    });
  });

  await describe('EXTERNAL_DOMAINS constant', async () => {
    it('contains misc domain', () => {
      assert.ok(EXTERNAL_DOMAINS.has('misc'));
    });
    it('contains fhir domain', () => {
      assert.ok(EXTERNAL_DOMAINS.has('fhir'));
    });
    it('has exactly 2 external domains', () => {
      assert.equal(EXTERNAL_DOMAINS.size, 2);
    });
  });

  await describe('scanDocInventory', async () => {
    it('returns empty inventory when docs directory does not exist', () => {
      const inventory = scanDocInventory(
        '/nonexistent/path',
        [],
        API_PREFIX,
        EXTERNAL_ROUTES,
        EXTERNAL_DOMAINS,
      );
      assert.equal(inventory.total, 0);
      assert.equal(inventory.managed.length, 0);
      assert.equal(inventory.external.length, 0);
      assert.equal(inventory.unrecognized.length, 0);
    });

    it('returns empty inventory when docs directory is empty', () => {
      const docsDir = freshDocsDir();
      try {
        const inventory = scanDocInventory(docsDir, [], API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);
        assert.equal(inventory.total, 0);
      } finally {
        cleanup(docsDir);
      }
    });

    it('classifies managed docs that match Go routes', () => {
      const docsDir = freshDocsDir();
      try {
        mkdirSync(join(docsDir, 'auth'), { recursive: true });
        writeFileSync(
          join(docsDir, 'auth', 'magiclink.yml'),
          makeDoc('Send Magic Link', 'POST', '/api/v1/auth/magiclink'),
        );
        writeFileSync(
          join(docsDir, 'auth', 'folder.yml'),
          `info:\n  name: auth\n  type: folder\n  seq: 1\n\nrequest: {}\n`,
        );

        const routes: ParsedRoute[] = [
          makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
        ];

        const inventory = scanDocInventory(docsDir, routes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        assert.equal(inventory.total, 1, 'folder.yml should be excluded');
        assert.equal(inventory.managed.length, 1);
        assert.equal(inventory.external.length, 0);
        assert.equal(inventory.unrecognized.length, 0);

        const entry = inventory.managed[0];
        assert.equal(entry.relPath, 'auth/magiclink.yml');
        assert.equal(entry.name, 'Send Magic Link');
        assert.equal(entry.category, 'managed');
        assert.equal(entry.routeKey, 'POST /magiclink');
      } finally {
        cleanup(docsDir);
      }
    });

    it('classifies external route docs (SuperTokens SDK) in non-external domain', () => {
      const docsDir = freshDocsDir();
      try {
        mkdirSync(join(docsDir, 'auth'), { recursive: true });

        // A managed doc that has a matching route
        writeFileSync(
          join(docsDir, 'auth', 'magiclink.yml'),
          makeDoc('Send Magic Link', 'POST', '/api/v1/auth/magiclink'),
        );

        // An external doc (SuperTokens SDK) that doesn't match any Go route
        writeFileSync(
          join(docsDir, 'auth', 'signout.yml'),
          makeDoc('Sign Out', 'POST', '/api/v1/auth/signout'),
        );

        const routes: ParsedRoute[] = [
          makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
        ];

        const inventory = scanDocInventory(docsDir, routes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        assert.equal(inventory.total, 2);
        assert.equal(inventory.managed.length, 1);
        assert.equal(inventory.external.length, 1);
        assert.equal(inventory.unrecognized.length, 0);

        const ext = inventory.external.find(e => e.relPath === 'auth/signout.yml');
        assert.ok(ext);
        assert.equal(ext!.category, 'external');
        assert.equal(ext!.routeKey, 'POST /auth/signout');
      } finally {
        cleanup(docsDir);
      }
    });

    it('classifies docs in external domains (misc, fhir) as external', () => {
      const docsDir = freshDocsDir();
      try {
        // misc domain — entirely external
        mkdirSync(join(docsDir, 'misc'), { recursive: true });
        writeFileSync(
          join(docsDir, 'misc', 'mailinator-inbox.yml'),
          makeExternalDoc('Mailinator Poll', 'GET', 'https://api.mailinator.com/test'),
        );

        // fhir domain — entirely external
        mkdirSync(join(docsDir, 'fhir'), { recursive: true });
        writeFileSync(
          join(docsDir, 'fhir', 'metadata.yml'),
          makeDoc('FHIR Metadata', 'GET', '/fhir/metadata'),
        );

        const inventory = scanDocInventory(docsDir, [], API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        assert.equal(inventory.total, 2);
        assert.equal(inventory.external.length, 2);

        const miscExt = inventory.external.find(e => e.relPath === 'misc/mailinator-inbox.yml');
        assert.ok(miscExt);

        const fhirExt = inventory.external.find(e => e.relPath === 'fhir/metadata.yml');
        assert.ok(fhirExt);
      } finally {
        cleanup(docsDir);
      }
    });

    it('classifies unrecognized docs that match neither Go routes nor external patterns', () => {
      const docsDir = freshDocsDir();
      try {
        mkdirSync(join(docsDir, 'auth'), { recursive: true });

        // A managed doc
        writeFileSync(
          join(docsDir, 'auth', 'magiclink.yml'),
          makeDoc('Send Magic Link', 'POST', '/api/v1/auth/magiclink'),
        );

        // An external doc (SuperTokens)
        writeFileSync(
          join(docsDir, 'auth', 'signout.yml'),
          makeDoc('Sign Out', 'POST', '/api/v1/auth/signout'),
        );

        // An unrecognized doc — no matching route, not in external patterns
        writeFileSync(
          join(docsDir, 'auth', 'unknown.yml'),
          makeDoc('Mystery', 'DELETE', '/api/v1/auth/unknown'),
        );

        const routes: ParsedRoute[] = [
          makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
        ];

        const inventory = scanDocInventory(docsDir, routes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        assert.equal(inventory.total, 3);
        assert.equal(inventory.managed.length, 1);
        assert.equal(inventory.external.length, 1);
        assert.equal(inventory.unrecognized.length, 1);

        const unrec = inventory.unrecognized[0];
        assert.equal(unrec.relPath, 'auth/unknown.yml');
        assert.equal(unrec.category, 'unrecognized');
      } finally {
        cleanup(docsDir);
      }
    });

    it('excludes opencollection.yml from inventory', () => {
      const docsDir = freshDocsDir();
      try {
        mkdirSync(join(docsDir, 'auth'), { recursive: true });
        writeFileSync(
          join(docsDir, 'auth', 'magiclink.yml'),
          makeDoc('Send Magic Link', 'POST', '/api/v1/auth/magiclink'),
        );
        writeFileSync(
          join(docsDir, 'opencollection.yml'),
          `opencollection: 1.0.0\ninfo:\n  name: Test\n`,
        );

        const routes: ParsedRoute[] = [
          makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
        ];

        const inventory = scanDocInventory(docsDir, routes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        // 1 doc (opencollection.yml excluded)
        assert.equal(inventory.total, 1);
        assert.equal(inventory.managed.length, 1);
      } finally {
        cleanup(docsDir);
      }
    });

    it('returns correct category counts for mixed inventory', () => {
      const docsDir = freshDocsDir();
      try {
        mkdirSync(join(docsDir, 'auth'), { recursive: true });
        mkdirSync(join(docsDir, 'misc'), { recursive: true });
        mkdirSync(join(docsDir, 'fhir'), { recursive: true });

        // Managed: auth/magiclink.yml
        writeFileSync(join(docsDir, 'auth', 'magiclink.yml'),
          makeDoc('Send Magic Link', 'POST', '/api/v1/auth/magiclink'));
        // Managed: auth/anonymous-session.yml
        writeFileSync(join(docsDir, 'auth', 'anonymous-session.yml'),
          makeDoc('Create Anonymous Session', 'POST', '/api/v1/auth/anonymous-session'));
        // External (SuperTokens): auth/signout.yml
        writeFileSync(join(docsDir, 'auth', 'signout.yml'),
          makeDoc('Sign Out', 'POST', '/api/v1/auth/signout'));
        // External (SuperTokens): auth/consume-code.yml
        writeFileSync(join(docsDir, 'auth', 'consume-code.yml'),
          makeDoc('Consume Code', 'POST', '/api/v1/auth/signinup/code/consume'));
        // External (Mailinator): misc/mailinator-inbox.yml
        writeFileSync(join(docsDir, 'misc', 'mailinator-inbox.yml'),
          makeExternalDoc('Mailinator Poll', 'GET', 'https://api.mailinator.com/test'));
        // External (FHIR): fhir/metadata.yml
        writeFileSync(join(docsDir, 'fhir', 'metadata.yml'),
          makeDoc('FHIR Metadata', 'GET', '/fhir/metadata'));
        // Unrecognized: auth/unknown.yml
        writeFileSync(join(docsDir, 'auth', 'unknown.yml'),
          makeDoc('Unknown', 'PATCH', '/api/v1/auth/unknown'));

        // folder.yml in each domain
        for (const dir of ['auth', 'misc', 'fhir']) {
          writeFileSync(join(docsDir, dir, 'folder.yml'),
            `info:\n  name: ${dir}\n  type: folder\n  seq: 1\n\nrequest: {}\n`);
        }

        const routes: ParsedRoute[] = [
          makeRoute({ method: 'POST', path: '/magiclink', handler: 'CreateMagicLink', file: 'auth_router.go' }),
          makeRoute({ method: 'POST', path: '/anonymous-session', handler: 'CreateAnonymousSession', file: 'auth_router.go' }),
        ];

        const inventory = scanDocInventory(docsDir, routes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        assert.equal(inventory.total, 7, 'expected 7 non-folder docs');
        assert.equal(inventory.managed.length, 2, 'expected 2 managed');
        assert.equal(inventory.external.length, 4, 'expected 4 external (SuperTokens + misc + fhir)');
        assert.equal(inventory.unrecognized.length, 1, 'expected 1 unrecognized');
      } finally {
        cleanup(docsDir);
      }
    });
  });
});

await describe('formatInventoryReport', async () => {
  it('formats report with no route changes and full inventory', () => {
    const inventory: DocInventory = {
      managed: [
        { relPath: 'auth/magiclink.yml', name: 'Send Magic Link', category: 'managed' },
        { relPath: 'auth/anonymous-session.yml', name: 'Create Anonymous Session', category: 'managed' },
      ],
      external: [
        { relPath: 'auth/signout.yml', name: 'Sign Out', category: 'external', routeKey: 'POST /auth/signout' },
        { relPath: 'auth/consume-code.yml', name: 'Consume Code', category: 'external', routeKey: 'POST /auth/signinup/code/consume' },
        { relPath: 'misc/mailinator-inbox.yml', name: 'Mailinator Poll', category: 'external' },
      ],
      unrecognized: [],
      total: 5,
    };

    const report = formatInventoryReport(inventory, 'Bruno docs synced: no route changes.');

    assert.ok(report.includes('Bruno docs synced: no route changes.'));
    assert.ok(report.includes('Doc inventory: 2 managed, 3 external'));
    assert.ok(report.includes('auth/signout.yml, auth/consume-code.yml'));
    assert.ok(report.includes('misc/mailinator-inbox.yml'));
    assert.ok(!report.includes('Unrecognized'));
  });

  it('formats report with route changes and inventory', () => {
    const inventory: DocInventory = {
      managed: [
        { relPath: 'auth/magiclink.yml', name: 'Send Magic Link', category: 'managed' },
      ],
      external: [
        { relPath: 'auth/signout.yml', name: 'Sign Out', category: 'external' },
      ],
      unrecognized: [],
      total: 2,
    };

    const report = formatInventoryReport(inventory, 'Bruno docs synced: created 1, deleted 0, 0 stale.');

    assert.ok(report.includes('Bruno docs synced: created 1, deleted 0, 0 stale.'));
    assert.ok(report.includes('Doc inventory: 1 managed, 1 external'));
  });

  it('includes warning for unrecognized docs', () => {
    const inventory: DocInventory = {
      managed: [{ relPath: 'auth/magiclink.yml', name: 'Send Magic Link', category: 'managed' }],
      external: [],
      unrecognized: [
        { relPath: 'auth/unknown.yml', name: 'Mystery', category: 'unrecognized' },
      ],
      total: 2,
    };

    const report = formatInventoryReport(inventory, 'Bruno docs synced: no route changes.');

    assert.ok(report.includes('Doc inventory: 1 managed, 0 external, 1 unrecognized'));
    assert.ok(report.includes('⚠ Unrecognized: auth/unknown.yml'));
    assert.ok(report.includes('no matching Go route'));
  });

  it('handles empty inventory', () => {
    const inventory: DocInventory = {
      managed: [], external: [], unrecognized: [], total: 0,
    };

    const report = formatInventoryReport(inventory, 'Bruno docs synced: no route changes.');

    assert.ok(report.includes('Doc inventory: 0 managed, 0 external'));
    assert.ok(!report.includes('Unrecognized'));
  });

  it('groups external docs by domain', () => {
    const inventory: DocInventory = {
      managed: [],
      external: [
        { relPath: 'auth/signout.yml', name: 'Sign Out', category: 'external' },
        { relPath: 'auth/consume-code.yml', name: 'Consume Code', category: 'external' },
        { relPath: 'misc/mailinator-inbox.yml', name: 'Mailinator', category: 'external' },
        { relPath: 'misc/mailinator-message.yml', name: 'Get Magic Link', category: 'external' },
        { relPath: 'fhir/metadata.yml', name: 'FHIR Metadata', category: 'external' },
      ],
      unrecognized: [],
      total: 5,
    };

    const report = formatInventoryReport(inventory, 'Bruno docs synced: no route changes.');

    // Should have 3 External lines (one per domain)
    const externalLines = report.split('\n').filter(l => l.startsWith('  External:'));
    assert.equal(externalLines.length, 3);
    assert.ok(externalLines.some(l => l.includes('auth/signout.yml, auth/consume-code.yml')));
    assert.ok(externalLines.some(l => l.includes('misc/mailinator-inbox.yml, misc/mailinator-message.yml')));
    assert.ok(externalLines.some(l => l.includes('fhir/metadata.yml')));
  });
});
