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
    check: () => true,
    console,
    encoding: { b64decode: () => "{}" },
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
  source += `\nglobalThis.__test = { loginWithTransportRetry: typeof loginWithTransportRetry === "function" ? loginWithTransportRetry : undefined, request };`;
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

test("capacity mode keeps retrying transport errors and records every recovery attempt", () => {
  const response = { status: 200 };
  const harness = loadScript([{ status: 0 }, { status: 0 }, response], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });

  assert.equal(harness.request("GET", "/exam/registrations", null, {}, "registration"), response);
  assert.equal(harness.requests.length, 3);
  assert.deepEqual(harness.sleeps, [1.5, 2.5]);
  assert.deepEqual(harness.metrics.get("transport_retries"), [1, 1]);
});

test("capacity mode returns server errors without hiding them behind transport retries", () => {
  const response = { status: 503 };
  const harness = loadScript([response], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });

  assert.equal(harness.request("GET", "/exam/registrations", null, {}, "registration"), response);
  assert.equal(harness.requests.length, 1);
  assert.deepEqual(harness.sleeps, []);
});

test("capacity login survives more than three transport failures without hiding the first failure", () => {
  const response = { status: 200 };
  const harness = loadScript([{ status: 0 }, { status: 0 }, { status: 0 }, { status: 0 }, response], 0.5, { CONTINUE_TRANSPORT_ERRORS: "true" });

  assert.equal(harness.loginWithTransportRetry({}, {}), response);
  assert.equal(harness.requests.length, 5);
  assert.deepEqual(harness.metrics.get("login_first_attempt_failed"), [true]);
  assert.deepEqual(harness.metrics.get("login_final_failed"), [false]);
  assert.deepEqual(harness.metrics.get("transport_retries"), [1, 1, 1, 1]);
});
