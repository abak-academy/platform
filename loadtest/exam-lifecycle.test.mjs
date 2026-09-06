import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

function loadScript(responses, random = 0.5, env = {}) {
  const metrics = new Map();
  const sleeps = [];
  const requests = [];
  const metric = (name) => ({
    add(value) {
      const values = metrics.get(name) || [];
      values.push(value);
      metrics.set(name, values);
    },
  });
  function Metric(name) {
    return metric(name);
  }
  const math = Object.create(Math);
  math.random = () => random;
  const context = {
    __ENV: {
      BASE_URL: "https://loadtest.example.test/api/v1",
      EXAM_ID: "10000000-0000-4000-8000-000000000200",
      RUN_ID: "login_retry_test",
      LOADTEST_PASSWORD: "test-password",
      ...env,
    },
    Counter: Metric,
    Rate: Metric,
    check: (response, checks) => Object.values(checks).every((check) => check(response)),
    console: { error() {} },
    encoding: { b64decode: (value) => Buffer.from(value, "base64url").toString() },
    exec: { vu: { idInTest: 1 } },
    http: {
      request(...args) {
        requests.push(args);
        return responses.shift();
      },
    },
    Math: math,
    sleep(seconds) {
      sleeps.push(seconds);
    },
  };
  let source = readFileSync(new URL("./exam-lifecycle.js", import.meta.url), "utf8")
    .replace(/^import .*;$/gm, "")
    .replace("export const options", "const options")
    .replace("export function setup", "function setup")
    .replace("export default function (test)", "function examLifecycle(test)");
  source += `\nglobalThis.__test = { loginWithTransportRetry, request, saveWithRetry, authenticatedRequest };`;
  vm.runInNewContext(source, context);
  return { ...context.__test, metrics, requests, sleeps };
}

test("retries a transport failure and records recovery separately", () => {
  const response = { status: 200 };
  const harness = loadScript([{ status: 0, error: "dial: i/o timeout" }, response]);

  assert.equal(typeof harness.loginWithTransportRetry, "function");
  assert.equal(harness.loginWithTransportRetry({ identifier: "student", password: "password" }, {}), response);
  assert.equal(harness.requests.length, 2);
  assert.deepEqual(harness.sleeps, [1.25]);
  assert.deepEqual(harness.metrics.get("login_first_attempt_failed"), [true]);
  assert.deepEqual(harness.metrics.get("login_transport_retries"), [1]);
  assert.deepEqual(harness.metrics.get("login_final_failed"), [false]);
});

test("stops after three bounded transport retries", () => {
  const finalResponse = { status: 0, error: "unexpected EOF" };
  const harness = loadScript([
    { status: 0, error: "dial: i/o timeout" },
    { status: 0, error: "request timeout" },
    { status: 0, error: "unexpected EOF" },
    finalResponse,
  ]);

  assert.equal(harness.loginWithTransportRetry({}, {}), finalResponse);
  assert.equal(harness.requests.length, 4);
  assert.deepEqual(harness.sleeps, [1.25, 2.25, 4.25]);
  assert.deepEqual(harness.metrics.get("login_first_attempt_failed"), [true]);
  assert.deepEqual(harness.metrics.get("login_transport_retries"), [1, 1, 1]);
  assert.deepEqual(harness.metrics.get("login_final_failed"), [true]);
});

test("does not retry an HTTP failure", () => {
  const response = { status: 503 };
  const harness = loadScript([response]);

  assert.equal(harness.loginWithTransportRetry({}, {}), response);
  assert.equal(harness.requests.length, 1);
  assert.deepEqual(harness.sleeps, []);
  assert.deepEqual(harness.metrics.get("login_first_attempt_failed"), [true]);
  assert.equal(harness.metrics.has("login_transport_retries"), false);
  assert.deepEqual(harness.metrics.get("login_final_failed"), [true]);
});

test("transport mode recovers a GET and records each retry", () => {
  const response = { status: 200 };
  const harness = loadScript([{ status: 0 }, { status: 0 }, response], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });

  assert.equal(harness.request("GET", "/exam/registrations", null, {}, "registration"), response);
  assert.equal(harness.requests.length, 3);
  assert.deepEqual(harness.sleeps, [1.5, 2.5]);
  assert.deepEqual(harness.metrics.get("transport_retries"), [1, 1]);
});

test("transport mode returns server errors without hiding them behind transport retries", () => {
  const response = { status: 503 };
  const harness = loadScript([response], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });

  assert.equal(harness.request("GET", "/exam/registrations", null, {}, "registration"), response);
  assert.equal(harness.requests.length, 1);
  assert.deepEqual(harness.sleeps, []);
});

test("transport mode keeps login bounded to three retries", () => {
  const response = { status: 0 };
  const harness = loadScript([{ status: 0 }, { status: 0 }, { status: 0 }, response, { status: 200 }], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });

  assert.equal(harness.loginWithTransportRetry({}, {}), response);
  assert.equal(harness.requests.length, 4);
  assert.deepEqual(harness.metrics.get("login_first_attempt_failed"), [true]);
  assert.deepEqual(harness.metrics.get("login_final_failed"), [true]);
  assert.deepEqual(harness.metrics.get("transport_retries"), [1, 1, 1]);
});

