import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

function loadScript(responses, random = 0.5) {
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
  source += `\nglobalThis.__test = { loginWithTransportRetry: typeof loginWithTransportRetry === "function" ? loginWithTransportRetry : undefined };`;
  vm.runInNewContext(source, context);
  return { loginWithTransportRetry: context.__test.loginWithTransportRetry, metrics, requests, sleeps };
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
