const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const test = require('node:test')
const assert = require('node:assert/strict')

const converter = path.join(__dirname, 'browser-to-istanbul.js')

function fixture() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'browser-coverage-test-'))
  const input = path.join(root, 'input', 'block', 'scenario')
  const sourceRoot = path.join(root, 'src')
  const output = path.join(root, 'output')
  fs.mkdirSync(input, { recursive: true })
  fs.mkdirSync(sourceRoot, { recursive: true })
  return { root, input, sourceRoot, output }
}

test('merges source-level Istanbul counters and inventories unvisited source', () => {
  const { root, input, sourceRoot, output } = fixture()
  const covered = path.join(sourceRoot, 'covered.js')
  const uncovered = path.join(sourceRoot, 'uncovered.js')
  fs.writeFileSync(covered, 'function covered() { return 1 }\ncovered()\n')
  fs.writeFileSync(uncovered, 'function neverCalled() { return 2 }\n')
  fs.writeFileSync(path.join(input, 'raw-istanbul.json'), JSON.stringify({
    ['/web/src/covered.js']: {
      path: '/web/src/covered.js',
      statementMap: { 0: { start: { line: 1, column: 28 }, end: { line: 1, column: 36 } } },
      fnMap: {}, branchMap: {}, s: { 0: 1 }, f: {}, b: {},
    },
  }))

  execFileSync(process.execPath, [
    converter, input, output, root,
    '--source-root', 'src', '--inventory-root', 'src', '--include', 'src/**/*.js',
  ])

  const summary = JSON.parse(fs.readFileSync(path.join(output, 'summary.json'), 'utf8'))
  const lcov = fs.readFileSync(path.join(output, 'lcov.info'), 'utf8')
  assert.equal(summary.statements.total, 2)
  assert.equal(summary.statements.covered, 1)
  assert.match(lcov, /SF:.*covered\.js/)
  assert.match(lcov, /SF:.*uncovered\.js/)
  assert.equal(fs.existsSync(path.join(output, 'index.html')), true)
})

test('fails loudly when source-level browser coverage is absent', () => {
  const { root, input, sourceRoot, output } = fixture()
  fs.writeFileSync(path.join(sourceRoot, 'app.js'), 'console.log("app")\n')

  assert.throws(
    () => execFileSync(process.execPath, [
      converter, input, output, root, '--source-root', 'src', '--inventory-root', 'src',
    ], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }),
    (error) => {
      assert.equal(error.status, 1)
      assert.match(error.stderr, /source-level browser coverage reports found/)
      assert.match(error.stderr, /CDP\/V8 fallback is disabled/)
      return true
    },
  )
})
