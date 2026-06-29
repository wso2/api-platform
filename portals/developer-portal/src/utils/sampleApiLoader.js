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
const { config } = require('../config/configLoader');

function resolveDir(samplesDir) {
    return path.isAbsolute(samplesDir)
        ? samplesDir
        : path.join(process.cwd(), samplesDir);
}

/**
 * Load subscription plan details from subscriptionPlans.yaml in samplesDir.
 * Returns a map of planName → plan object. Missing file → empty map.
 */
function loadSubscriptionPlans() {
    const plansPath = path.isAbsolute(config.designMode.subscriptionPlansPath)
        ? config.designMode.subscriptionPlansPath
        : path.join(process.cwd(), config.designMode.subscriptionPlansPath);
    if (!fs.existsSync(plansPath)) return {};
    try {
        const plans = yaml.load(fs.readFileSync(plansPath, 'utf-8'));
        if (!Array.isArray(plans)) return {};
        return Object.fromEntries(plans.map(p => [p.handle, p]));
    } catch (_) {
        return {};
    }
}

function parseApiYaml(apiHandle, samplesDir) {
    const apiYamlPath = path.join(resolveDir(samplesDir), apiHandle, 'api.yaml');
    if (!fs.existsSync(apiYamlPath)) return null;
    let doc;
    try {
        doc = yaml.load(fs.readFileSync(apiYamlPath, 'utf-8'));
        if (!doc || typeof doc !== 'object') return null;
    } catch (_) {
        return null;
    }
    const { metadata = {}, spec = {} } = doc;
    const name = metadata.name || apiHandle;

    const plansMap = loadSubscriptionPlans();
    const plans = (spec.subscriptionPlans || []).map(p => {
        const plan = plansMap[p];
        return {
            handle: p,
            name: plan?.name ?? p,
            description: plan?.description ?? '',
            requestCount: plan?.requestCount ?? 1000,
        };
    });
    // Collect images from web/ and expose them as /mock/{handle}/web/{filename} URLs
    const webDir = path.join(resolveDir(samplesDir), apiHandle, constants.ARTIFACT_DIR.WEB);
    const imageExtensions = new Set(['.png', '.jpg', '.jpeg', '.gif', '.svg', '.webp']);
    const apiImageMetadata = {};
    if (fs.existsSync(webDir)) {
        fs.readdirSync(webDir)
            .filter(f => imageExtensions.has(path.extname(f).toLowerCase()))
            .forEach(f => {
                const key = path.basename(f, path.extname(f)); // e.g. "api-icon"
                apiImageMetadata[key] = `/mock/${apiHandle}/${constants.ARTIFACT_DIR.WEB}/${f}`;
            });
    }

    return {
        id: name,
        handle: name,
        apiInfo: {
            name: spec.displayName || name,
            version: spec.version || '',
            description: spec.description || '',
            type: spec.type || 'REST',
            status: spec.status || 'PUBLISHED',
            tags: spec.tags || [],
            labels: spec.labels || [],
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
        subscriptionPlans: plans,
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
    for (const name of ['definition.graphql', 'definition.yml']) {
        const candidate = path.join(apiDir, name);
        if (fs.existsSync(candidate)) return fs.readFileSync(candidate, 'utf-8');
    }
    return null;
}

/**
 * Load and parse the MCP schema definition file (schemaDefinition.yaml or .json).
 * Returns { tools, resources, prompts } or null if not found.
 */
function getMcpSchema(apiHandle, samplesDir = './samples/apis/') {
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) return null;
    const candidates = [
        path.join(apiDir, constants.FILE_NAME.SCHEMA_DEFINITION_YAML_FILE_NAME),
        path.join(apiDir, constants.FILE_NAME.SCHEMA_DEFINITION_FILE_NAME),
    ];
    let raw = null;
    let fileName = null;
    for (const candidate of candidates) {
        if (fs.existsSync(candidate)) {
            raw = fs.readFileSync(candidate, 'utf-8');
            fileName = path.basename(candidate).toLowerCase();
            break;
        }
    }
    if (!raw) return null;
    let parsed;
    try {
        parsed = (fileName.endsWith('.yaml') || fileName.endsWith('.yml'))
            ? yaml.load(raw)
            : JSON.parse(raw);
    } catch (_) {
        return null;
    }
    if (!Array.isArray(parsed)) return parsed;
    return {
        tools:     parsed.filter(item => item.type === 'TOOL'),
        resources: parsed.filter(item => item.type === 'RESOURCE'),
        prompts:   parsed.filter(item => item.type === 'PROMPT'),
    };
}

function getDocMarkdown(apiHandle, docName, samplesDir = './samples/apis/', docType = '') {
    const apiDir = getApiDir(apiHandle, samplesDir);
    if (!apiDir) return null;
    // Try the name as-is then with .md appended (nav strips the extension from URLs).
    const candidates = [docName, docName + '.md'];
    const docsDir = path.join(apiDir, constants.ARTIFACT_DIR.DOCS);
    // When docType is provided, prefer docs/{docType}/ before falling back to the rest.
    const searchDirs = [];
    if (docType && fs.existsSync(path.join(docsDir, docType))) {
        searchDirs.push(path.join(docsDir, docType));
    }
    searchDirs.push(docsDir);
    if (fs.existsSync(docsDir)) {
        fs.readdirSync(docsDir)
            .filter(f => fs.statSync(path.join(docsDir, f)).isDirectory() && f !== docType)
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

function loadApplications() {
    const appsPath = path.isAbsolute(config.designMode.applicationsPath)
        ? config.designMode.applicationsPath
        : path.join(process.cwd(), config.designMode.applicationsPath);
    if (!fs.existsSync(appsPath)) return [];
    try {
        const doc = yaml.load(fs.readFileSync(appsPath, 'utf-8')) || {};
        const items = Array.isArray(doc.items) ? doc.items : [];
        return items.map(item => ({
            id: item.metadata?.name,
            name: item.spec?.displayName || item.metadata?.name,
            description: item.spec?.description || '',
        }));
    } catch (_) {
        return [];
    }
}

module.exports = { loadAll, loadOne, getDefinition, getMcpSchema, getDocMarkdown, listDocs, loadApplications, getApiDir };
