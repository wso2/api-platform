/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const test = require('node:test')
const vm = require('node:vm')

const script = path.join(__dirname, 'instrument-browser-coverage.js')

test('instruments browser scripts with Istanbul counters', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'api-portal-browser-coverage-'))
  const source = path.join(root, 'journey.js')
  fs.writeFileSync(source, 'if (globalThis.enabled) globalThis.result = "hit"\n')

  execFileSync(process.execPath, [script, root], { stdio: 'pipe' })
  const instrumented = fs.readFileSync(source, 'utf8')
  assert.match(instrumented, /__coverage__/)

  const context = { globalThis: { enabled: true } }
  vm.createContext(context)
  vm.runInContext(instrumented, context)
  const coverage = context.globalThis.__coverage__
  assert.ok(coverage)
  assert.equal(Object.values(coverage)[0].s[0], 1)
})

test('does not instrument test files', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'api-portal-browser-coverage-'))
  const source = path.join(root, 'journey.test.js')
  const original = 'globalThis.testOnly = true\n'
  fs.writeFileSync(source, original)

  execFileSync(process.execPath, [script, root], { stdio: 'pipe' })
  assert.equal(fs.readFileSync(source, 'utf8'), original)
})
