#!/usr/bin/env node
// merge-apr-report.js
//
// Belongs to the `api-platform-rest-api-design-rules` skill. Renders a single
// HTML report containing this skill's API Platform house rules (APR-*, defined
// in references/api-platform-house-rules.md), REUSING the base api-design
// skill's HTML generation:
//   - the same per-dimension scoring formula as scripts/assess.js
//   - the same self-contained assets/report_template.html
// The original template is read-only and never modified. An extra "API Platform
// House Rules" section is injected into an IN-MEMORY copy of the template (two
// hardcoded section arrays each get one extra entry), then the report JSON is
// inlined exactly as assess.js does.
//
// Works for BOTH skill modes:
//   * Both legs ran (option 1): pass --report <base assess.js report json>.
//     The APR section is merged INTO that report, so the final HTML shows all
//     four dimensions plus API Platform Rules.
//   * Platform rules only (option 3): omit --report. A minimal report
//     { meta, apiPlatformReadiness } is built from the APR file's own meta, and
//     the HTML shows only the API Platform Rules section (the template already
//     skips dimensions whose data is absent).
//
// Usage:
//   node merge-apr-report.js \
//     --apr      <path to api-platform-rules.json>        (required) \
//     --template <path to api-design skill assets/report_template.html> (required) \
//     [--report  <path to *-api-readiness-report.json>]   (merge mode if present) \
//     [--meta    '<json>']   (header meta override for standalone mode) \
//     [--out       <path to output html>] \
//     [--rules-ref <path to api-platform-house-rules.md>]  (score denominator;
//                  defaults to ../references/api-platform-house-rules.md) \
//     [--write-json]         (also write/refresh the report JSON next to the HTML)

const fs = require('node:fs');
const path = require('node:path');
const { parseArgs } = require('node:util');

// ---- scoring: identical to base skill scripts/assess.js computeScore ----
const SEVERITY_PENALTY = { CRITICAL: 1.0, HIGH: 0.6, MEDIUM: 0.3, LOW: 0.15 };
function computeScore(issues, totalRules) {
  const counts = { critical: 0, high: 0, medium: 0, low: 0 };
  const penaltyByRule = new Map();
  for (const issue of issues) {
    const sev = (issue.severity || '').toUpperCase();
    if (sev === 'CRITICAL') counts.critical += 1;
    else if (sev === 'HIGH') counts.high += 1;
    else if (sev === 'MEDIUM') counts.medium += 1;
    else if (sev === 'LOW') counts.low += 1;
    const penalty = SEVERITY_PENALTY[sev] || 0;
    const rule = issue.rule || '';
    if ((penaltyByRule.get(rule) || 0) < penalty) penaltyByRule.set(rule, penalty);
  }
  if (!totalRules || totalRules <= 0) return { ...counts, score: 100 };
  let sumPenalty = 0;
  for (const p of penaltyByRule.values()) sumPenalty += p;
  let score = Math.round(((totalRules - sumPenalty) / totalRules) * 100);
  if (score < 0) score = 0;
  if (score > 100) score = 100;
  return { ...counts, score };
}

