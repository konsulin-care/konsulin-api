/**
 * Doc Inventory — scans and classifies Bruno API documentation files.
 *
 * Walks the docs directory, reads each Bruno YAML file, and cross-references
 * it against parsed Go routes and known third-party patterns to produce a
 * full doc inventory categorized as:
 *
 *   - managed:     matches a Go route definition
 *   - external:    matches a known third-party route or domain
 *   - unrecognized: no match (potential orphan)
 */

import { readdirSync, existsSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import type { ParsedRoute } from './go-parser.ts';

// --- Public types ---

export interface DocInventoryEntry {
  /** Relative path from docsBaseDir, e.g. "auth/magiclink.yml" */
  relPath: string;
  /** Display name from info.name in the YAML */
  name: string;
  /** Classification category */
  category: 'managed' | 'external' | 'unrecognized';
  /** The Go route key if matched, e.g. "POST /magiclink" */
  routeKey?: string;
}

export interface DocInventory {
  managed: DocInventoryEntry[];
  external: DocInventoryEntry[];
  unrecognized: DocInventoryEntry[];
  total: number;
}

// --- External doc registry ---

/** Known third-party route keys (method + path) not defined in Go routers. */
export const EXTERNAL_ROUTES = new Set<string>([
  'POST /auth/signout',
  'POST /auth/signinup/code/consume',
]);

/** Domain directories whose entire contents are third-party (no Go routes). */
export const EXTERNAL_DOMAINS = new Set<string>([
  'misc',
  'fhir',
]);

// --- Internal helpers ---

/**
 * Build a route key from method and path.
 */
function buildRouteKey(method: string, path: string): string {
  return `${method} ${path}`;
}

/**
 * Normalize a URL path segment or route path by replacing template variables
 * with a canonical placeholder.
 *
 * Converts:
 *   {id}        → {param}
 *   {{serviceId}} → {param}
 *   {organizationId} → {param}
 */
function normalizePath(path: string): string {
  return path.replace(/\{\{[^}]+\}\}|\{[^}]+\}/g, '{param}');
}

/**
 * Parse a Bruno doc file to extract the display name, HTTP method, and URL.
 */
function parseDocMeta(
  content: string,
): { name: string; method: string; url?: string } | null {
  const nameMatch = content.match(/^\s*name:\s*"?(.+?)"?\s*$/m);
  if (!nameMatch) return null;

  const methodMatch = content.match(/^\s*method:\s*(\w+)/m);
  if (!methodMatch) return null;

  const urlMatch = content.match(/^\s*url:\s*"(.+?)"/m);
  const url = urlMatch ? urlMatch[1] : undefined;

  return {
    name: nameMatch[1].trim(),
    method: methodMatch[1].toUpperCase(),
    url,
  };
}

/**
 * Derive a relative API path from a Bruno doc URL.
 *
 * Template URLs like '{{process.env.APP_BASE_URL}}/api/v1/auth/magiclink'
 * are parsed to extract the path after the API prefix, yielding '/auth/magiclink'.
 *
 * For URLs without the base variable, returns null.
 */
function derivePathFromUrl(url: string, apiPrefix: string): string | null {
  const baseVar = '{{process.env.APP_BASE_URL}}';
  const idx = url.indexOf(baseVar);
  if (idx < 0) return null;

  let pathPart = url.slice(idx + baseVar.length);

  // Remove query string if present
  const qsIdx = pathPart.indexOf('?');
  if (qsIdx >= 0) {
    pathPart = pathPart.slice(0, qsIdx);
  }

  // Remove the apiPrefix from the path
  if (pathPart.startsWith(apiPrefix)) {
    pathPart = pathPart.slice(apiPrefix.length);
  }

  // Ensure path starts with /
  if (!pathPart.startsWith('/')) {
    pathPart = '/' + pathPart;
  }

  return pathPart;
}

/**
 * Check if a derived Bruno doc path matches a Go route path.
 *
 * Uses suffix matching to handle router nesting prefixes (e.g., `/auth/magiclink`
 * matches the Go route `/magiclink`), and normalizes template variables so that
 * `{id}` and `{{serviceId}}` are treated as equivalent.
 */
function doesRouteMatch(
  docDerivedPath: string,
  goRoutePath: string,
): boolean {
  const normalizedDoc = normalizePath(docDerivedPath);
  const normalizedGo = normalizePath(goRoutePath);
  // Exact match or suffix match (for nested routes like /auth prefix)
  return normalizedDoc === normalizedGo || normalizedDoc.endsWith(normalizedGo);
}

/**
 * Check if a file should be excluded from the inventory.
 * Excludes folder.yml and opencollection.yml.
 */
