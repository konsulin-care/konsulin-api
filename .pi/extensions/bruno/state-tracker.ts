/**
 * Tracks the current state of Go API routes and computes diffs
 * between route snapshots. Used by the extension to detect
 * new, deleted, modified, and stale routes.
 */

import { type ParsedRoute } from './go-parser.ts';

/** A snapshot of all known routes at a point in time. */
export type RouteState = Map<string, ParsedRoute>;

/** Describes changes between two route snapshots. */
export interface RouteDiff {
  /** Routes that exist in new but not in old. */
  new: ParsedRoute[];
  /** Routes that exist in old but not in new. */
  deleted: ParsedRoute[];
  /** Routes where the method or path changed (same handler, different key). */
  modified: { key: string; from: ParsedRoute; to: ParsedRoute }[];
  /** Routes where the handler function changed (same key, different handler). */
  stale: { key: string; from: ParsedRoute; to: ParsedRoute }[];
}

/**
 * Build a composite key for a route: "METHOD /path".
 * Used for diffing and lookup.
 */
export function buildRouteKey(route: ParsedRoute): string {
  return `${route.method} ${route.path}`;
}

/**
 * Check if a file path should trigger a Bruno sync.
 * Only routers/*.go and controllers/*.go in the delivery/http directory.
 */
export function shouldTrackFile(filePath: string): boolean {
  const normalized = filePath.replace(/\\/g, '/');

  // Never track Bruno doc files (avoid infinite loop)
  if (normalized.endsWith('.yml') || normalized.endsWith('.yaml')) {
    return false;
  }

  // Only track Go files
  if (!normalized.endsWith('.go')) {
    return false;
  }

  // Must be in routers/ or controllers/ directories
  if (normalized.includes('/routers/') && normalized.endsWith('.go')) {
    return true;
  }
  if (normalized.includes('/controllers/') && normalized.endsWith('.go')) {
    return true;
  }

  // For bare filenames (tests), check the pattern directly
  const basename = normalized.split('/').pop() ?? '';
  if (basename.endsWith('_router.go') || basename.endsWith('_controller.go')) {
    return true;
  }

  return false;
}

/**
 * Build a RouteState (key → ParsedRoute) from a list of routes.
 */
export function buildState(routes: ParsedRoute[]): RouteState {
  const state: RouteState = new Map();
  for (const route of routes) {
    state.set(buildRouteKey(route), route);
  }
  return state;
}

/**
 * Compute the diff between two route snapshots.
 */
export function computeDiff(
  oldRoutes: ParsedRoute[],
  newRoutes: ParsedRoute[],
): RouteDiff {
  const oldState = buildState(oldRoutes);
  const newState = buildState(newRoutes);

  const diff: RouteDiff = {
    new: [],
    deleted: [],
    modified: [],
    stale: [],
  };

  // Find new and modified routes
  for (const [key, newRoute] of newState) {
    const oldRoute = oldState.get(key);
    if (!oldRoute) {
      diff.new.push(newRoute);
    } else if (oldRoute.handler !== newRoute.handler) {
      // Same method+path but different handler → stale
      diff.stale.push({ key, from: oldRoute, to: newRoute });
    } else if (
      oldRoute.method !== newRoute.method ||
      oldRoute.path !== newRoute.path
    ) {
      diff.modified.push({ key, from: oldRoute, to: newRoute });
    }
  }

  // Find deleted routes
  for (const [key, oldRoute] of oldState) {
    if (!newState.has(key)) {
      diff.deleted.push(oldRoute);
    }
  }

  return diff;
}
