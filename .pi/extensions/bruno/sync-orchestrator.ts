/**
 * Sync orchestrator — core logic for syncing Bruno docs with Go route definitions.
 *
 * Separated from index.ts to keep both files under 300 lines.
 */

import { type ParsedRoute, parseGoRoutes } from './go-parser.ts';
import { computeDiff } from './state-tracker.ts';
import { scanDocInventory, EXTERNAL_ROUTES, EXTERNAL_DOMAINS, formatInventoryReport } from './doc-inventory.ts';
import type { DocInventory, DocInventoryEntry } from './doc-inventory.ts';
import {
  generateBrunoSkeleton,
  buildBrunoPath,
  findNextSeq,
  deleteBrunoDoc,
  type ChainInfo,
} from './bruno-generator.ts';
import { validateBrunoYaml } from './bruno-validator.ts';
import { readFileSync, writeFileSync, existsSync, mkdirSync, readdirSync } from 'node:fs';
import { dirname } from 'node:path';

// Re-export doc-inventory and bruno-generator symbols for use by index.ts
export { scanDocInventory, EXTERNAL_ROUTES, EXTERNAL_DOMAINS, formatInventoryReport };
export type { DocInventory, DocInventoryEntry };
export type { ChainInfo };

// --- Public types ---

export interface SyncResult {
  action: 'created' | 'deleted' | 'stale' | 'unchanged' | 'error';
  docPath?: string;
  handler?: string;
  key?: string;
  success: boolean;
  error?: string;
}

const HTTP_ROUTER_DIR = 'internal/app/delivery/http/routers';

/**
 * Scan all Go router files from the standard router directory.
 */
export function scanAllRouterFiles(projectRoot: string): ParsedRoute[] {
  const routerDir = `${projectRoot}/${HTTP_ROUTER_DIR}`;
  if (!existsSync(routerDir)) return [];

  const files = readdirSync(routerDir).filter(
    (f: string) => f.endsWith('.go') && f.endsWith('_router.go'),
  );

  const allRoutes: ParsedRoute[] = [];
  for (const file of files) {
    const source = readFileSync(`${routerDir}/${file}`, 'utf-8');
    const routes = parseGoRoutes(source, file);
    allRoutes.push(...routes);
  }

  return allRoutes;
}

/**
 * Build a route key string from a route.
 */
export function routeKey(route: ParsedRoute): string {
  return `${route.method} ${route.path}`;
}

/**
 * Run a full sync between old and new route states.
 * Creates, deletes, and flags Bruno docs as needed.
 *
 * @param chainLookup - Optional map from route key to chain info.
 *   When provided, new Bruno docs get chain scripts filled based
 *   on detected dependencies.
 */
export function runSync(
  oldRoutes: ParsedRoute[],
  newRoutes: ParsedRoute[],
  docsBaseDir: string,
  apiPrefix: string,
  chainLookup?: Map<string, ChainInfo>,
): SyncResult[] {
  const diff = computeDiff(oldRoutes, newRoutes);
  const results: SyncResult[] = [];

  for (const route of diff.new) {
    const docPath = buildBrunoPath(route, docsBaseDir);
    const domainDir = dirname(docPath);

    if (route.path.startsWith('/fhir') || route.path.startsWith('/tx')) {
      continue;
    }

    if (!existsSync(domainDir)) {
      mkdirSync(domainDir, { recursive: true });
    }

    const seq = findNextSeq(domainDir);
    const chainInfo = chainLookup?.get(routeKey(route));
    const yaml = generateBrunoSkeleton(route, seq, apiPrefix, chainInfo);

    try {
      writeFileSync(docPath, yaml, 'utf-8');
      const valErrors = validateBrunoYaml(docPath);

      if (valErrors.length > 0) {
        results.push({
          action: 'error',
          docPath,
          handler: route.handler,
          key: `${route.method} ${route.path}`,
          success: false,
          error: `Validation failed: ${valErrors.map(e => e.message).join('; ')}`,
        });
      } else {
        results.push({
          action: 'created',
          docPath,
          handler: route.handler,
          key: `${route.method} ${route.path}`,
          success: true,
        });
      }
    } catch (err) {
      results.push({
        action: 'error',
        docPath,
        handler: route.handler,
        key: `${route.method} ${route.path}`,
        success: false,
        error: String(err),
      });
    }
  }

  for (const route of diff.deleted) {
    const docPath = buildBrunoPath(route, docsBaseDir);

    try {
      const deleted = deleteBrunoDoc(docPath);
      results.push({
        action: 'deleted',
        docPath,
        handler: route.handler,
        key: `${route.method} ${route.path}`,
        success: deleted,
      });
    } catch (err) {
      results.push({
        action: 'error',
        docPath,
        handler: route.handler,
        key: `${route.method} ${route.path}`,
        success: false,
        error: String(err),
      });
    }
  }

  for (const stale of diff.stale) {
    const docPath = buildBrunoPath(stale.to, docsBaseDir);
    results.push({
      action: 'stale',
      docPath,
      handler: stale.to.handler,
      key: stale.key,
      success: true,
    });
  }

  return results;
}
