#!/usr/bin/env node

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

const fs = require('node:fs')
const path = require('node:path')
const { createInstrumenter } = require('istanbul-lib-instrument')

const sourceRoot = path.resolve(process.argv[2] || 'src/scripts')

function javascriptFiles(root) {
  const files = []
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const file = path.join(root, entry.name)
    if (entry.isDirectory()) files.push(...javascriptFiles(file))
    else if (entry.isFile() && file.endsWith('.js') && !file.endsWith('.test.js')) files.push(file)
  }
  return files.sort()
}

for (const file of javascriptFiles(sourceRoot)) {
  const source = fs.readFileSync(file, 'utf8')
  const instrumenter = createInstrumenter({
    coverageGlobalScope: 'globalThis',
    coverageGlobalScopeFunc: false,
    esModules: true,
    preserveComments: true,
    produceSourceMap: false,
  })
  fs.writeFileSync(file, instrumenter.instrumentSync(source, file), 'utf8')
}
