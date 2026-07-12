/**
 * Bruno API Guardrail Extension
 *
 * Auto-syncs Bruno API docs with Go chi route definitions.
 * Hooks tool_call to track file changes, turn_end to auto-sync,
 * registers sync_bruno_api_docs for explicit invocation,
 * and provides a /bruno slash command for manual sync.
 *
 * Lifecycle:
 *   session_start → parse routes → baseline
 *   tool_call (write/edit/bash touching routers/*.go) → dirty flag
 *   turn_end → doSync → validate → auto-inject on failure
 *   /bruno → doSync → report via ctx.ui
 *   sync_bruno_api_docs → doSync → return results
 */

import { type ParsedRoute } from './go-parser.ts';
import { shouldTrackFile } from './state-tracker.ts';
import {
  runSync,
  scanAllRouterFiles,
  scanDocInventory,
  EXTERNAL_ROUTES,
  EXTERNAL_DOMAINS,
  formatInventoryReport,
} from './sync-orchestrator.ts';

const DOCS_BASE = 'docs/api';
const API_PREFIX = '/api/v1';

export {
  runSync,
  scanAllRouterFiles,
  scanDocInventory,
  EXTERNAL_ROUTES,
  EXTERNAL_DOMAINS,
} from './sync-orchestrator.ts';
export type { SyncResult, SyncError } from './sync-orchestrator.ts';
export type { DocInventory, DocInventoryEntry } from './sync-orchestrator.ts';

