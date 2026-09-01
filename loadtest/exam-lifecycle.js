import exec from "k6/execution";
import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate } from "k6/metrics";

const BASE_URL = (__ENV.BASE_URL || "").replace(/\/$/, "");
const EXAM_ID = __ENV.EXAM_ID || "";
const RUN_ID = __ENV.RUN_ID || "";
const PASSWORD = __ENV.LOADTEST_PASSWORD || "";
const USERS = integerEnv("USERS", 1);
const LOGIN_SPREAD_SECONDS = numberEnv("LOGIN_SPREAD_SECONDS", 60);
const ANSWER_INTERVAL_SECONDS = numberEnv("ANSWER_INTERVAL_SECONDS", 45);
const ANSWER_JITTER_SECONDS = numberEnv("ANSWER_JITTER_SECONDS", 10);
const MAX_QUESTIONS = integerEnv("MAX_QUESTIONS", 0);
const SAVE_RETRIES = integerEnv("SAVE_RETRIES", 3);
const SUBMIT_AT_SECONDS = numberEnv("SUBMIT_AT_SECONDS", 0);
const MAX_DURATION = __ENV.MAX_DURATION || "2h";
const REQUIRES_CHECKIN = (__ENV.REQUIRES_CHECKIN || "false") === "true";

const lifecycleFailed = new Rate("lifecycle_failed");
const completedLifecycles = new Counter("completed_lifecycles");
const lostAnswers = new Counter("lost_answers");

export const options = {
  scenarios: {
    exam_lifecycle: {
      executor: "per-vu-iterations",
      vus: USERS,
      iterations: 1,
      maxDuration: MAX_DURATION,
    },
  },
  thresholds: {
    lifecycle_failed: ["rate<0.01"],
    lost_answers: ["count==0"],
    "http_req_failed{phase:login}": ["rate<0.01"],
    "http_req_failed{phase:start}": ["rate<0.01"],
    "http_req_failed{phase:autosave}": ["rate<0.01"],
    "http_req_failed{phase:reconnect}": ["rate<0.01"],
    "http_req_failed{phase:submit}": ["rate<0.01"],
    "http_req_duration{phase:login}": [`p(95)<${numberEnv("LOGIN_P95_MS", 1000)}`],
    "http_req_duration{phase:start}": [`p(95)<${numberEnv("START_P95_MS", 1500)}`],
    "http_req_duration{phase:autosave}": [`p(95)<${numberEnv("AUTOSAVE_P95_MS", 300)}`],
    "http_req_duration{phase:reconnect}": [`p(95)<${numberEnv("RECONNECT_P95_MS", 1500)}`],
    "http_req_duration{phase:submit}": [`p(95)<${numberEnv("SUBMIT_P95_MS", 1500)}`],
  },
};

export function setup() {
  if (__ENV.NON_PRODUCTION_CONFIRM !== "loadtest") {
    throw new Error("set NON_PRODUCTION_CONFIRM=loadtest");
  }
  if (!BASE_URL || !EXAM_ID || !RUN_ID || !PASSWORD) {
    throw new Error("BASE_URL, EXAM_ID, RUN_ID, and LOADTEST_PASSWORD are required");
  }
  if (!/^[a-z0-9][a-z0-9_-]{2,30}$/.test(RUN_ID)) {
    throw new Error("RUN_ID must match ^[a-z0-9][a-z0-9_-]{2,30}$");
  }
  if (USERS < 1 || USERS > 10000) {
    throw new Error("USERS must be between 1 and 10000");
  }

  return {
    submitAt: SUBMIT_AT_SECONDS > 0 ? Date.now() + SUBMIT_AT_SECONDS * 1000 : 0,
  };
}

