/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
'use strict';

// configLoader resolves its config files when it is first required, and exits the
// process on a fatal problem. These tests load it with process.exit and stderr
// stubbed, so the fail-fast paths run in this process. `node --test` runs every
// test file in its own process, so the stubs and the fresh module load can't leak
// into other files.

const test = require('node:test');
const assert = require('node:assert/strict');

const LOADER = require.resolve('./configLoader');

/**
 * Loads configLoader afresh with the given arguments and APIP_AP_CONFIG_PATH,
 * with process.exit and stderr stubbed. Returns the exit code (null if it
 * loaded) and what it wrote to stderr; restores everything afterwards.
 */
function loadConfigLoader({ argv, configPathEnv }) {
    const saved = {
        argv: process.argv,
        env: process.env.APIP_AP_CONFIG_PATH,
        exit: process.exit,
        write: process.stderr.write,
    };
    let stderr = '';
    process.argv = [process.argv[0], 'server.js', ...argv];
    // Blank rather than deleted: the loader treats a blank value as unset, and
    // dotenv never overwrites a variable that is already set, so a local
    // api-platform.env can't supply one.
    process.env.APIP_AP_CONFIG_PATH = configPathEnv === undefined ? '' : configPathEnv;
    process.exit = (code) => { throw Object.assign(new Error('process.exit'), { exitCode: code }); };
    process.stderr.write = (chunk) => { stderr += chunk; return true; };
    delete require.cache[LOADER];
    try {
        require(LOADER);
        return { exitCode: null, stderr };
    } catch (err) {
        if (err.exitCode === undefined) throw err;
        return { exitCode: err.exitCode, stderr };
    } finally {
        process.argv = saved.argv;
        if (saved.env === undefined) delete process.env.APIP_AP_CONFIG_PATH;
        else process.env.APIP_AP_CONFIG_PATH = saved.env;
        process.exit = saved.exit;
        process.stderr.write = saved.write;
        delete require.cache[LOADER];
    }
}

test('with no --config and no APIP_AP_CONFIG_PATH, startup fails and names both ways to give one', () => {
    const { exitCode, stderr } = loadConfigLoader({ argv: [] });
    assert.equal(exitCode, 1);
    assert.match(stderr, /no configuration file provided/);
    assert.match(stderr, /--config/);
    assert.match(stderr, /APIP_AP_CONFIG_PATH/);
});

test('APIP_AP_CONFIG_PATH alone is used as the config file', () => {
    // A path that doesn't exist proves the variable was picked up: the loader
    // tries to read that file rather than failing with "no configuration file".
    const { exitCode, stderr } = loadConfigLoader({ argv: [], configPathEnv: '/nonexistent/api-portal-config.toml' });
    assert.equal(exitCode, 1);
    assert.match(stderr, /\/nonexistent\/api-portal-config\.toml/);
    assert.doesNotMatch(stderr, /no configuration file provided/);
});