// Score denominator = the number of house rules that actually exist, counted
// from the reference file (the single source of truth) so nothing has to be
// kept in sync by hand. Each rule is a level-2 heading "## APR-NNN — ...".
// Fallback: the distinct rule codes referenced by the findings themselves, so
// the script still produces a sane score if the reference can't be read.
function countAprRules(refPath, findings) {
  try {
    const text = fs.readFileSync(refPath, 'utf8');
    const m = text.match(/^##\s+APR-\d+\b/gm);
    if (m && m.length) return m.length;
  } catch { /* fall through */ }
  const distinct = new Set((findings || []).map(f => f.rule).filter(Boolean));
  return distinct.size;
}

// Default location of the reference relative to this script (scripts/ -> ../references/).
const DEFAULT_RULES_REF = path.join(__dirname, '..', 'references', 'api-platform-house-rules.md');

function withHtmlSuffix(p) {
  const ext = path.extname(p);
  return ext ? p.slice(0, -ext.length) + '.html' : p + '.html';
}

function main() {
  const { values } = parseArgs({
    args: process.argv.slice(2),
    options: {
      apr:        { type: 'string' },
      template:   { type: 'string' },
      report:     { type: 'string' },
      meta:       { type: 'string' },
      out:        { type: 'string' },
      'rules-ref': { type: 'string' },
      'write-json': { type: 'boolean', default: false },
    },
  });

  if (!values.apr || !values.template) {
    process.stderr.write('Error: --apr and --template are required\n');
    process.exit(1);
  }
  for (const [flag, p] of [['--apr', values.apr], ['--template', values.template]]) {
    if (!fs.existsSync(p)) { process.stderr.write(`Error: ${flag} file not found: ${p}\n`); process.exit(1); }
  }
  if (values.report && !fs.existsSync(values.report)) {
    process.stderr.write(`Error: --report file not found: ${values.report}\n`);
    process.exit(1);
  }

  const apr = JSON.parse(fs.readFileSync(values.apr, 'utf8'));

  // Real findings only; drop any INFO/PASS markers (a passed rule contributes 0
  // penalty but shouldn't appear as a table row).
  const aprIssues = (apr.findings || [])
    .filter(f => (f.severity || '').toUpperCase() !== 'INFO')
    .map(f => ({
      id: f.id,
      severity: (f.severity || 'MEDIUM').toUpperCase(),
      rule: f.rule,
      path: f.path,
      issue: f.issue,
      description: f.description,
      fixSuggestion: f.fixSuggestion,
      autoFixable: !!f.autoFixable,
    }));

  // ---- assemble the report: merge mode vs standalone mode ----
  let report;
  if (values.report) {
    // Merge mode — both legs ran. Add the APR section to the existing report.
    report = JSON.parse(fs.readFileSync(values.report, 'utf8'));
  } else {
    // Standalone mode — platform rules only. Build a minimal report; derive the
    // header meta from --meta, else from the APR file's own meta block.
    let meta = {};
    if (values.meta) {
      try { meta = JSON.parse(values.meta); }
      catch (e) { process.stderr.write(`Error: --meta is not valid JSON: ${e.message}\n`); process.exit(1); }
    } else if (apr.meta) {
      meta = { specFile: apr.meta.spec, assessedAt: apr.meta.assessedAt };
    }
    report = { meta };
  }

  const rulesRef = values['rules-ref'] || DEFAULT_RULES_REF;
  const totalRules = countAprRules(rulesRef, aprIssues);

  report.apiPlatformReadiness = {
    spectral: {
      status: 'completed',
      ruleset: `api-platform-rest-api-design-rules (${totalRules} house rules)`,
      score: computeScore(aprIssues, totalRules),
      issues: aprIssues,
    },
  };

  // ---- patch an in-memory copy of the read-only template ----
  let template = fs.readFileSync(values.template, 'utf8');

  const cardAnchor = `      { key: 'design',          label: 'Design Guidelines · WSO2', data: d.designReadiness && d.designReadiness.spectral },\n    ].filter(s => s.data);`;
  const cardReplace = `      { key: 'design',          label: 'Design Guidelines · WSO2', data: d.designReadiness && d.designReadiness.spectral },\n      { key: 'api-platform',    label: 'API Platform Rules', data: d.apiPlatformReadiness && d.apiPlatformReadiness.spectral },\n    ].filter(s => s.data);`;

  const defAnchor = `      { title: 'API Design Guidelines — WSO2', subtitle: 'Automated checks via WSO2 REST API design ruleset (28 rules)', data: d.designReadiness && d.designReadiness.spectral },\n    ];`;
  const defReplace = `      { title: 'API Design Guidelines — WSO2', subtitle: 'Automated checks via WSO2 REST API design ruleset (28 rules)', data: d.designReadiness && d.designReadiness.spectral },\n      { title: 'API Platform House Rules (APR)', subtitle: 'WSO2 API Platform-specific conventions layered on top of the generic api-design checks', data: d.apiPlatformReadiness && d.apiPlatformReadiness.spectral },\n    ];`;

  if (!template.includes(cardAnchor) || !template.includes(defAnchor)) {
    process.stderr.write(
      'Error: template anchors not found. The base api-design report_template.html ' +
      'has changed shape; update the anchor strings in this script.\n'
    );
    process.exit(1);
  }
  template = template.replace(cardAnchor, cardReplace).replace(defAnchor, defReplace);

  // Inline JSON exactly as assess.js generateHtml does.
  const safeJson = JSON.stringify(report)
    .replace(/</g, '\\u003c')
    .replace(new RegExp('\\u2028', 'g'), '\\u2028')
    .replace(new RegExp('\\u2029', 'g'), '\\u2029');
  const html = template.replace('__REPORT_DATA_JSON__', () => safeJson);

  // Default output path: alongside --report in merge mode, else alongside --apr.
  const outPath = values.out
    || (values.report ? withHtmlSuffix(values.report)
                      : path.join(path.dirname(values.apr), 'api-platform-rules-report.html'));
  fs.writeFileSync(outPath, html);

  if (values['write-json']) {
    const jsonPath = values.report || withHtmlSuffix(outPath).replace(/\.html$/, '.json');
    fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2));
  }

  const s = report.apiPlatformReadiness.spectral.score;
  process.stdout.write(
    `Mode: ${values.report ? 'merge (all dimensions + API Platform)' : 'standalone (API Platform only)'}\n` +
    `API Platform Rules: ${s.score}% — ${s.critical} critical, ${s.high} high, ${s.medium} medium, ${s.low} low (${aprIssues.length} findings)\n` +
    `HTML: ${path.resolve(outPath)}\n`
  );
}

main();