export default function (test) {
  const index = exec.vu.idInTest;
  const suffix = String(index).padStart(5, "0");
  const identifier = `lt_${RUN_ID}_${suffix}`;
  const registrationToken = `lt-${RUN_ID}-${suffix}`;
  const clientHeaders = { "X-Forwarded-For": virtualUserIP(index) };

  sleep(Math.random() * LOGIN_SPREAD_SECONDS);

  const login = request("POST", "/auth/login", {
    identifier,
    password: PASSWORD,
  }, clientHeaders, "login");
  if (!expectStatus(login, 200, "login")) return failLifecycle();

  const accessToken = json(login, "access_token");
  if (!accessToken) return failLifecycle();
  const headers = { ...clientHeaders, Authorization: `Bearer ${accessToken}` };

  const registrations = request("GET", "/exam/registrations", null, headers, "registration");
  if (!expectStatus(registrations, 200, "registration list")) return failLifecycle();

  const registration = (json(registrations, "data") || []).find((item) => item.exam_id === EXAM_ID);
  if (!registration || !registration.id) return failLifecycle();

  if (REQUIRES_CHECKIN) {
    const checkin = request("POST", "/exam/checkin", { token: registrationToken }, headers, "checkin");
    if (!expectStatus(checkin, 200, "check-in")) return failLifecycle();
  }

  const start = request("POST", "/exam/sessions", {
    registration_id: registration.id,
  }, headers, "start");
  if (!expectStatus(start, 200, "start")) return failLifecycle();

  const started = responseJSON(start);
  if (!started || !started.session_id || !Array.isArray(started.tests)) return failLifecycle();
  const questionCount = started.tests.reduce(
    (total, testSection) => total + (Array.isArray(testSection.questions) ? testSection.questions.length : 0),
    0,
  );
  if (questionCount === 0) return failLifecycle();

  const sessionID = started.session_id;
  const sectioned = started.mode === "utbk" || started.mode === "ielts";
  const answers = new Map();
  const standardAnswers = [];
  let position = 0;
  let answered = 0;

  for (const testSection of started.tests) {
    if (!Array.isArray(testSection.questions)) continue;
    const saveAnswers = sectioned ? [] : standardAnswers;
    let sectionPosition = 0;

    for (const question of testSection.questions) {
      if (MAX_QUESTIONS > 0 && answered >= MAX_QUESTIONS) break;
      sleep(humanDelay());

      const answer = answerFor(question);
      const input = {
        question_id: question.id,
        answer,
        flagged_for_review: false,
      };
      answers.set(question.id, answer);
      saveAnswers.push(input);

      const currentPosition = sectioned ? sectionPosition : position;
      if (!saveWithRetry(sessionID, saveAnswers, currentPosition, headers)) return failLifecycle();
      answered++;
      position++;
      sectionPosition++;
    }

    if (sectioned && saveAnswers.length > 0) {
      const advance = request(
        "POST",
        `/exam/sessions/${sessionID}/sections/${testSection.id}/advance`,
        null,
        headers,
        "advance",
      );
      if (!expectStatus(advance, 200, "advance section")) return failLifecycle();
    }

    if (MAX_QUESTIONS > 0 && answered >= MAX_QUESTIONS) break;
  }

  const reconnect = request("GET", `/exam/sessions/${sessionID}`, null, headers, "reconnect");
  if (!expectStatus(reconnect, 200, "reconnect")) return failLifecycle();
  if (!answersPersisted(responseJSON(reconnect), answers)) return failLifecycle();

  if (test.submitAt > 0) {
    const waitSeconds = (test.submitAt - Date.now()) / 1000;
    if (waitSeconds > 0) sleep(waitSeconds);
  }

  const submit = request("POST", `/exam/sessions/${sessionID}/submit`, null, headers, "submit");
  if (!expectStatus(submit, 200, "submit")) return failLifecycle();

  completedLifecycles.add(1);
  lifecycleFailed.add(false);
}

function saveWithRetry(sessionID, answers, position, headers) {
  for (let attempt = 0; attempt <= SAVE_RETRIES; attempt++) {
    const response = request(
      "PATCH",
      `/exam/sessions/${sessionID}/answers`,
      { answers, current_position: position },
      headers,
      "autosave",
    );
    if (response.status === 200) return true;
    if (attempt < SAVE_RETRIES) sleep(2 ** (attempt + 1));
  }
  return false;
}

function answersPersisted(state, expected) {
  if (!state || !Array.isArray(state.answers)) {
    lostAnswers.add(expected.size);
    return false;
  }

  const actual = new Map(state.answers.map((answer) => [answer.question_id, answer.answer || ""]));
  let lost = 0;
  for (const [questionID, answer] of expected) {
    if (actual.get(questionID) !== answer) lost++;
  }
  lostAnswers.add(lost);
  return lost === 0;
}

function answerFor(question) {
  switch (question.format) {
    case "mcq":
      return question.options && question.options.length > 0 ? question.options[0].key : "A";
    case "multi_answer":
      return (question.options || []).slice(0, 2).map((option) => option.key).join(",");
    case "multi_blank":
      return JSON.stringify((question.blanks || []).map(() => "loadtest"));
    case "true_false":
      return JSON.stringify((question.statements || []).map(() => "false"));
    default:
      return "loadtest answer";
  }
}

function request(method, path, body, headers, phase) {
  const params = {
    headers: {
      Accept: "application/json",
      ...(headers || {}),
    },
    tags: { phase },
  };
  let payload = null;
  if (body !== null) {
    payload = JSON.stringify(body);
    params.headers["Content-Type"] = "application/json";
  }
  return http.request(method, `${BASE_URL}${path}`, payload, params);
}

function expectStatus(response, status, label) {
  const passed = check(response, { [`${label}: status ${status}`]: (res) => res.status === status });
  if (!passed && exec.vu.idInTest <= 3) {
    console.error(`${label}: got ${response.status} body=${response.body}`);
  }
  return passed;
}

function responseJSON(response) {
  try {
    return response.json();
  } catch (_) {
    return null;
  }
}

function json(response, selector) {
  try {
    return response.json(selector);
  } catch (_) {
    return null;
  }
}

function failLifecycle() {
  lifecycleFailed.add(true);
}

function humanDelay() {
  const jitter = (Math.random() * 2 - 1) * ANSWER_JITTER_SECONDS;
  return Math.max(0, ANSWER_INTERVAL_SECONDS + jitter);
}

function integerEnv(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === "") return fallback;
  if (!/^-?[0-9]+$/.test(raw)) {
    throw new Error(`${name} must be an integer`);
  }
  const value = Number(raw);
  if (!Number.isSafeInteger(value)) {
    throw new Error(`${name} must be a safe integer`);
  }
  return value;
}

function numberEnv(name, fallback) {
  const value = Number.parseFloat(__ENV[name] || String(fallback));
  return Number.isFinite(value) ? value : fallback;
}

function virtualUserIP(index) {
  const zeroBased = index - 1;
  return `10.240.${Math.floor(zeroBased / 254)}.${(zeroBased % 254) + 1}`;
}
