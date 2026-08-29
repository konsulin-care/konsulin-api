#!/usr/bin/env node
// Magic-link delivery stub for the disposable CI environment.
//
// The Bruno suite's auth chain depends on two external hops in development:
//   1. the synchronous webhook service that receives the app's `send-magiclink`
//      payload (InternalConfig.Webhook.URL / HOOK_URL) and actually sends the
//      email;
//   2. the Mailinator public inbox API that docs/api/auth/consume-code.yml
//      polls to extract preAuthSessionId/linkCode from the delivered email.
//
// This stub replaces both with a per-run, in-network HTTP server:
//   POST /send-magiclink                            — app webhook forwarder target
//   GET  /api/v2/domains/public/inboxes/{org}       — inbox listing (Mailinator
//                                                     shape: msgs[] with
//                                                     id + seconds_ago)
//   GET  /api/v2/domains/public/messages/{id}/links — magic-link URLs for a msg
//
// docs/api/auth/consume-code.yml resolves its polling base URL from the
// MAILINATOR_BASE_URL collection env var, so dev/pre-push runs keep using the
// real Mailinator while CI points the suite at this stub. The stub only
// relays the magic-link URL the app forwards; nothing leaves the runner.
//
// Run: PORT=8081 node scripts/ci/magiclink-stub.mjs

import http from "node:http";

const PORT = Number(process.env.PORT || 8081);
const inboxes = new Map(); // org -> [{ id, secondsAgo, links }]
let nextMessageId = 1;

function json(res, status, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(status, {
    "content-type": "application/json",
    "content-length": Buffer.byteLength(body),
  });
  res.end(body);
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://${req.headers.host || "localhost"}`);
  const path = url.pathname;

  // App webhook forwarder target (HOOK_URL service). Receives the
  // send-magiclink payload { url, exp, email } and files the link in the
  // mailbox of the org derived from the recipient address.
  if (req.method === "POST" && path.endsWith("/send-magiclink")) {
    let raw = "";
    req.on("data", (chunk) => {
      raw += chunk;
    });
    req.on("end", () => {
      let payload;
      try {
        payload = JSON.parse(raw);
      } catch {
        return json(res, 400, { error: "invalid JSON" });
      }
      const email = String(payload.email || "");
      const org = email.split("@")[0] || "unknown";
      const id = `msg_${nextMessageId++}`;
      if (!inboxes.has(org)) {
        inboxes.set(org, []);
      }
      inboxes.get(org).push({
        id,
        secondsAgo: 0,
        links: [String(payload.url || "")],
      });
      // 2xx: the app classifies this as "dispatched" (see
      // magiclink_delivery_service.go classifyWebhookStatus).
      json(res, 200, { success: true, message: "magic link stubbed" });
    });
    return;
  }

  // Mailinator-shaped public inbox API consumed by consume-code.yml.
  const inboxMatch = path.match(/^\/api\/v2\/domains\/public\/inboxes\/([^/]+)$/);
  if (req.method === "GET" && inboxMatch) {
    const msgs = (inboxes.get(inboxMatch[1]) || []).map((m) => ({
      id: m.id,
      seconds_ago: m.secondsAgo,
    }));
    return json(res, 200, { msgs });
  }

  const linksMatch = path.match(/^\/api\/v2\/domains\/public\/messages\/([^/]+)\/links$/);
  if (req.method === "GET" && linksMatch) {
    let links = [];
    for (const msgs of inboxes.values()) {
      for (const m of msgs) {
        if (m.id === linksMatch[1]) {
          links = m.links;
        }
      }
    }
    return json(res, 200, { links });
  }

  json(res, 404, { error: `no route for ${req.method} ${path}` });
});

server.listen(PORT, () => {
  console.log(`magiclink-stub listening on :${PORT}`);
});
