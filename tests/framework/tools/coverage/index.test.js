const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const test = require('node:test')
const assert = require('node:assert/strict')

const indexer = path.join(__dirname, 'index.js')

test('builds a navigable root index from component summaries', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'coverage-index-test-'))
  fs.mkdirSync(path.join(root, 'platform-api'), { recursive: true })
  fs.mkdirSync(path.join(root, 'api-portal', 'ui'), { recursive: true })
  fs.writeFileSync(path.join(root, 'platform-api', 'summary.json'), JSON.stringify({
    type: 'go', covered: 3, total: 4, percent: 75,
  }))
  fs.writeFileSync(path.join(root, 'platform-api', 'coverage.html'), '<html>go</html>')
  fs.writeFileSync(path.join(root, 'api-portal', 'ui', 'summary.json'), JSON.stringify({
    statements: { covered: 8, total: 10, pct: 80 },
  }))
  fs.writeFileSync(path.join(root, 'api-portal', 'ui', 'index.html'), '<html>ui</html>')

  execFileSync(process.execPath, [indexer, root])
  const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8')
  assert.match(html, /platform-api/)
  assert.match(html, /api-portal/)
  assert.match(html, /platform-api\/coverage\.html/)
  assert.match(html, /api-portal\/ui\/index\.html/)
  assert.match(html, /75\.00%/)
  assert.match(html, /80\.00%/)
})

test('skips missing or malformed summaries without unsafe links', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'coverage-index-empty-test-'))
  fs.mkdirSync(path.join(root, 'platform-api'), { recursive: true })
  fs.writeFileSync(path.join(root, 'platform-api', 'summary.json'), '{invalid')
  execFileSync(process.execPath, [indexer, root])
  const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8')
  assert.doesNotMatch(html, /href="\.\./)
  assert.match(html, /<tbody>\s*<\/tbody>/)
})
