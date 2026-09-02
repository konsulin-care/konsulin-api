#!/usr/bin/env node
// Functional pins for magiclink-stub.mjs route behavior.
//
// SonarCloud (S-rule on RegExp) wants `RegExp.exec()` instead of `path.match()`
// for route extraction; these tests prove the routes keep identical semantics
// (null or array) after the refactor. Spawns the real stub on an ephemeral port.

import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

const stubPath = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "magiclink-stub.mjs"
);

function freePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
  });
}

let child;
let base;

before(async () => {
  const port = await freePort();
  child = spawn(process.execPath, [stubPath], {
    env: { ...process.env, PORT: String(port) },
    stdio: ["ignore", "pipe", "pipe"],
  });
  await new Promise((resolve, reject) => {
    let out = "";
    const timer = setTimeout(() => reject(new Error("stub did not start")), 4000);
    child.stdout.on("data", (d) => {
      out += d.toString();
      if (out.includes("listening on")) {
        clearTimeout(timer);
        resolve();
      }
    });
    child.on("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`stub exited early (code ${code})`));
    });
  });
  base = `http://127.0.0.1:${port}`;
});

after(() => {
  child?.kill();
});

test("POST /send-magiclink files the magic link", async () => {
  const res = await fetch(`${base}/send-magiclink`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ url: "https://magic/link", email: "alice@example.test" }),
  });
  assert.equal(res.status, 200);
  assert.equal((await res.json()).success, true);
});

test("POST /send-magiclink with malformed body returns 400", async () => {
  const res = await fetch(`${base}/send-magiclink`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: "{not json",
  });
  assert.equal(res.status, 400);
  assert.equal((await res.json()).error, "invalid JSON");
});

test("inbox listing returns filed messages grouped by org", async () => {
  const res = await fetch(`${base}/api/v2/domains/public/inboxes/alice`);
  assert.equal(res.status, 200);
  const j = await res.json();
  assert.equal(j.msgs.length, 1);
  assert.ok(/^msg_\d+$/.test(j.msgs[0].id));
  assert.equal(j.msgs[0].seconds_ago, 0);
});

test("message links endpoint returns the relayed URL", async () => {
  const inbox = await (await fetch(`${base}/api/v2/domains/public/inboxes/alice`)).json();
  const id = inbox.msgs[0].id;
  const res = await fetch(`${base}/api/v2/domains/public/messages/${id}/links`);
  assert.equal(res.status, 200);
  assert.deepEqual((await res.json()).links, ["https://magic/link"]);
});

test("unknown routes return 404 with a JSON body", async () => {
  const res = await fetch(`${base}/api/v2/nope`);
  assert.equal(res.status, 404);
  assert.match((await res.json()).error, /no route/);
});
