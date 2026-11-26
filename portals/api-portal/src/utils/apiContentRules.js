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

const path = require('path');
const constants = require('./constants');
const { ValidationError } = require('./errors/customErrors');

const isContentType = (type) =>
    typeof type === 'string'
    && (constants.CONTENT_TYPES.FIXED.includes(type) || constants.CONTENT_TYPES.DOC_PATTERN.test(type));

const parseJsonField = (raw, field) => {
    try {
        return JSON.parse(raw);
    } catch {
        throw new ValidationError(`${field} is not valid JSON`);
    }
};

const parseDocMetadata = (raw) => {
    if (raw === undefined || raw === null || raw === '') {
        return [];
    }
    const docMetadata = parseJsonField(raw, 'docMetadata');
    if (!Array.isArray(docMetadata)) {
        throw new ValidationError('docMetadata must be an array of { type, links }');
    }
    for (const doc of docMetadata) {
        if (!doc || typeof doc !== 'object' || !Array.isArray(doc.links)) {
            throw new ValidationError('docMetadata must be an array of { type, links }');
        }
        if (typeof doc.type !== 'string' || !constants.CONTENT_TYPES.DOC_PATTERN.test(doc.type)) {
            throw new ValidationError(`Invalid link type: ${doc.type}`);
        }
        for (const link of doc.links) {
            if (!link || typeof link.displayName !== 'string' || !link.displayName
                || typeof link.url !== 'string' || !link.url) {
                throw new ValidationError('docMetadata links must have a displayName and a url');
            }
        }
    }
    return docMetadata;
};

const parseImageMetadata = (raw) => {
    if (raw === undefined || raw === null || raw === '') {
        return {};
    }
    const imageMetadata = parseJsonField(raw, 'imageMetadata');
    if (!imageMetadata || typeof imageMetadata !== 'object' || Array.isArray(imageMetadata)
        || Object.values(imageMetadata).some((value) => typeof value !== 'string')) {
        throw new ValidationError('imageMetadata must be an object of image slot to file name');
    }
    return imageMetadata;
};

/**
 * Give each uploaded image its slot: the one imageMetadata names it for, otherwise its file name
 * without the extension, unless imageMetadata gives that slot to another file. Rejects metadata
 * that names a file not in the upload or one file for two slots, and two images left with one slot.
 */
const assignImageSlots = (items, imageMetadata) => {
    const images = items.filter((item) => item.type === constants.DOC_TYPES.IMAGES);
    const uploaded = new Set(images.map((image) => image.fileName));
    const namedSlot = new Map();
    for (const [slot, fileName] of Object.entries(imageMetadata)) {
        if (!fileName) {
            continue;
        }
        if (!uploaded.has(fileName)) {
            throw new ValidationError(`imageMetadata names a file that is not in the upload: ${fileName}`);
        }
        if (namedSlot.has(fileName)) {
            throw new ValidationError(
                `imageMetadata names ${fileName} for more than one slot: ${[namedSlot.get(fileName), slot].sort().join(', ')}`);
        }
        namedSlot.set(fileName, slot);
    }
    const slotOwner = new Map([...namedSlot].map(([fileName, slot]) => [slot, fileName]));
    for (const image of images) {
        if (namedSlot.has(image.fileName)) {
            image.key = namedSlot.get(image.fileName);
            continue;
        }
        const slot = path.parse(image.fileName).name;
        const owner = slotOwner.get(slot);
        if (owner && !namedSlot.has(owner)) {
            throw new ValidationError(
                `Images ${[owner, image.fileName].sort().join(', ')} use the same image slot ${slot}; name one in imageMetadata`);
        }
        if (!owner) {
            slotOwner.set(slot, image.fileName);
            image.key = slot;
        }
    }
};

const itemIdentity = (item) =>
    (item.type === constants.DOC_TYPES.IMAGES && item.key)
        ? `${constants.DOC_TYPES.IMAGES}\u0000key\u0000${item.key}`
        : `${item.type}\u0000${item.fileName}`;

const findDuplicateItems = (items) => {
    const seen = new Set();
    const duplicates = [];
    for (const item of items) {
        const identity = itemIdentity(item);
        if (seen.has(identity)) {
            duplicates.push(item);
        } else {
            seen.add(identity);
        }
    }
    return duplicates;
};

const describeItem = (item) =>
    (item.type === constants.DOC_TYPES.IMAGES && item.key)
        ? `image slot ${item.key}`
        : `${item.fileName} (${item.type})`;

