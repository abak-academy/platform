const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { spawnSync } = require('node:child_process');
const output = path.resolve(process.env.EXAM_FLOW_EVIDENCE_DIR || path.join(__dirname, 'evidence-handoff-2'));
fs.mkdirSync(output, { recursive: true });
const inputs = ['index.html', 'style.css', 'tests.css', 'app.js', 'tests.js', 'verify.cjs', 'verify-tests.cjs', 'verify-feedback.cjs', 'verify-handoff.cjs'];
const report = {
  scope: 'static prototype only; no production API, auth, database, build, or deployment verification',
  verified_at: new Date().toISOString(),
  base_commit: '0d2b6ca1a6c2985f903d16fbd94e71ee297a4540',
  sources: Object.fromEntries(inputs.map(name => [name, createHash('sha256').update(fs.readFileSync(path.join(__dirname, name))).digest('hex')])),
  checks: [],
};
for (const [name, script] of [['exam', 'verify.cjs'], ['tests', 'verify-tests.cjs'], ['feedback', 'verify-feedback.cjs']]) {
  const result = spawnSync(process.execPath, [path.join(__dirname, script)], {
    env: { ...process.env, EXAM_FLOW_EVIDENCE_DIR: path.join(output, name) },
    encoding: 'utf8', timeout: 180000,
  });
  const log = (result.stdout || '') + (result.stderr || '') + (result.error ? String(result.error) : '');
  fs.writeFileSync(path.join(output, name + '.log'), log);
  report.checks.push({ name, script, passed: result.status === 0, exit_code: result.status, log: name + '.log' });
  console.log(`${name}: ${result.status === 0 ? 'PASS' : 'FAIL'}`);
  if (result.status !== 0) break;
}
report.passed = report.checks.length === 3 && report.checks.every(check => check.passed);
fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
if (!report.passed) process.exitCode = 1;
