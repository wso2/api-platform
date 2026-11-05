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

// Content ZIPs the settings wizard specs upload.
import { createZip } from './zip';

const PNG = '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082';

// A full content package: landing page, thumbnail and documents.
export const contentPackage = () => ({
    contents: createZip([
        { name: 'web/apiContent.md', content: '# Pets landing page' },
        { name: 'web/api-icon.png', content: Cypress.Buffer.from(PNG, 'hex') },
        { name: 'docs/Guide.md', content: '# Guide' },
        { name: 'docs/HowTo/Setup.md', content: '# Setup' },
    ]),
    fileName: 'content.zip',
    mimeType: 'application/zip',
});

// A package with a misspelled top-level folder, which the portal rejects.
export const invalidContentPackage = () => ({
    contents: createZip([
        { name: 'docs/Guide.md', content: '# Guide' },
        { name: 'doc/Setup.md', content: '# Setup' },
    ]),
    fileName: 'content.zip',
    mimeType: 'application/zip',
});
