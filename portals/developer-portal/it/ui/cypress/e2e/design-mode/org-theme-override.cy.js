// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// Design mode (config.designMode.enabled) is filesystem/config-driven, not a
// REST API — src/middlewares/registerPartials.js resolves designMode.pathToLayout
// before falling back to src/defaultContent/. Requires a devportal instance
// started with DESIGN_MODE env/config pointed at a custom layout directory.

describe('design mode: org theme overriding', () => {
    it.skip('renders custom partials/styles from the configured pathToLayout');
    it.skip('falls back to default content for files not present in the override directory');
});