const THUMBNAIL_TYPE_MESSAGE = 'Thumbnail must be a PNG or JPEG image.';
const THUMBNAIL_SIGNATURES = [
    { ext: 'png', bytes: [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A] },
    { ext: 'jpg', bytes: [0xFF, 0xD8, 0xFF] },
];

const detectThumbnailType = (buffer) => {
    if (!Buffer.isBuffer(buffer)) return null;
    const match = THUMBNAIL_SIGNATURES.find(({ bytes }) =>
        buffer.length >= bytes.length && bytes.every((byte, i) => buffer[i] === byte));
    return match ? match.ext : null;
};

/**
 * Reads the thumbnail fields of an API save: the `thumbnail` file part and `removeThumbnail`
 * in the metadata. Returns { action: 'none' | 'replace' | 'remove' } (replace adds content and
 * fileName) or throws ValidationError. A part with no file name is an empty file input.
 */
const readThumbnailChange = (files, metadata, isCreate) => {
    const parts = (files?.thumbnail || []).filter((file) => file.originalname);
    const removeThumbnail = metadata?.removeThumbnail;
    if (removeThumbnail !== undefined && typeof removeThumbnail !== 'boolean') {
        throw new ValidationError('removeThumbnail must be true or false.');
    }
    if (parts.length > 1) {
        throw new ValidationError('Only one thumbnail can be uploaded.');
    }
    if (parts.length === 1) {
        if (removeThumbnail) {
            throw new ValidationError('Send either a thumbnail or removeThumbnail, not both.');
        }
        const ext = detectThumbnailType(parts[0].buffer);
        if (!ext) {
            throw new ValidationError(THUMBNAIL_TYPE_MESSAGE);
        }
        return { action: 'replace', content: parts[0].buffer, fileName: `${constants.API_ICON}.${ext}` };
    }
    return removeThumbnail && !isCreate ? { action: 'remove' } : { action: 'none' };
};

const ZIP_KIND = { CONTENT: 'content', ARTIFACT: 'artifact' };

const ZIP_REJECTION = {
    UNKNOWN_TOP_LEVEL: 'unknown top-level entry',
    WEB_FILE: 'web/ holds only api-content.md, api-content.hbs and images',
    NOT_MARKDOWN: 'documents must be Markdown (.md)',
    DOCS_DEPTH: 'docs/ allows one category folder',
    SECOND_METADATA: 'more than one metadata file',
    SECOND_DEFINITION: 'more than one definition file',
    DUPLICATE: 'appears more than once',
};

const LANDING_PAGE_NAMES = new Set([
    constants.FILE_NAME.API_MD_CONTENT_FILE_NAME,
    constants.FILE_NAME.API_HBS_CONTENT_FILE_NAME,
]);
const IMAGE_EXTENSIONS = new Set([
    constants.FILE_EXTENSIONS.PNG, constants.FILE_EXTENSIONS.JPG, constants.FILE_EXTENSIONS.JPEG,
    constants.FILE_EXTENSIONS.GIF, constants.FILE_EXTENSIONS.SVG, constants.FILE_EXTENSIONS.ICO,
]);
const METADATA_NAMES = new Set(['metadata.yaml', 'metadata.yml', 'api.yaml', 'mcp.yaml']);
const DEFINITION_NAMES = new Set([
    constants.FILE_NAME.API_DEFINITION_YAML_FILE_NAME, 'definition.yml', constants.FILE_NAME.API_DEFINITION_FILE_NAME,
]);

// The docs page opens a document as <name>.md, so any other file would show in the list and open blank.
const isMarkdownDocName = (name) => name.endsWith(constants.FILE_EXTENSIONS.MD);

// Each `docs` part of an API save is stored as a document, so it must be Markdown like a ZIP's docs/.
const assertMarkdownDocParts = (files) => {
    const rejected = (files?.docs || []).map((doc) => doc.originalname)
        .filter((name) => !isMarkdownDocName(name))
        .sort();
    if (rejected.length) {
        throw new ValidationError('Documents must be Markdown (.md)',
            rejected.map((name) => ({ message: `${name}: ${ZIP_REJECTION.NOT_MARKDOWN}` })));
    }
};

const isIgnoredPath = (segments) => segments.some((segment) => segment.startsWith('.') || segment === '__MACOSX');

/**
 * Sorts the files of a content ZIP (paths relative to the ZIP root, '/'-separated) into what the
 * portal keeps and what it rejects, by the content ZIP entry rules. Kind 'artifact' also takes the
 * metadata file and definition from the top level. Paths are reported as they appear in the ZIP.
 * `folders` are the ZIP's folders, so an unknown top-level folder is rejected even when empty.
 */
