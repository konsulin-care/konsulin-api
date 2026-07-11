/**
 * Bruno API Guardrail Extension
 *
 * Auto-syncs Bruno API docs with Go chi route definitions.
 * Hooks tool_call to track file changes, turn_end to auto-sync,
 * and registers sync_bruno_api_docs for explicit invocation.
 *
 * Lifecycle:
 *   session_start → parse routes → baseline
 *   tool_call (write/edit/bash touching routers/*.go) → dirty flag
 *   turn_end → runSync → validate → auto-inject on failure
 */

import { type ParsedRoute } from './go-parser.ts';
import { shouldTrackFile } from './state-tracker.ts';
import { runSync, scanAllRouterFiles } from './sync-orchestrator.ts';

const DOCS_BASE = 'docs/api';
const API_PREFIX = '/api/v1';

export { runSync, scanAllRouterFiles } from './sync-orchestrator.ts';
export type { SyncResult, SyncError } from './sync-orchestrator.ts';

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

  pi.on('turn_end', async (_event: any, ctx: any) => {
    if (!dirty) return;

    const newRoutes = scanAllRouterFiles(projectRoot);
    const results = runSync(currentRoutes, newRoutes, `${projectRoot}/${DOCS_BASE}`, API_PREFIX);

    const errors = results.filter(r => r.action === 'error');
    const stale = results.filter(r => r.action === 'stale');

    if (errors.length > 0) {
      const errorSummary = errors
        .map(e => `  File: ${e.docPath}\n  Error: ${e.error}`)
        .join('\n');

      pi.sendMessage({
        customType: 'bruno-sync',
        content: [
          { type: 'text', text: `BRUNO SYNC FAILED\n\n${errors.length} validation error(s):\n${errorSummary}\n\nFix the YAML and run sync-bruno-api-docs again.` },
        ],
        display: true,
      });
    } else {
      currentRoutes = newRoutes;
      dirty = false;

      const parts: string[] = [];
      const created = results.filter(r => r.action === 'created');
      const deleted = results.filter(r => r.action === 'deleted');
      if (created.length > 0) parts.push(`Created ${created.length} doc(s)`);
      if (deleted.length > 0) parts.push(`Deleted ${deleted.length} doc(s)`);
      if (stale.length > 0) parts.push(`${stale.length} stale`);

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
      const newRoutes = scanAllRouterFiles(projectRoot);
      const results = runSync(currentRoutes, newRoutes, `${projectRoot}/${DOCS_BASE}`, API_PREFIX);

      const errors = results.filter(r => r.action === 'error');
      const created = results.filter(r => r.action === 'created');
      const deleted = results.filter(r => r.action === 'deleted');
      const stale = results.filter(r => r.action === 'stale');

      currentRoutes = newRoutes;
      dirty = false;

      if (errors.length > 0) {
        const errorText = errors.map(e => `- ${e.docPath}: ${e.error}`).join('\n');
        return {
          content: [{ type: 'text', text: `Bruno sync failed with ${errors.length} error(s):\n${errorText}\n\nFix and retry.` }],
          details: { created: created.length, deleted: deleted.length, stale: stale.length, errors: errors.length, results },
        };
      }

      const parts: string[] = [];
      if (created.length > 0) parts.push(`created ${created.length}`);
      if (deleted.length > 0) parts.push(`deleted ${deleted.length}`);
      if (stale.length > 0) parts.push(`${stale.length} stale`);
      const summary = parts.length > 0 ? `Bruno docs synced: ${parts.join(', ')}.` : 'Bruno docs are up to date.';

      return {
        content: [{ type: 'text', text: summary }],
        details: { created: created.length, deleted: deleted.length, stale: stale.length, errors: 0, results },
      };
    },
  });
}
