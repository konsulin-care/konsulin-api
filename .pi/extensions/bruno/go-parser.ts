/**
 * Parses Go chi router source files to extract API endpoint definitions.
 *
 * Handles patterns like:
 *   router.Post("/path", handler)
 *   router.With(middleware).Post("/path", handler)
 *   router.Get("/path/{param}", handler)
 */

export interface ParsedRoute {
  /** HTTP method (POST, GET, PATCH, PUT, DELETE) */
  method: string;
  /** URL path pattern (e.g., /magiclink, /hook/{service}) */
  path: string;
  /** Handler function name (e.g., CreateMagicLink) */
  handler: string;
  /** Controller type name (e.g., AuthController) — derived from function param */
  controller?: string;
  /** Source file name */
  file: string;
  /** Middleware names from .With() chains */
  middlewares: string[];
}

/**
 * Parse a Go source file for chi route registrations.
 *
 * Extracts routes from patterns found in `attach*` functions where
 * controller parameters follow the pattern `*controllers.XxxController`.
 */
export function parseGoRoutes(source: string, filename: string): ParsedRoute[] {
  const routes: ParsedRoute[] = [];

  // Extract controller parameter types from the attach function signature.
  // e.g., "func attachAuthRoutes(... authController *controllers.AuthController)"
  const funcMatch = source.match(/func\s+\w+\([^)]*\)/);
  const controllerVarToType = new Map<string, string>();
  if (funcMatch) {
    const paramRegex = /(\w+)\s+\*controllers\.(\w+)/g;
    let m: RegExpExecArray | null;
    while ((m = paramRegex.exec(funcMatch[0])) !== null) {
      controllerVarToType.set(m[1], m[2]);
    }
  }

  // Match route registrations:
  //   router.With("middleware").Method("/path", handler)
  //   router.Method("/path", handler)
  // Captures: [full, routerVar, middlewareArg?, method, quotedPath, handlerExpr]
  const routeRegex = /(\w+)\.(?:With\(([^)]+)\)\.)?(Post|Get|Patch|Put|Delete)\(\s*("[^"]+")\s*,\s*(\w+(?:\.\w+)*)\s*\)/g;

  let match: RegExpExecArray | null;
  while ((match = routeRegex.exec(source)) !== null) {
    const [, , middlewareArg, method, quotedPath, handlerExpr] = match;

    // Separate handler variable and function name: "authController.CreateMagicLink"
    const dotIdx = handlerExpr.lastIndexOf('.');
    const handlerName = dotIdx >= 0 ? handlerExpr.slice(dotIdx + 1) : handlerExpr;
    const controllerVar = dotIdx >= 0 ? handlerExpr.slice(0, dotIdx) : undefined;

    routes.push({
      method: method.toUpperCase(),
      path: quotedPath.slice(1, -1), // strip surrounding quotes
      handler: handlerName,
      controller: controllerVar ? controllerVarToType.get(controllerVar) : undefined,
      file: filename,
      middlewares: middlewareArg ? [middlewareArg] : [],
    });
  }

  return routes;
}

/**
 * Derive the domain name from a router filename.
 * "auth_router.go" → "auth"
 * "payment_router.go" → "payments"
 */
export function routeFileToDomain(filename: string): string {
  const name = filename.replace(/_router\.go$/, '').replace(/\.go$/, '');
  // Map common singular router names to their Bruno domain folder names
  const domainMap: Record<string, string> = {
    auth: 'auth',
    payment: 'payments',
    webhook: 'webhooks',
    schedule: 'schedules',
    organization: 'organizations',
  };
  return domainMap[name] ?? name;
}

/**
 * Build the Bruno YAML filename from a route.
 * POST /magiclink → magiclink.yml
 * GET /passwordless/email/exists → passwordless-email-exists.yml
 */
export function routeToDocFilename(route: ParsedRoute): string {
  // Take the last path segment or the full path if short
  const segments = route.path.replace(/[{}]/g, '').split('/').filter(Boolean);
  const namePart = segments.join('-');
  return `${namePart}.yml`;
}

/**
 * Derive the controller file path from a route's router file.
 * auth_router.go → controllers/auth_controller.go
 * payment_router.go → controllers/payment_controller.go
 */
export function routeToControllerFile(route: ParsedRoute): string {
  const name = route.file
    .replace(/_router\.go$/, '')
    .replace(/\.go$/, '');
  return `controllers/${name}_controller.go`;
}

/**
 * Build a human-readable display name from a handler function.
 * CreateMagicLink → "Create Magic Link"
 * HandleSynchronousWebHook → "Handle Synchronous Web Hook"
 */
export function handlerToDisplayName(handler: string): string {
  // Insert space before uppercase letters, then trim and title-case
  return handler
    .replace(/([A-Z])/g, ' $1')
    .trim()
    .replace(/^[a-z]/, c => c.toUpperCase());
}
