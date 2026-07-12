/**
 * Chain Analyzer — scans Go controller source files to infer
 * endpoint chain relationships with confidence scores.
 */

import { type ParsedRoute, routeToControllerFile } from './go-parser.ts';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

export interface DetectedChain {
  fromKey: string;
  toKey: string;
  type: 'data-flow' | 'http-call' | 'domain-sequence' | 'async' | 'fhir-proxy';
  confidence: number;
  evidence: string[];
}

export interface ChainDiagnostic {
  endpoint: ParsedRoute;
  controllerFile: string;
  usecaseCalls: string[];
  httpClientCalls: string[];
  asyncPublishes: string[];
  responseFields: string[];
  requestFields: string[];
  detectedChains: DetectedChain[];
}

/** Extract the body of a Go handler function by name. */
export function extractHandlerSource(source: string, handlerName: string): string | null {
  const funcRegex = new RegExp(
    `func\\s+\\(\\w+\\s+\\*?\\w+\\)\\s+${escapeRegex(handlerName)}\\s*\\([^)]*\\)\\s*\\{`,
  );
  const match = funcRegex.exec(source);
  if (!match) return null;

  const start = match.index + match[0].length;
  let depth = 1;
  let pos = start;
  while (pos < source.length && depth > 0) {
    const ch = source[pos];
    if (ch === '{') depth++;
    else if (ch === '}') depth--;
    pos++;
  }
  return source.slice(start, pos - 1).trim();
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/** Extract usecase method calls: ctrl.AuthUsecase.CreateMagicLink(...) */
export function extractUsecaseCalls(source: string): string[] {
  const regex = /ctrl\.(\w+)\.(\w+)\s*\(/g;
  const calls: string[] = [];
  let match: RegExpExecArray | null;
  while ((match = regex.exec(source)) !== null) {
    calls.push(`${match[1]}.${match[2]}`);
  }
  return [...new Set(calls)];
}

/** Extract response body fields from map literals in BuildSuccessResponse or json.NewEncoder. */
export function extractResponseFields(source: string): string[] {
  const fields: string[] = [];
  const mapRegex = /map\[string\]interface\{\}\s*\{([^}]*)\}/g;
  let match: RegExpExecArray | null;
  while ((match = mapRegex.exec(source)) !== null) {
    const keyRegex = /"([^"]+)"\s*:/g;
    let keyMatch: RegExpExecArray | null;
    while ((keyMatch = keyRegex.exec(match[1])) !== null) {
      fields.push(keyMatch[1]);
    }
  }
  return [...new Set(fields)];
}

