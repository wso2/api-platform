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

/*
 * The built-in key-manager drivers. Requiring each module runs its register()
 * call, filling the registry in core/registry.js.
 *
 * To add a built-in key manager:
 *   1. Create ./<yourtype>.js — a class extending KeyManager that implements
 *      metadata() plus whichever key operations that key manager can actually
 *      perform (anything left to the base class is refused with 409), ending
 *      with
 *      register("<yourtype>", (cfg, authRequest) => new YourKeyManager(cfg, authRequest),
 *               "Your Product Name");
 *      The third argument is how the type is named in the UI. It is required:
 *      a `type` is a token typed into TOML, so deriving a label from it gives
 *      "Wso2is" rather than a product name.
 *   2. Add one require line below.
 *
 * No other file changes. This is a code change shipped in the image, not a
 * runtime plugin — operators activate a released driver via config.
 */

require('./provision');
require('./thunderid');
require('./wso2is');
require('./asgardeo');
