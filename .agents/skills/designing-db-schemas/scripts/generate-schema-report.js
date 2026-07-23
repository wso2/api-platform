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
//     "severity": "HIGH"|"MEDIUM"|"LOW", "finding": "...", "fix": "..." }
//
// Output shape:
//   {
//     "meta": { "schema": "...", "reviewedAt": "...", "rules": [...] },
//     "summary": { "HIGH": N, "MEDIUM": N, "LOW": N },
//     "findings": [ { "id": "r1-001", "severity": "...", "rule": "...", ... } ]
//   }

'use strict';

const fs   = require('fs');
const path = require('path');

// ---------- arg parsing ----------
const args = process.argv.slice(2);
function flag(name) {
  const i = args.indexOf(name);
  return i !== -1 ? args[i + 1] : null;
}

const findingsRaw = flag('--findings');
const schemaPath  = flag('--schema');
const outPath     = flag('--out') || './schema-reports/schema-review.json';

if (!findingsRaw || !schemaPath) {
  console.error('Usage: generate-schema-report.js --findings \'[...]\' --schema <path> [--out <path>]');
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

// Severity ordering and the set of supported, normalised severity values
const ORDER = { HIGH: 0, MEDIUM: 1, LOW: 2 };

// Assign sequential IDs per rule group and normalise
const counters = {};
const normalised = findings.map(f => {
  const rule = f.rule || 'UNKNOWN';
  // Rule-group identifier per the report contract: R3-NO-TEXT -> r3
  const group = rule.split('-')[0].toLowerCase().replace(/[^a-z0-9]/g, '') || 'unknown';
  counters[group] = (counters[group] || 0) + 1;
  const seq = String(counters[group]).padStart(3, '0');
  // Normalise severity to a supported uppercase value before sort/summary
  const sev = String(f.severity || 'MEDIUM').toUpperCase();
  return {
    id:       `${group}-${seq}`,
    severity: ORDER[sev] !== undefined ? sev : 'MEDIUM',
    rule,
    table:    f.table  || null,
    column:   f.column || null,
    finding:  f.finding || '',
    fix:      f.fix     || '',
  };
});

// Sort: HIGH → MEDIUM → LOW
normalised.sort((a, b) => (ORDER[a.severity] ?? 3) - (ORDER[b.severity] ?? 3));

// Summary counts
const summary = { HIGH: 0, MEDIUM: 0, LOW: 0 };
for (const f of normalised) summary[f.severity] = (summary[f.severity] || 0) + 1;

// ---------- build output ----------
const report = {
  meta: {
    schema:     schemaPath,
    reviewedAt: new Date().toISOString(),
    rules:      ['R1','R2','R3','R4','R5','R6','R7','R8','R9','R10'],
  },
  summary,
  findings: normalised,
};

// ---------- write output ----------
const outDir = path.dirname(outPath);
if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });

fs.writeFileSync(outPath, JSON.stringify(report, null, 2) + '\n');
console.log(`Schema review report written to: ${outPath}`);
console.log(`  HIGH: ${summary.HIGH}  MEDIUM: ${summary.MEDIUM}  LOW: ${summary.LOW}  Total: ${normalised.length}`);