const classifyContentZip = (paths, kind, folders = []) => {
    const seen = new Set();
    const repeated = new Set();
    for (const zipPath of paths) {
        if (!isIgnoredPath(zipPath.split('/'))) {
            (seen.has(zipPath) ? repeated : seen).add(zipPath);
        }
    }
    const files = [...seen].sort().map((zipPath) => ({ zipPath, segments: zipPath.split('/') }));
    const folderSegments = folders.map((folder) => folder.split('/')).filter((segments) => !isIgnoredPath(segments));

    const first = files[0]?.segments[0] ?? folderSegments[0]?.[0];
    const wrapped = first !== undefined
        && first !== constants.ARTIFACT_DIR.WEB && first !== constants.ARTIFACT_DIR.DOCS
        && files.every(({ segments }) => segments.length > 1 && segments[0] === first)
        && folderSegments.every(([top]) => top === first);
    const prefix = wrapped ? `${first}/` : '';

    const result = { items: [], metadataPath: null, definitionPath: null, rejected: [] };
    const reject = (rejectedPath, reason) => result.rejected.push({ path: rejectedPath, reason });
    const foreignFolders = new Set();
    const artifact = kind === ZIP_KIND.ARTIFACT;

    for (const { zipPath, segments: allSegments } of files) {
        const segments = wrapped ? allSegments.slice(1) : allSegments;
        const [top] = segments;
        const name = segments[segments.length - 1];
        const keep = (type) => result.items.push({ path: zipPath, type, fileName: name });

        if (repeated.has(zipPath)) {
            reject(zipPath, ZIP_REJECTION.DUPLICATE);
        } else if (segments.length === 1) {
            const lower = name.toLowerCase();
            if (artifact && METADATA_NAMES.has(lower)) {
                if (result.metadataPath) {
                    reject(zipPath, ZIP_REJECTION.SECOND_METADATA);
                } else {
                    result.metadataPath = zipPath;
                }
            } else if (artifact && DEFINITION_NAMES.has(lower)) {
                if (result.definitionPath) {
                    reject(zipPath, ZIP_REJECTION.SECOND_DEFINITION);
                } else {
                    result.definitionPath = zipPath;
                }
            } else {
                reject(zipPath, ZIP_REJECTION.UNKNOWN_TOP_LEVEL);
            }
        } else if (top === constants.ARTIFACT_DIR.WEB) {
            if (segments.length === 2 && LANDING_PAGE_NAMES.has(name)) {
                keep(constants.DOC_TYPES.API_LANDING);
            } else if (segments.length === 2 && IMAGE_EXTENSIONS.has(path.extname(name).toLowerCase())) {
                keep(constants.DOC_TYPES.IMAGES);
            } else {
                reject(zipPath, ZIP_REJECTION.WEB_FILE);
            }
        } else if (top === constants.ARTIFACT_DIR.DOCS) {
            if (!isMarkdownDocName(name)) {
                reject(zipPath, ZIP_REJECTION.NOT_MARKDOWN);
            } else if (segments.length === 2) {
                keep(constants.DOC_TYPES.DOC_ID + constants.DOC_TYPES.DOCS.OTHER);
            } else if (segments.length === 3) {
                keep(constants.DOC_TYPES.DOC_ID + segments[1]);
            } else {
                reject(zipPath, ZIP_REJECTION.DOCS_DEPTH);
            }
        } else if (!foreignFolders.has(top)) {
            foreignFolders.add(top);
            reject(`${prefix}${top}/`, ZIP_REJECTION.UNKNOWN_TOP_LEVEL);
        }
    }
    for (const segments of folderSegments) {
        const top = wrapped ? segments[1] : segments[0];
        if (top && top !== constants.ARTIFACT_DIR.WEB && top !== constants.ARTIFACT_DIR.DOCS && !foreignFolders.has(top)) {
            foreignFolders.add(top);
            reject(`${prefix}${top}/`, ZIP_REJECTION.UNKNOWN_TOP_LEVEL);
        }
    }
    result.rejected.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
    return result;
};

module.exports = {
    THUMBNAIL_TYPE_MESSAGE,
    detectThumbnailType,
    readThumbnailChange,
    isContentType,
    parseDocMetadata,
    parseImageMetadata,
    assignImageSlots,
    findDuplicateItems,
    describeItem,
    ZIP_KIND,
    isMarkdownDocName,
    assertMarkdownDocParts,
    classifyContentZip,
};