/** Extract HTTP client calls: Client.Do, http.NewRequest, http.Get/Post/etc. */
export function extractHttpClientCalls(source: string): string[] {
  const calls: string[] = [];
  const patterns = [
    /\w+Client\.Do\s*\(/g, /http\.NewRequest\s*\(/g,
    /http\.(Get|Post|Put|Delete|Patch)\s*\(/g,
  ];
  for (const regex of patterns) {
    let match: RegExpExecArray | null;
    while ((match = regex.exec(source)) !== null) {
      calls.push(match[0].replace(/\s*\($/, ''));
    }
  }
  return [...new Set(calls)];
}

/** Extract async publish calls: .Publish(), .PublishAsync() */
export function extractAsyncPublishes(source: string): string[] {
  const calls: string[] = [];
  let match: RegExpExecArray | null;
  const regex = /\.(Publish|PublishAsync)\s*\(/g;
  while ((match = regex.exec(source)) !== null) {
    calls.push(match[0].replace(/\s*\($/, ''));
  }
  return [...new Set(calls)];
}

/** Infer chain type from usecase call patterns. */
function detectChainType(from: ChainDiagnostic, to: ChainDiagnostic): { type: DetectedChain['type']; confidence: number; evidence: string[] } | null {
  const fromKey = `${from.endpoint.method} ${from.endpoint.path}`;
  const toKey = `${to.endpoint.method} ${to.endpoint.path}`;
  const fromAsync = from.usecaseCalls.some(c => /enqueue|publish|async/i.test(c));
  const toAsync = to.usecaseCalls.some(c => /callback|async|result|get/i.test(c));

  if (fromAsync && toAsync) {
    return {
      type: 'async', confidence: 0.95,
      evidence: [`'${from.usecaseCalls.filter(c => /enqueue|publish|async/i.test(c)).join(', ')}' → '${to.usecaseCalls.filter(c => /callback|async|result|get/i.test(c)).join(', ')}'`],
    };
  }

  const tokenResponse = from.responseFields.some(f => /token/i.test(f));
  const toHasAuth = to.usecaseCalls.some(c => /auth|role|claim/i.test(c)) ||
    to.endpoint.path.toLowerCase().includes('auth');
  if (tokenResponse && toHasAuth && from.endpoint.file === to.endpoint.file) {
    return { type: 'data-flow', confidence: 0.85, evidence: [`response_field 'token' flows to '${toKey}' (requires Authorization)`] };
  }

  if (from.endpoint.file === to.endpoint.file) {
    return { type: 'domain-sequence', confidence: 0.40, evidence: [`Same domain (${from.endpoint.file})`] };
  }
  return null;
}

/** Scan a controller file and extract all diagnostic data for a route. */
function buildEndpointDiagnostic(route: ParsedRoute, projectRoot: string): ChainDiagnostic {
  const controllerFile = routeToControllerFile(route);
  const controllerPath = join(projectRoot, controllerFile);
  let usecaseCalls: string[] = [];
  let httpClientCalls: string[] = [];
  let asyncPublishes: string[] = [];
  let responseFields: string[] = [];

  if (existsSync(controllerPath)) {
    const source = readFileSync(controllerPath, 'utf-8');
    const handlerBody = extractHandlerSource(source, route.handler);
    if (handlerBody) {
      usecaseCalls = extractUsecaseCalls(handlerBody);
      httpClientCalls = extractHttpClientCalls(handlerBody);
      asyncPublishes = extractAsyncPublishes(handlerBody);
      responseFields = extractResponseFields(handlerBody);
    }
  }

  return { endpoint: route, controllerFile, usecaseCalls, httpClientCalls, asyncPublishes, responseFields, requestFields: [], detectedChains: [] };
}

/** Analyze all routes and build chain diagnostics with confidence scores. */
export function analyzeChains(routes: ParsedRoute[], projectRoot: string): ChainDiagnostic[] {
  if (routes.length === 0) return [];
  const diags = routes.map(route => buildEndpointDiagnostic(route, projectRoot));

  const chains: DetectedChain[] = [];
  for (let i = 0; i < diags.length; i++) {
    const fromKey = `${diags[i].endpoint.method} ${diags[i].endpoint.path}`;
    for (let j = 0; j < diags.length; j++) {
      if (i === j) continue;
      const toKey = `${diags[j].endpoint.method} ${diags[j].endpoint.path}`;
      const result = detectChainType(diags[i], diags[j]);
      if (result) chains.push({ ...result, fromKey, toKey });
    }
    if (diags[i].endpoint.path.startsWith('/fhir') || diags[i].endpoint.path.startsWith('/tx')) {
      chains.push({ fromKey, toKey: 'fhir: Blaze server', type: 'fhir-proxy', confidence: 0.95, evidence: ['Proxied to FHIR Blaze — no local logic'] });
    }
  }

  for (const diag of diags) {
    const key = `${diag.endpoint.method} ${diag.endpoint.path}`;
    diag.detectedChains = chains.filter(c => c.fromKey === key);
  }
  return diags;
}

/** Format chain diagnostics into a human-readable report. */
export function formatChainReport(diags: ChainDiagnostic[]): string {
  if (diags.length === 0) return 'Chain Analysis:\n  (no routes)';

  const lines: string[] = ['Chain Analysis:'];
  const byDomain = new Map<string, ChainDiagnostic[]>();
  for (const d of diags) {
    const domain = d.endpoint.file.replace(/_router\.go$/, '');
    if (!byDomain.has(domain)) byDomain.set(domain, []);
    byDomain.get(domain)!.push(d);
  }

  for (const [domain, domainDiags] of byDomain) {
    lines.push(`  ${domain}/`);
    for (const d of domainDiags) {
      const key = `${d.endpoint.method} ${d.endpoint.path}`;
      lines.push(`    ${key}`);
      if (d.usecaseCalls.length > 0) lines.push(`      usecase: ${d.usecaseCalls.join(', ')}`);
      if (d.responseFields.length > 0) lines.push(`      returns: ${d.responseFields.join(', ')}`);
      if (d.detectedChains.length === 0) {
        lines.push('      (terminal — no downstream detected)');
      } else for (const chain of d.detectedChains) {
        const flag = chain.confidence < 0.5 ? ' ⚠ low confidence' : '';
        lines.push(`      → ${chain.toKey}  [${chain.confidence.toFixed(2)} — ${chain.type}]${flag}`);
        for (const ev of chain.evidence) lines.push(`        ${ev}`);
      }
    }
  }
  return lines.join('\n');
}
