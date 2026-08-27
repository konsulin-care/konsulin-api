/**
 * Generates, updates, and deletes Bruno API documentation YAML files
 * following the OpenCollection v1.0.0 spec.
 *
 * The generated YAML is a skeleton — the LLM fills in the docs,
 * examples, assertions, headers, and chain scripts after reading
 * the Go controller source.
 */

import { type ParsedRoute } from './go-parser.ts';
import { readFileSync, readdirSync, existsSync, unlinkSync } from 'node:fs';
import { join } from 'node:path';

/**
 * A parsed Bruno doc entry for sequence number discovery.
 */
export interface BrunoDocEntry {
  /** The doc filename (e.g., magiclink.yml) */
  file: string;
  /** The sequence number from info.seq */
  seq: number;
}

/**
 * Find the next available sequence number in a Bruno domain folder.
 * Scans all .yml files for info.seq values and returns max + 1.
 */
export function findNextSeq(domainDir: string): number {
  if (!existsSync(domainDir)) return 1;

  let maxSeq = 0;
  const files = readdirSync(domainDir).filter(f => f.endsWith('.yml'));

  for (const file of files) {
    try {
      const content = readFileSync(join(domainDir, file), 'utf-8');
      const match = content.match(/^info:\s*\n(?:.*\n)*?\s+seq:\s*(\d+)/m);
      if (match) {
        const seq = parseInt(match[1], 10);
        if (seq > maxSeq) maxSeq = seq;
      }
    } catch {
      // Skip unreadable files
    }
  }

  return maxSeq + 1;
}

/**
 * Map a router filename to its Bruno domain folder name.
 * "auth_router.go" → "auth"
 * "payment_router.go" → "payments"
 */
function routerFileToDomain(filename: string): string {
  const base = filename.replace(/_router\.go$/, '').replace(/\.go$/, '');
  const domainMap: Record<string, string> = {
    auth: 'auth',
    payment: 'payments',
    webhook: 'webhooks',
    schedule: 'schedules',
    organization: 'organizations',
  };
  return domainMap[base] ?? base;
}

/**
 * Convert a URL path segment to a Bruno doc filename segment.
 * /magiclink → magiclink
 * /passwordless/email/exists → passwordless-email-exists
 */
function pathToFilename(path: string): string {
  return path
    .replace(/[{}]/g, '')
    .split('/')
    .filter(Boolean)
    .join('-');
}

/**
 * Build the full path to a Bruno doc file for a given route.
 */
export function buildBrunoPath(route: ParsedRoute, docsBaseDir: string): string {
  const domain = routerFileToDomain(route.file);
  const filename = pathToFilename(route.path);
  return join(docsBaseDir, domain, `${filename}.yml`);
}

/**
 * Chain information for filling in the Bruno runtime chain script.
 */
export interface ChainInfo {
  /** User-confirmed name of the next request in the chain. */
  nextRequestName?: string;
  /** Detected downstream dependencies (not yet confirmed). */
  downstreamDeps?: string[];
}

/**
 * Generate the runtime after-response script code based on chain info.
 */
function buildChainScript(chainInfo?: ChainInfo): string {
  if (chainInfo?.nextRequestName) {
    return `        bru.runner.setNextRequest("${chainInfo.nextRequestName}");`;
  }

  if (chainInfo?.downstreamDeps && chainInfo.downstreamDeps.length > 0) {
    const depLines = chainInfo.downstreamDeps
      .map(dep => `        # Detected downstream: ${dep}`)
      .join('\n');
    return `${depLines}\n        # TODO: confirm chain order, then enable:
        # bru.runner.setNextRequest("Next Request Name");`;
  }

  return `        # TODO: set chain based on workflow
        # bru.runner.setNextRequest("Next Request Name");`;
}

/**
 * Generate an OpenCollection-compliant YAML skeleton for a new route.
 *
 * The skeleton is minimal but spec-valid. The LLM is expected to:
 * 1. Read the Go controller implementation
 * 2. Fill headers, body schema, assertions, examples, docs, and chain scripts
 * 3. Call sync-bruno-api-docs again to validate
 *
 * @param chainInfo - Optional chain information. If `nextRequestName` is set,
 *   writes a confirmed chain script. If only `downstreamDeps` is set, writes
 *   detected dependencies as context for the user. Omit for generic TODO.
 */
export function generateBrunoSkeleton(
  route: ParsedRoute,
  seq: number,
  apiPrefix: string,
  chainInfo?: ChainInfo,
): string {
  const displayName = handlerToDisplayName(route.handler);
  const fullUrl = `{{process.env.APP_BASE_URL}}${apiPrefix}${route.path}`;
  const chainCode = buildChainScript(chainInfo);

  return `info:
  name: "${displayName}"
  type: http
  seq: ${seq}
  tags: []

http:
  method: ${route.method}
  url: "${fullUrl}"
  headers:
    - name: content-type
      value: application/json
  body:
    type: json
    data: "{}"

runtime:
  scripts:
    - type: after-response
      code: |-
${chainCode}
  assertions: []

settings:
  encodeUrl: true
  timeout: 0
  followRedirects: true
  maxRedirects: 5

examples: []

docs: |
  TODO: Document this endpoint
`;
}

/**
 * Delete a Bruno doc file. Returns true if deleted, false if not found.
 */
export function deleteBrunoDoc(docPath: string): boolean {
  if (!existsSync(docPath)) return false;
  unlinkSync(docPath);
  return true;
}

/**
 * Convert a Go-style CamelCase handler to a display name.
 * CreateMagicLink → Create Magic Link
 */
function handlerToDisplayName(handler: string): string {
  return handler
    .replace(/([A-Z])/g, ' $1')
    .trim()
    .replace(/^[a-z]/, c => c.toUpperCase());
}
