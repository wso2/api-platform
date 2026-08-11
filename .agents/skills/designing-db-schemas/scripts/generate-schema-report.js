#!/usr/bin/env node
// generate-schema-report.js
//
// Belongs to the `designing-db-schemas` skill.
// Writes a structured JSON findings report from schema review findings.
//
// Usage:
//   node generate-schema-report.js \
//     --findings '<json-array-of-findings>'  (required) \
//     --schema   <path-to-reviewed-schema>   (required) \
//     [--out     <output-path>]              (default: ./schema-reports/schema-review.json)
//
// Finding shape (each element of --findings array):
//   { "rule": "R3-NO-TEXT", "table": "apis", "column": "config",
//     "severity": "HIGH"|"MEDIUM"|"LOW"|"LEGACY-ACCEPTED", "finding": "...", "fix": "..." }
//
// LEGACY-ACCEPTED marks a violation on a shipped (GA) table, frozen by R0.
// It is recorded, not remediated — such findings carry no `fix`.
//
// Output shape:
//   {
//     "meta": { "schema": "...", "reviewedAt": "...", "rules": [...] },
//     "summary": { "HIGH": N, "MEDIUM": N, "LOW": N, "LEGACY-ACCEPTED": N },
//     "findings": [ { "id": "r1-001", "severity": "...", "rule": "...", ... } ]
//   }
//
// IDs are deterministic: findings are sorted on stable keys BEFORE numbering,
// so the same finding set always yields the same rN-### ids regardless of the
// order they were passed in. That keeps ids comparable across reviews.

'use strict';

const fs   = require('fs');
const path = require('path');

// ---------- arg parsing ----------
const args = process.argv.slice(2);
function flag(name) {
  const i = args.indexOf(name);
  return i !== -1 ? args[i + 1] : null;
}

if (args.includes('--help') || args.includes('-h')) {
  console.log("Usage: generate-schema-report.js --findings '[...]' --schema <path> [--out <path>]");
  process.exit(0);
}

const findingsRaw = flag('--findings');
const schemaPath  = flag('--schema');
const outPath     = flag('--out') || './schema-reports/schema-review.json';

if (!findingsRaw || !schemaPath) {
  console.error("Usage: generate-schema-report.js --findings '[...]' --schema <path> [--out <path>]");
  process.exit(1);
}

// ---------- parse findings ----------
let findings;
try {
  findings = JSON.parse(findingsRaw);
} catch (e) {
  console.error('--findings must be a valid JSON array:', e.message);
  process.exit(1);
}

if (!Array.isArray(findings)) {
  console.error('--findings must be a JSON array');
  process.exit(1);
}

// Severity ordering and the set of supported, normalised severity values.
// LEGACY-ACCEPTED sorts last: it is a record of an R0-frozen deviation, not
// actionable work.
const ORDER = { HIGH: 0, MEDIUM: 1, LOW: 2, 'LEGACY-ACCEPTED': 3 };

// ---------- normalise (no ids yet) ----------
const normalised = findings.map(f => {
  const rule = f.rule || 'UNKNOWN';
  const sev  = String(f.severity || 'MEDIUM').toUpperCase();
  return {
    severity: ORDER[sev] !== undefined ? sev : 'MEDIUM',
    rule,
    table:    f.table  || null,
    column:   f.column || null,
    finding:  f.finding || '',
    fix:      f.fix     || '',
  };
});

// ---------- sort on stable keys BEFORE numbering ----------
// Severity first (report order), then rule/table/column/finding so that two
// runs over the same findings in a different input order produce identical ids.
const cmp = (a, b) => String(a ?? '').localeCompare(String(b ?? ''));
normalised.sort((a, b) =>
  (ORDER[a.severity] ?? 9) - (ORDER[b.severity] ?? 9) ||
  cmp(a.rule, b.rule) ||
  cmp(a.table, b.table) ||
  cmp(a.column, b.column) ||
  cmp(a.finding, b.finding)
);

// ---------- assign ids from the sorted order ----------
const counters = {};
for (const f of normalised) {
  // Rule-group identifier per the report contract: R3-NO-TEXT -> r3
  const group = f.rule.split('-')[0].toLowerCase().replace(/[^a-z0-9]/g, '') || 'unknown';
  counters[group] = (counters[group] || 0) + 1;
  f.id = `${group}-${String(counters[group]).padStart(3, '0')}`;
}

// Put `id` first in each object for readability
const ordered = normalised.map(({ id, severity, rule, table, column, finding, fix }) =>
  ({ id, severity, rule, table, column, finding, fix }));

// ---------- summary counts ----------
const summary = { HIGH: 0, MEDIUM: 0, LOW: 0, 'LEGACY-ACCEPTED': 0 };
for (const f of ordered) summary[f.severity] = (summary[f.severity] || 0) + 1;

// ---------- build output ----------
const report = {
  meta: {
    schema:     schemaPath,
    reviewedAt: new Date().toISOString(),
    rules:      ['R0','R1','R2','R3','R4','R5','R6','R7','R8','R9','R10'],
  },
  summary,
  findings: ordered,
};

// ---------- write output ----------
const outDir = path.dirname(outPath);
if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });

fs.writeFileSync(outPath, JSON.stringify(report, null, 2) + '\n');
console.log(`Schema review report written to: ${outPath}`);
console.log(`  HIGH: ${summary.HIGH}  MEDIUM: ${summary.MEDIUM}  LOW: ${summary.LOW}` +
            `  LEGACY-ACCEPTED: ${summary['LEGACY-ACCEPTED']}  Total: ${ordered.length}`);
