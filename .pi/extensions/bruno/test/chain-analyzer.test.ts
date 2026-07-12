/**
 * Tests for chain-analyzer.ts — controller code analysis
 * for inferring endpoint chains with confidence scores.
 */

import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { type ParsedRoute } from '../go-parser.ts';
import {
  extractHandlerSource,
  extractUsecaseCalls,
  extractResponseFields,
  extractHttpClientCalls,
  extractAsyncPublishes,
  analyzeChains,
  type ChainDiagnostic,
  type DetectedChain,
} from '../chain-analyzer.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const CONTROLLER_FIXTURES = join(__dirname, 'fixtures/controllers');
const FIXTURES_ROOT = join(__dirname, 'fixtures');

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

function loadController(name: string): string {
  return readFileSync(join(CONTROLLER_FIXTURES, name), 'utf-8');
}

await describe('chain-analyzer', async () => {
  await describe('extractHandlerSource', async () => {
    await it('extracts a handler function by name from controller source', () => {
      const source = loadController('auth_controller.go');
      const body = extractHandlerSource(source, 'CreateMagicLink');
      assert.ok(body, 'expected a non-null result');
      assert.ok(body.includes('ctrl.AuthUsecase.CheckUserExists'));
      assert.ok(body.includes('utils.BuildSuccessResponse'));
    });

    await it('returns null for non-existent handler', () => {
      const source = loadController('auth_controller.go');
      const body = extractHandlerSource(source, 'NonExistentHandler');
      assert.equal(body, null);
    });

    await it('extracts a specific handler, not all handlers', () => {
      const source = loadController('auth_controller.go');
      const body = extractHandlerSource(source, 'CreateAnonymousSession');
      assert.ok(body, 'expected a non-null result');
      assert.ok(body.includes('CreateAnonymousSession'));
      assert.ok(!body.includes('CreateMagicLink'));
    });
  });

  await describe('extractUsecaseCalls', async () => {
    await it('extracts usecase calls from handler source', () => {
      const handlerSrc = `
        userExistsOutput, err := ctrl.AuthUsecase.CheckUserExists(ctx, email)
        err = ctrl.AuthUsecase.CreateMagicLink(ctx, request)
        ctrl.AuthUsecase.DoSomething()
      `;
      const calls = extractUsecaseCalls(handlerSrc);
      assert.equal(calls.length, 3);
      assert.ok(calls.includes('AuthUsecase.CheckUserExists'));
      assert.ok(calls.includes('AuthUsecase.CreateMagicLink'));
      assert.ok(calls.includes('AuthUsecase.DoSomething'));
    });

    await it('returns empty array when no usecase calls exist', () => {
      const handlerSrc = `utils.BuildErrorResponse(ctrl.Log, w, err)`;
      const calls = extractUsecaseCalls(handlerSrc);
      assert.deepEqual(calls, []);
    });

    await it('extracts usecase calls from real auth controller handlers', () => {
      const source = loadController('auth_controller.go');
      const body = extractHandlerSource(source, 'CreateMagicLink')!;
      const calls = extractUsecaseCalls(body);
      assert.ok(calls.includes('AuthUsecase.CheckUserExists'));
      assert.ok(calls.includes('AuthUsecase.CreateMagicLink'));
    });

    await it('extracts usecase calls from real webhook controller handlers', () => {
      const source = loadController('webhook_controller.go');
      const body = extractHandlerSource(source, 'HandleEnqueueWebHook')!;
      const calls = extractUsecaseCalls(body);
      assert.ok(calls.includes('Usecase.Enqueue'));
      assert.equal(calls.length, 1);
    });

    await it('extracts multiple usecase methods from async handlers', () => {
      const source = loadController('webhook_controller.go');
      const body = extractHandlerSource(source, 'HandleAsyncServiceResultCallback')!;
      const calls = extractUsecaseCalls(body);
      assert.ok(calls.includes('Usecase.HandleAsyncServiceResult'));
    });
  });

  await describe('extractResponseFields', async () => {
    await it('extracts fields from BuildSuccessResponse map literal', () => {
      const handlerSrc = `
        utils.BuildSuccessResponse(w, constvars.StatusOK, "ok", map[string]interface{}{
          "token":    result.Token,
          "guest_id": result.GuestID,
          "is_new":   result.IsNew,
          "role":     "guest",
        })
      `;
      const fields = extractResponseFields(handlerSrc);
      assert.equal(fields.length, 4);
      assert.ok(fields.includes('token'));
      assert.ok(fields.includes('guest_id'));
      assert.ok(fields.includes('is_new'));
      assert.ok(fields.includes('role'));
    });

    await it('extracts fields from json.NewEncoder(w).Encode(response)', () => {
      const handlerSrc = `
        response := map[string]interface{}{
          "exists":          exists,
          "status":          "OK",
          "patientIds":      patientIds,
          "practitionerIds": practitionerIds,
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
      `;
      const fields = extractResponseFields(handlerSrc);
      assert.equal(fields.length, 4);
      assert.ok(fields.includes('exists'));
      assert.ok(fields.includes('status'));
      assert.ok(fields.includes('patientIds'));
      assert.ok(fields.includes('practitionerIds'));
    });

    await it('returns empty array when no response fields found', () => {
      const fields = extractResponseFields(`w.WriteHeader(http.StatusOK)`);
      assert.deepEqual(fields, []);
    });
  });

  await describe('extractHttpClientCalls', async () => {
    await it('detects FHIRHTTPClient.Do calls', () => {
      const handlerSrc = `
        resp, err := fhirClient.Do(ctx, req)
        fhirClient.Do(ctx, anotherReq)
      `;
      const calls = extractHttpClientCalls(handlerSrc);
      assert.equal(calls.length, 1, 'duplicate calls should be deduped');
      assert.ok(calls[0].includes('Client.Do'));
    });

    await it('detects http.Client calls', () => {
      const handlerSrc = `
        req, _ := http.NewRequest("GET", url, nil)
        resp, err := httpClient.Do(req)
        http.Get("https://example.com")
        http.Post("https://example.com", "application/json", body)
      `;
      const calls = extractHttpClientCalls(handlerSrc);
      assert.equal(calls.length, 4);
      assert.ok(calls.some(c => c.includes('httpClient.Do')));
      assert.ok(calls.some(c => c.includes('http.NewRequest')));
      assert.ok(calls.some(c => c.includes('http.Get')));
      assert.ok(calls.some(c => c.includes('http.Post')));
    });

    await it('returns empty when no HTTP calls', () => {
      const calls = extractHttpClientCalls(`utils.BuildSuccessResponse(w, 200, "ok", nil)`);
      assert.deepEqual(calls, []);
    });
  });

  await describe('extractAsyncPublishes', async () => {
    await it('detects rabbitmq publish calls', () => {
      const handlerSrc = `
        err := ctrl.amqp.Publish(ctx, "exchange", "routing-key", body)
        rabbitClient.PublishAsync(msg)
      `;
      const calls = extractAsyncPublishes(handlerSrc);
      assert.equal(calls.length, 2);
    });

    await it('returns empty when no async calls', () => {
      const calls = extractAsyncPublishes(`utils.BuildSuccessResponse(w, 200, "ok", nil)`);
      assert.deepEqual(calls, []);
    });
  });

  await describe('analyzeChains', async () => {
    await it('builds diagnostics for each route with controller file', () => {
      const routes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/magiclink',
          handler: 'CreateMagicLink',
          file: 'auth_router.go',
        }),
        makeRoute({
          method: 'POST',
          path: '/anonymous-session',
          handler: 'CreateAnonymousSession',
          file: 'auth_router.go',
        }),
      ];

      const diags = analyzeChains(routes, FIXTURES_ROOT);
      assert.equal(diags.length, 2);

      const magicLinkDiag = diags.find(d => d.endpoint.handler === 'CreateMagicLink');
      assert.ok(magicLinkDiag, 'expected CreateMagicLink diagnostic');
      assert.ok(magicLinkDiag.controllerFile.endsWith('controllers/auth_controller.go'));
      assert.ok(magicLinkDiag.usecaseCalls.length > 0);
    });

    await it('returns diagnostic with empty arrays when controller file not found', () => {
      const routes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/nonexistent',
          handler: 'NonExistent',
          file: 'unknown_router.go',
        }),
      ];

      const diags = analyzeChains(routes, FIXTURES_ROOT);
      assert.equal(diags.length, 1);
      assert.equal(diags[0].usecaseCalls.length, 0);
      assert.equal(diags[0].httpClientCalls.length, 0);
    });

    await it('returns empty array for empty routes', () => {
      const diags = analyzeChains([], FIXTURES_ROOT);
      assert.deepEqual(diags, []);
    });

    await it('infers async chain between webhook endpoints', () => {
      const routes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/hook/{service}',
          handler: 'HandleEnqueueWebHook',
          file: 'webhook_router.go',
        }),
        makeRoute({
          method: 'POST',
          path: '/callback/service-request',
          handler: 'HandleAsyncServiceResultCallback',
          file: 'webhook_router.go',
        }),
        makeRoute({
          method: 'GET',
          path: '/service-request/{id}/result',
          handler: 'HandleGetAsyncServiceResult',
          file: 'webhook_router.go',
        }),
      ];

      const diags = analyzeChains(routes, FIXTURES_ROOT);
      assert.equal(diags.length, 3);

      const enqueueDiag = diags.find(d => d.endpoint.handler === 'HandleEnqueueWebHook')!;
      assert.ok(enqueueDiag.usecaseCalls.includes('Usecase.Enqueue'));

      const callbackDiag = diags.find(d => d.endpoint.handler === 'HandleAsyncServiceResultCallback')!;
      assert.ok(callbackDiag.usecaseCalls.includes('Usecase.HandleAsyncServiceResult'));

      // Should detect chains between enqueue → callback → get-result
      const enqueueChains = enqueueDiag.detectedChains;
      assert.ok(enqueueChains.length > 0);

      const asyncChain = enqueueChains.find(c => c.type === 'async');
      assert.ok(asyncChain, 'expected an async chain between enqueue and callback');
      assert.ok(asyncChain!.confidence >= 0.9);
    });

    await it('detects data-flow chain between CreateAnonymousSession (token) and ClaimAnonymousResources (Authorization)', () => {
      const routes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/anonymous-session',
          handler: 'CreateAnonymousSession',
          file: 'auth_router.go',
        }),
      ];

      const diags = analyzeChains(routes, FIXTURES_ROOT);
      const sessionDiag = diags.find(d => d.endpoint.handler === 'CreateAnonymousSession')!;
      assert.ok(sessionDiag.responseFields.includes('token'));
    });

    await it('sets controller file for each diagnostic', () => {
      const routes: ParsedRoute[] = [
        makeRoute({
          method: 'POST',
          path: '/hook/{service}',
          handler: 'HandleEnqueueWebHook',
          file: 'webhook_router.go',
        }),
      ];

      const diags = analyzeChains(routes, FIXTURES_ROOT);
      assert.equal(diags.length, 1);
      assert.ok(diags[0].controllerFile.endsWith('controllers/webhook_controller.go'));
    });
  });
});
