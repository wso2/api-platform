#!/usr/bin/env node

const fs = require('node:fs')
const path = require('node:path')

if (!process.argv[2]) {
  console.error('usage: index.js <coverage-root>')
  process.exit(2)
}
const root = path.resolve(process.argv[2])
if (!root || !fs.existsSync(root)) {
  console.error('usage: index.js <coverage-root>')
  process.exit(2)
}

const entries = [
  ['platform-gateway', 'controller', 'platform-gateway/controller/summary.json', 'platform-gateway/controller/coverage.html'],
  ['platform-gateway', 'runtime', 'platform-gateway/runtime/summary.json', 'platform-gateway/runtime/coverage.html'],
  ['platform-api', 'service', 'platform-api/summary.json', 'platform-api/coverage.html'],
  ['ai-workspace', 'bff', 'ai-workspace/bff/summary.json', 'ai-workspace/bff/coverage.html'],
  ['ai-workspace', 'ui', 'ai-workspace/ui/summary.json', 'ai-workspace/ui/index.html'],
  ['api-portal', 'server', 'api-portal/server/coverage-summary.json', 'api-portal/server/index.html'],
  ['api-portal', 'ui', 'api-portal/ui/summary.json', 'api-portal/ui/index.html'],
]

function readJSON(relative) {
  try { return JSON.parse(fs.readFileSync(path.join(root, relative), 'utf8')) } catch (_) { return null }
}

function metric(summary) {
  if (!summary) return null
  const selected = summary.total?.statements || summary.statements || summary
  const percent = selected.pct ?? selected.percent
  if (typeof selected !== 'object' || selected === null || Array.isArray(selected)
    || ![selected.covered, selected.total, percent].every(Number.isFinite)) return null
  return selected
}

function escape(value) {
  return String(value).replace(/[&<>'"]/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;',
  }[character]))
}

const rows = entries.flatMap(([component, service, summaryPath, reportPath]) => {
  const summary = metric(readJSON(summaryPath))
  if (!summary) return []
  const report = path.relative(root, path.join(root, reportPath)).split(path.sep).join('/')
  return [{ component, service, percent: Number(summary.pct ?? summary.percent), covered: summary.covered, total: summary.total, report }]
})

const html = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Coverage reports</title>
<style>body{font:16px system-ui,sans-serif;margin:2rem;color:#222}table{border-collapse:collapse;min-width:48rem}th,td{border:1px solid #ccc;padding:.55rem;text-align:left}th{background:#eee}td:nth-child(3),td:nth-child(4){text-align:right}a{color:#0645ad}</style>
</head><body><h1>Coverage reports</h1>
<p>Component and service coverage collected by the integration framework.</p>
<table><thead><tr><th>Component</th><th>Service</th><th>Coverage</th><th>Statements</th><th>Report</th></tr></thead><tbody>
${rows.map((row) => `<tr><td>${escape(row.component)}</td><td>${escape(row.service)}</td><td>${escape(row.percent.toFixed(2))}%</td><td>${escape(`${row.covered}/${row.total}`)}</td><td><a href="${escape(row.report)}">Open HTML report</a></td></tr>`).join('\n')}
</tbody></table></body></html>\n`

const temporary = path.join(root, `.index-${process.pid}-${Date.now()}.tmp`)
fs.writeFileSync(temporary, html, { mode: 0o644 })
fs.renameSync(temporary, path.join(root, 'index.html'))
console.log(`coverage index: ${rows.length} reports under ${root}`)
