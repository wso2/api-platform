#!/usr/bin/env node

const fs = require('node:fs')
const path = require('node:path')

const [, , input, output, sourceRoot] = process.argv
if (!input || !output || !sourceRoot) {
  console.error('usage: normalize-v8.js <input> <output> <source-root>')
  process.exit(2)
}

const report = JSON.parse(fs.readFileSync(input, 'utf8'))
const root = path.resolve(sourceRoot).replace(/\\/g, '/')
const prefix = 'file:///app/'

for (const entry of report.result || []) {
  if (typeof entry.url === 'string' && entry.url.startsWith(prefix)) {
    entry.url = `file://${root}/${entry.url.slice(prefix.length)}`
  }
}

fs.writeFileSync(output, JSON.stringify(report))