function isSkippable(filename: string): boolean {
  return filename === 'folder.yml' || filename === 'opencollection.yml';
}

// --- Public API ---

/**
 * Scan the Bruno docs directory and classify every doc.
 *
 * Classification works as follows:
 *   1. Parse each Bruno doc's HTTP method and URL.
 *   2. Derive the relative API path from the URL.
 *   3. Match against parsed Go routes using suffix matching
 *      (to handle nesting like `/auth/magiclink` vs `/magiclink`).
 *   4. If no Go route match, check known EXTERNAL_ROUTES and EXTERNAL_DOMAINS.
 *   5. Otherwise, "unrecognized".
 *
 * @param docsBaseDir - Path to the docs/api/ directory
 * @param parsedRoutes - Parsed Go route definitions
 * @param apiPrefix - API prefix (e.g., /api/v1)
 * @param externalRoutes - Set of known third-party route keys
 * @param externalDomains - Set of external domain directory names
 * @returns Full doc inventory with categorization
 */
export function scanDocInventory(
  docsBaseDir: string,
  parsedRoutes: ParsedRoute[],
  apiPrefix: string = '/api/v1',
  externalRoutes: Set<string> = EXTERNAL_ROUTES,
  externalDomains: Set<string> = EXTERNAL_DOMAINS,
): DocInventory {
  const inventory: DocInventory = {
    managed: [],
    external: [],
    unrecognized: [],
    total: 0,
  };

  if (!existsSync(docsBaseDir)) return inventory;

  // Walk docs directories
  const entries = readdirSync(docsBaseDir, { withFileTypes: true });

  for (const entry of entries) {
    if (!entry.isDirectory()) continue;

    const domainDir = join(docsBaseDir, entry.name);
    const domainName = entry.name;
    const domainFiles = readdirSync(domainDir).filter(
      (f: string) => f.endsWith('.yml') && !isSkippable(f),
    );

    for (const file of domainFiles) {
      const filePath = join(domainDir, file);
      if (!statSync(filePath).isFile()) continue;

      const content = readFileSync(filePath, 'utf-8');
      const meta = parseDocMeta(content);
      const name = meta?.name ?? file;
      const relPath = `${domainName}/${file}`;

      let category: 'managed' | 'external' | 'unrecognized' = 'unrecognized';
      let routeKey: string | undefined;

      if (meta && meta.url) {
        const derivedPath = derivePathFromUrl(meta.url, apiPrefix);

        if (derivedPath) {
          const key = buildRouteKey(meta.method, derivedPath);

          // Check against managed Go routes
          const matchedRoute = parsedRoutes.find(
            r => meta!.method === r.method && doesRouteMatch(derivedPath!, r.path),
          );

          if (matchedRoute) {
            category = 'managed';
            routeKey = buildRouteKey(matchedRoute.method, matchedRoute.path);
          } else if (externalRoutes.has(key)) {
            category = 'external';
            routeKey = key;
          }
        }
      }

      // Fallback: check external domain (even if URL couldn't be parsed)
      if (category === 'unrecognized' && externalDomains.has(domainName)) {
        category = 'external';
      }

      inventory[category].push({ relPath, name, category, routeKey });
      inventory.total++;
    }
  }

  return inventory;
}

/**
 * Format a doc inventory report for display.
 *
 * @param inventory - The doc inventory to format
 * @param syncSummary - The sync summary line (e.g. "Bruno docs synced: no route changes.")
 * @returns A multi-line report string
 */
export function formatInventoryReport(
  inventory: DocInventory,
  syncSummary: string,
): string {
  const lines: string[] = [];

  // Sync summary
  lines.push(syncSummary);

  // Inventory line: counts
  const countParts: string[] = [`${inventory.managed.length} managed`];
  countParts.push(`${inventory.external.length} external`);
  if (inventory.unrecognized.length > 0) {
    countParts.push(`${inventory.unrecognized.length} unrecognized`);
  }
  lines.push(`Doc inventory: ${countParts.join(', ')}`);

  // List external docs grouped by domain
  if (inventory.external.length > 0) {
    const byDomain = new Map<string, string[]>();
    for (const ext of inventory.external) {
      const domain = ext.relPath.split('/')[0];
      if (!byDomain.has(domain)) byDomain.set(domain, []);
      byDomain.get(domain)!.push(ext.relPath);
    }
    for (const [domain, paths] of byDomain) {
      lines.push(`  External: ${paths.join(', ')}`);
    }
  }

  // Warn about unrecognized docs
  if (inventory.unrecognized.length > 0) {
    for (const unrec of inventory.unrecognized) {
      lines.push(`⚠ Unrecognized: ${unrec.relPath} — no matching Go route or external pattern`);
    }
  }

  return lines.join('\n');
}