test("GET transport retries stop after four attempts and preserve headers", () => {
  const response = { status: 0 };
  const harness = loadScript([response, response, response, response, { status: 200 }], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });
  const headers = { Authorization: "Bearer access", "X-Forwarded-For": "198.18.0.1" };

  assert.equal(harness.request("GET", "/exam/sessions/session", null, headers, "reconnect"), response);
  assert.equal(harness.requests.length, 4);
  assert.deepEqual(harness.sleeps, [1.5, 2.5, 4.5]);
  assert.deepEqual(harness.metrics.get("transport_retries"), [1, 1, 1]);
  for (const request of harness.requests) {
    assert.equal(request[3].headers.Authorization, headers.Authorization);
    assert.equal(request[3].headers["X-Forwarded-For"], headers["X-Forwarded-For"]);
  }
});

test("GET transport retries remain opt-in", () => {
  const response = { status: 0 };
  const harness = loadScript([response, { status: 200 }]);

  assert.equal(harness.request("GET", "/exam/registrations", null, {}, "registration"), response);
  assert.equal(harness.requests.length, 1);
});

test("transport mode never blindly retries mutation requests", () => {
  for (const [method, path, phase] of [
    ["POST", "/auth/refresh", "refresh"],
    ["POST", "/exam/sessions", "start"],
    ["POST", "/exam/sessions/session/submit", "submit"],
    ["POST", "/exam/checkin", "checkin"],
    ["POST", "/exam/sessions/session/sections/test/advance", "advance"],
    ["PATCH", "/exam/sessions/session/answers", "autosave"],
  ]) {
    const response = { status: 0 };
    const harness = loadScript([response, { status: 200 }], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });
    assert.equal(harness.request(method, path, {}, {}, phase), response, phase);
    assert.equal(harness.requests.length, 1, phase);
    assert.deepEqual(harness.sleeps, [], phase);
  }
});

const freshAuth = () => ({ accessToken: "access", refreshToken: "refresh", expiresAt: Date.now() + 3600000 });

test("autosave retries an identical payload without multiplying retry budgets", () => {
  const harness = loadScript([{ status: 0 }, { status: 0 }, { status: 0 }, { status: 0 }, { status: 200 }], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true", SAVE_RETRIES: "3" });
  const headers = { "X-Forwarded-For": "198.18.0.1" };

  assert.equal(harness.saveWithRetry("session", [{ question_id: "question", answer: "A" }], 0, freshAuth(), headers), false);
  assert.equal(harness.requests.length, 4);
  assert.deepEqual(harness.metrics.get("transport_retries"), [1, 1, 1]);
  assert.deepEqual(harness.sleeps, [2.5, 4.5, 8.5]);
  for (const request of harness.requests) {
    assert.equal(request[2], harness.requests[0][2]);
    assert.equal(request[3].headers["X-Forwarded-For"], headers["X-Forwarded-For"]);
  }
});

test("autosave can recover and respects SAVE_RETRIES zero", () => {
  const recovered = loadScript([{ status: 0 }, { status: 200 }]);
  assert.equal(recovered.saveWithRetry("session", [], 0, freshAuth(), {}), true);
  assert.equal(recovered.requests.length, 2);
  assert.deepEqual(recovered.metrics.get("transport_retries"), [1]);

  const disabled = loadScript([{ status: 0 }, { status: 200 }], 0.5, { SAVE_RETRIES: "0" });
  assert.equal(disabled.saveWithRetry("session", [], 0, freshAuth(), {}), false);
  assert.equal(disabled.requests.length, 1);
  assert.deepEqual(disabled.sleeps, []);
});

test("autosave does not retry HTTP errors", () => {
  for (const status of [400, 403, 409, 429, 500, 503]) {
    const harness = loadScript([{ status }, { status: 200 }]);
    assert.equal(harness.saveWithRetry("session", [], 0, freshAuth(), {}), false);
    assert.equal(harness.requests.length, 1);
    assert.deepEqual(harness.sleeps, []);
  }
});

test("failed rotating refresh stops autosave without replaying the token", () => {
  for (const status of [0, 401, 503]) {
    const harness = loadScript([{ status }, { status: 200 }], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });
    assert.equal(harness.saveWithRetry("session", [], 0, { ...freshAuth(), expiresAt: 0 }, {}), false);
    assert.equal(harness.requests.length, 1);
    assert.match(harness.requests[0][1], /auth\/refresh$/);
    assert.deepEqual(harness.sleeps, []);
  }
});

test("failed refresh after autosave 401 also stops without retrying", () => {
  const harness = loadScript([{ status: 401 }, { status: 0 }, { status: 200 }]);
  assert.equal(harness.saveWithRetry("session", [], 0, freshAuth(), {}), false);
  assert.equal(harness.requests.length, 2);
  assert.deepEqual(harness.sleeps, []);
});

test("one-time 401 refresh still rotates tokens and preserves device identity", () => {
  const accessToken = `header.${Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64url")}.signature`;
  const refreshResponse = {
    status: 200,
    json: (key) => ({ access_token: accessToken, refresh_token: "rotated-refresh" })[key],
  };
  for (const finalStatus of [200, 401]) {
    const finalResponse = { status: finalStatus };
    const harness = loadScript([{ status: 401 }, refreshResponse, finalResponse]);
    const auth = freshAuth();
    const headers = { "X-Forwarded-For": "198.18.0.1" };

    assert.equal(harness.saveWithRetry("session", [], 0, auth, headers), finalStatus === 200);
    assert.equal(harness.requests.length, 3);
    assert.equal(auth.refreshToken, "rotated-refresh");
    assert.equal(harness.requests[2][3].headers.Authorization, `Bearer ${accessToken}`);
    for (const request of harness.requests) assert.equal(request[3].headers["X-Forwarded-For"], headers["X-Forwarded-For"]);
    assert.deepEqual(harness.sleeps, []);
  }
});