export default function (pi: any) {
  let dirty = false;
  let currentRoutes: ParsedRoute[] = [];
  const projectRoot = process.cwd();

  pi.on('session_start', async (_event: any, _ctx: any) => {
    currentRoutes = scanAllRouterFiles(projectRoot);
    dirty = false;
  });

  pi.on('session_shutdown', async (_event: any, _ctx: any) => {
    currentRoutes = [];
    dirty = false;
  });

  pi.on('tool_call', async (event: any, _ctx: any) => {
    if (dirty) return;
    const input = event.input ?? {};
    const toolName = event.toolName;

    if (toolName === 'write' || toolName === 'edit') {
      if (shouldTrackFile(input.path ?? '')) {
        dirty = true;
      }
    }

    if (toolName === 'bash') {
      const cmd: string = input.command ?? '';
      if (cmd.includes('routers/') || cmd.includes('controllers/')) {
        dirty = true;
      }
    }
  });

  /**
   * Run the full sync: scan router files, diff against currentRoutes,
   * generate/delete/stale Bruno docs, validate all output.
   *
   * Deduplicates the sync loop between turn_end, tool execute, and /bruno command.
   */
  function doSync(): {
    newRoutes: ParsedRoute[];
    created: number;
    deleted: number;
    stale: number;
    errors: number;
    results: ReturnType<typeof runSync>;
  } {
    const newRoutes = scanAllRouterFiles(projectRoot);
    const results = runSync(currentRoutes, newRoutes, `${projectRoot}/${DOCS_BASE}`, API_PREFIX);
    return {
      newRoutes,
      created: results.filter(r => r.action === 'created').length,
      deleted: results.filter(r => r.action === 'deleted').length,
      stale: results.filter(r => r.action === 'stale').length,
      errors: results.filter(r => r.action === 'error').length,
      results,
    };
  }

  pi.on('turn_end', async (_event: any, ctx: any) => {
    if (!dirty) return;

    const sync = doSync();

    if (sync.errors > 0) {
      const errorSummary = sync.results
        .filter(r => r.action === 'error')
        .map(e => `  File: ${e.docPath}\n  Error: ${e.error}`)
        .join('\n');

      pi.sendMessage({
        customType: 'bruno-sync',
        content: [
          { type: 'text', text: `BRUNO SYNC FAILED\n\n${sync.errors} validation error(s):\n${errorSummary}\n\nFix the YAML and run sync-bruno-api-docs again.` },
        ],
        display: true,
      });
    } else {
      currentRoutes = sync.newRoutes;
      dirty = false;

      const parts: string[] = [];
      if (sync.created > 0) parts.push(`Created ${sync.created} doc(s)`);
      if (sync.deleted > 0) parts.push(`Deleted ${sync.deleted} doc(s)`);
      if (sync.stale > 0) parts.push(`${sync.stale} stale`);

      if (parts.length > 0) {
        ctx.ui.notify(`Bruno docs synced. ${parts.join(' | ')}`, 'info');
      }
    }
  });

  pi.registerTool({
    name: 'sync_bruno_api_docs',
    label: 'Sync Bruno API Docs',
    description:
      'Sync Bruno API documentation with Go route definitions. ' +
      'Scans routers/*.go for new/deleted/modified endpoints and creates/updates/deletes ' +
      'corresponding Bruno YAML files in docs/api/. Validates all generated files.',
    promptSnippet: 'Synchronize Bruno API docs with Go route definitions',
    promptGuidelines: [
      'Use sync_bruno_api_docs after adding, deleting, or modifying API endpoints to keep Bruno docs in sync.',
      'After sync, fill generated YAML skeletons with request/response details from the controller code.',
    ],
    parameters: { type: 'object', properties: {} },
    async execute(_toolCallId: string, _params: any, _signal: any, _onUpdate: any, _ctx: any) {
      const sync = doSync();

      currentRoutes = sync.newRoutes;
      dirty = false;

      if (sync.errors > 0) {
        const errorText = sync.results
          .filter(r => r.action === 'error')
          .map(e => `- ${e.docPath}: ${e.error}`)
          .join('\n');
        return {
          content: [{ type: 'text', text: `Bruno sync failed with ${sync.errors} error(s):\n${errorText}\n\nFix and retry.` }],
          details: { created: sync.created, deleted: sync.deleted, stale: sync.stale, errors: sync.errors, results: sync.results },
        };
      }

      const docsDir = `${projectRoot}/${DOCS_BASE}`;
      const inventory = scanDocInventory(docsDir, sync.newRoutes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

      const syncParts: string[] = [];
      if (sync.created > 0) syncParts.push(`created ${sync.created}`);
      if (sync.deleted > 0) syncParts.push(`deleted ${sync.deleted}`);
      if (sync.stale > 0) syncParts.push(`${sync.stale} stale`);
      const syncSummary = syncParts.length > 0
        ? `Bruno docs synced: ${syncParts.join(', ')}.`
        : 'Bruno docs synced: no route changes.';

      const report = formatInventoryReport(inventory, syncSummary);

      return {
        content: [{ type: 'text', text: report }],
        details: {
          created: sync.created,
          deleted: sync.deleted,
          stale: sync.stale,
          errors: 0,
          results: sync.results,
          inventory: {
            managed: inventory.managed.length,
            external: inventory.external.length,
            unrecognized: inventory.unrecognized.length,
          },
        },
      };
    },
  });

  // --- Slash command ---

  pi.registerCommand('bruno', {
    description:
      'Sync Bruno API docs with Go route definitions. ' +
      'Scans routers, compares with docs, generates missing docs, ' +
      'deletes stale ones, and validates all YAML against official schema.',
    handler: async (_args: string, ctx: any) => {
      const sync = doSync();

      currentRoutes = sync.newRoutes;
      dirty = false;

      if (sync.errors > 0) {
        const detail = sync.results
          .filter(r => r.action === 'error')
          .map(e => `  ${e.docPath}: ${e.error}`)
          .join('\n');

        ctx.ui.notify(`Bruno sync failed (${sync.errors} error(s))`, 'error');

        pi.sendMessage({
          customType: 'bruno-sync',
          content: [{ type: 'text', text: `Bruno sync failed with ${sync.errors} error(s):\n${detail}\n\nFix the YAML and run /bruno again.` }],
          display: true,
        });
      } else {
        const docsDir = `${projectRoot}/${DOCS_BASE}`;
        const inventory = scanDocInventory(docsDir, sync.newRoutes, API_PREFIX, EXTERNAL_ROUTES, EXTERNAL_DOMAINS);

        const syncParts: string[] = [];
        if (sync.created > 0) syncParts.push(`${sync.created} created`);
        if (sync.deleted > 0) syncParts.push(`${sync.deleted} deleted`);
        if (sync.stale > 0) syncParts.push(`${sync.stale} stale`);
        const syncSummary = syncParts.length > 0
          ? `Bruno docs synced: ${syncParts.join(', ')}.`
          : 'Bruno docs synced: no route changes.';

        const report = formatInventoryReport(inventory, syncSummary);

        // Detailed change lines
        const changeLines = sync.results
          .filter(r => r.action === 'created' || r.action === 'deleted' || r.action === 'stale')
          .map(r => `  [${r.action}] ${r.docPath}  (${r.handler})`)
          .join('\n');

        ctx.ui.notify(report, 'info');

        pi.sendMessage({
          customType: 'bruno-sync',
          content: [{ type: 'text', text: changeLines ? `${report}\n\nChanges:\n${changeLines}` : report }],
          display: true,
        });
      }
    },
  });
}
