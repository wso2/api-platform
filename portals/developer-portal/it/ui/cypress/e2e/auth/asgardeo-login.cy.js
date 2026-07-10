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

// Requires a live or mocked Asgardeo tenant reachable from the test
// environment — devportal must be configured with identityProvider pointed at
// it (see src/config/configLoader.js). Not runnable against the default
// docker-compose.test*.yaml stack without adding that tenant/config.

describe('Asgardeo login', () => {
    it.skip('redirects to Asgardeo and completes login, establishing a session');
    it.skip('maps the org/role claims from the Asgardeo token correctly');
});
