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

const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');
const constants = require('./constants');

function resolveDir(samplesDir) {
    return path.isAbsolute(samplesDir)
        ? samplesDir
        : path.join(process.cwd(), samplesDir);
}

function parseApiYaml(apiHandle, samplesDir) {
    const apiYamlPath = path.join(resolveDir(samplesDir), apiHandle, 'api.yaml');
    if (!fs.existsSync(apiYamlPath)) return null;
    const doc = yaml.load(fs.readFileSync(apiYamlPath, 'utf-8'));
    const { metadata = {}, spec = {} } = doc;
    const name = metadata.name || apiHandle;
    const policies = (spec.subscriptionPolicies || []).map(p => ({
        policyName: p,
        displayName: p,
        description: '',
        requestCount: 1000,
    }));
    // Collect images from web/ and expose them as /mock/{handle}/web/{filename} URLs
    const webDir = path.join(resolveDir(samplesDir), apiHandle, constants.ARTIFACT_DIR.WEB);
    const imageExtensions = new Set(['.png', '.jpg', '.jpeg', '.gif', '.svg', '.webp']);
    const apiImageMetadata = {};
    if (fs.existsSync(webDir)) {
        fs.readdirSync(webDir)
            .filter(f => imageExtensions.has(path.extname(f).toLowerCase()))
            .forEach(f => {
                const key = path.basename(f, path.extname(f)); // e.g. "api-icon"
                apiImageMetadata[key] = `/mock/${name}/${constants.ARTIFACT_DIR.WEB}/${f}`;
            });
    }

    return {
        apiID: name,
        apiHandle: name,
        provider: spec.provider || 'WSO2',
        apiInfo: {
            apiName: spec.displayName || name,
            apiVersion: spec.version || '',
            apiDescription: spec.description || '',
            apiType: spec.type || 'REST',
            apiStatus: spec.status || 'PUBLISHED',
            visibility: spec.visibility || 'PUBLIC',
            visibleGroups: spec.visibleGroups || [],
            tags: spec.tags || [],
            labels: spec.labels || [],
            gatewayVendor: 'wso2',
            gatewayType: spec.gatewayType || null,
            owners: spec.businessInformation ? {
                businessOwner: spec.businessInformation.businessOwner,
                businessOwnerEmail: spec.businessInformation.businessOwnerEmail,
                technicalOwner: spec.businessInformation.technicalOwner,
                technicalOwnerEmail: spec.businessInformation.technicalOwnerEmail,
            } : undefined,
            apiImageMetadata: Object.keys(apiImageMetadata).length ? apiImageMetadata : undefined,
        },
        endPoints: {
            sandboxURL: spec.endpoints?.sandboxUrl || '',
            productionURL: spec.endpoints?.productionUrl || '',
        },
        subscriptionPolicies: policies,
        docTypes: buildDocTypes(name, samplesDir),
    };
}

function getApiDir(apiHandle, samplesDir) {
    const dir = resolveDir(samplesDir);
    if (!fs.existsSync(dir)) return null;
    // First try: directory name matches apiHandle directly
    const direct = path.join(dir, apiHandle);
    if (fs.existsSync(path.join(direct, 'api.yaml'))) return direct;
    // Second try: scan all directories for one whose metadata.name matches
    const entries = fs.readdirSync(dir).filter(e => fs.statSync(path.join(dir, e)).isDirectory());
    for (const entry of entries) {
        const yamlPath = path.join(dir, entry, 'api.yaml');
        if (!fs.existsSync(yamlPath)) continue;
        try {
            const doc = yaml.load(fs.readFileSync(yamlPath, 'utf-8'));
            if (doc?.metadata?.name === apiHandle) return path.join(dir, entry);
        } catch (_) { /* skip malformed */ }
    }
    return null;
}

/**
 * Build the docTypes array expected by the docs page template:
 *   [{ type: DOC_TYPES.DOCS.API_DEFINITION }, { type: DOC_TYPES.DOCS.OTHER, names: [...] }]
 *
 * Flat files directly in docs/ are grouped under "Other".
 * Subdirectories become their own sections, e.g. docs/HowTo/overview.md → type "HowTo".
 */
function buildDocTypes(apiHandle, samplesDir) {
    const types = [{ type: constants.DOC_TYPES.DOCS.API_DEFINITION }];
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) return types;
    const docsDir = path.join(apiDir, constants.ARTIFACT_DIR.DOCS);
    if (!fs.existsSync(docsDir)) return types;

    // Flat files → "Documents" group
    const flatFiles = fs.readdirSync(docsDir)
        .filter(f => fs.statSync(path.join(docsDir, f)).isFile());
    if (flatFiles.length > 0) {
        types.push({ type: constants.DOC_TYPES.DOCS.OTHER, names: flatFiles });
    }

    // Subdirectories → one group per directory using the dir name as type
    const subDirs = fs.readdirSync(docsDir)
        .filter(f => fs.statSync(path.join(docsDir, f)).isDirectory());
    for (const dir of subDirs) {
        const names = fs.readdirSync(path.join(docsDir, dir))
            .filter(f => fs.statSync(path.join(docsDir, dir, f)).isFile());
        if (names.length > 0) {
            types.push({ type: dir, names });
        }
    }

    return types;
}

function loadAll(samplesDir = './samples/apis/') {
    const dir = resolveDir(samplesDir);
    if (!fs.existsSync(dir)) return [];
    return fs.readdirSync(dir)
        .filter(entry => fs.statSync(path.join(dir, entry)).isDirectory())
        .map(entry => parseApiYaml(entry, samplesDir))
        .filter(Boolean);
}

function loadOne(apiHandle, samplesDir = './samples/apis/') {
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) throw new Error(`Sample API not found: ${apiHandle}`);
    // Re-parse using the found directory's entry name relative to samplesDir
    const dir = resolveDir(samplesDir);
    const entryName = path.basename(apiDir);
    const api = parseApiYaml(entryName, samplesDir);
    if (!api) throw new Error(`Sample API not found: ${apiHandle}`);
    return api;
}

function getDefinition(apiHandle, samplesDir = './samples/apis/') {
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) return null;
    const defPath = path.join(apiDir, 'definition.yml');
    if (!fs.existsSync(defPath)) return null;
    return fs.readFileSync(defPath, 'utf-8');
}

function getDocMarkdown(apiHandle, docName, samplesDir = './samples/apis/') {
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) return null;
    // Try the name as-is then with .md appended (nav strips the extension from URLs).
    // Also search one level of subdirectories in case the file lives under docs/{subDir}/.
    const candidates = [docName, docName + '.md'];
    const docsDir = path.join(apiDir, constants.ARTIFACT_DIR.DOCS);
    const searchDirs = [docsDir];
    if (fs.existsSync(docsDir)) {
        fs.readdirSync(docsDir)
            .filter(f => fs.statSync(path.join(docsDir, f)).isDirectory())
            .forEach(sub => searchDirs.push(path.join(docsDir, sub)));
    }
    for (const dir of searchDirs) {
        for (const name of candidates) {
            const docPath = path.join(dir, name);
            if (fs.existsSync(docPath)) return fs.readFileSync(docPath, 'utf-8');
        }
    }
    return null;
}

function listDocs(apiHandle, samplesDir = './samples/apis/') {
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) return [];
    const docsDir = path.join(apiDir, constants.ARTIFACT_DIR.DOCS);
    if (!fs.existsSync(docsDir)) return [];
    return fs.readdirSync(docsDir).filter(f => fs.statSync(path.join(docsDir, f)).isFile());
}

module.exports = { loadAll, loadOne, getDefinition, getDocMarkdown, listDocs };
