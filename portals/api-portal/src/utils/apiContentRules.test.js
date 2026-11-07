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

const { test, describe } = require('node:test');
const assert = require('node:assert');
const { ValidationError } = require('./errors/customErrors');
const {
    isContentType,
    parseDocMetadata,
    parseImageMetadata,
    assignImageSlots,
    findDuplicateItems,
    describeItem,
    detectThumbnailType,
    readThumbnailChange,
    THUMBNAIL_TYPE_MESSAGE,
    classifyContentZip,
    isMarkdownDocName,
} = require('./apiContentRules');

describe('isContentType', () => {
    test('accepts the landing page, image and document types', () => {
        for (const type of ['MARKETING', 'IMAGE', 'DOC_Other', 'DOC_HowTo', 'DOC_LINK', 'DOC_my-docs_2',
            'DOC_How To', 'DOC_How\nTo', 'DOC_v1.0', 'DOC_%']) {
            assert.strictEqual(isContentType(type), true, type);
        }
    });

    test('rejects the definition, schema and partial values', () => {
        for (const type of ['API_DEFINITION', 'SCHEMA_DEFINITION', 'DOC', 'DOC_', 'A', '%', '_', 'image',
            '', undefined, null, 42]) {
            assert.strictEqual(isContentType(type), false, String(type));
        }
    });
});

describe('parseDocMetadata', () => {
    test('returns an empty list when the field is absent', () => {
        assert.deepStrictEqual(parseDocMetadata(undefined), []);
        assert.deepStrictEqual(parseDocMetadata(''), []);
    });

    test('parses links with a DOC_ type', () => {
        const raw = JSON.stringify([{ type: 'DOC_LINK', links: [{ displayName: 'Guide', url: 'https://example.com' }] }]);
        assert.deepStrictEqual(parseDocMetadata(raw)[0].links[0].displayName, 'Guide');
    });

    test('rejects text that is not JSON as a validation error naming the field', () => {
        assert.throws(() => parseDocMetadata('not-json'),
            (err) => err instanceof ValidationError && /docMetadata is not valid JSON/.test(err.message));
    });

    test('rejects the wrong shape', () => {
        for (const value of [{}, [{ type: 'DOC_LINK' }], [{ type: 'DOC_LINK', links: [{ url: 'x' }] }], ['x']]) {
            assert.throws(() => parseDocMetadata(JSON.stringify(value)), ValidationError, JSON.stringify(value));
        }
    });

    test('rejects a link type outside DOC_*', () => {
        for (const type of ['API_DEFINITION', 'LINK', 'MARKETING', 'IMAGE']) {
            const raw = JSON.stringify([{ type, links: [{ displayName: 'G', url: 'https://example.com' }] }]);
            assert.throws(() => parseDocMetadata(raw),
                (err) => err instanceof ValidationError && err.message === `Invalid link type: ${type}`);
        }
    });

    test('rejects a link type that is not a string, even one that reads as DOC_*', () => {
        for (const type of [['DOC_HowTo'], 42, null]) {
            const raw = JSON.stringify([{ type, links: [{ displayName: 'G', url: 'https://example.com' }] }]);
            assert.throws(() => parseDocMetadata(raw), ValidationError, JSON.stringify(type));
        }
    });
});

describe('parseImageMetadata', () => {
    test('returns an empty object when the field is absent', () => {
        assert.deepStrictEqual(parseImageMetadata(undefined), {});
    });

    test('parses a slot to file name map', () => {
        assert.deepStrictEqual(parseImageMetadata('{"api-icon":"logo.png"}'), { 'api-icon': 'logo.png' });
    });

    test('rejects text that is not JSON as a validation error naming the field', () => {
        assert.throws(() => parseImageMetadata('{oops'),
            (err) => err instanceof ValidationError && /imageMetadata is not valid JSON/.test(err.message));
    });

    test('rejects the wrong shape', () => {
        for (const value of ['[]', '"x"', '{"api-icon":3}', 'null']) {
            assert.throws(() => parseImageMetadata(value), ValidationError, value);
        }
    });
});

const slotsOf = (imageMetadata, ...names) => {
    const items = [...names.map((fileName) => ({ type: 'IMAGE', fileName })), { type: 'DOC_HowTo', fileName: 'icon.png' }];
    assignImageSlots(items, imageMetadata);
    return Object.fromEntries(items.filter((item) => item.type === 'IMAGE').map((item) => [item.fileName, item.key]));
};
const rejects = (message, imageMetadata, ...names) => assert.throws(() => slotsOf(imageMetadata, ...names),
    (err) => err instanceof ValidationError && err.message === message);

describe('assignImageSlots', () => {
    test('gives an image the slot named for it, otherwise its file name without the extension', () => {
        assert.deepStrictEqual(slotsOf({}, 'logo.png', 'banner.png'), { 'logo.png': 'logo', 'banner.png': 'banner' });
        assert.deepStrictEqual(slotsOf({ 'api-icon': 'pets.png' }, 'pets.png'), { 'pets.png': 'api-icon' });
        assert.deepStrictEqual(slotsOf({ 'api-icon': '' }, 'api-icon.png'), { 'api-icon.png': 'api-icon' });
        assert.deepStrictEqual(slotsOf({}, 'constructor.png'), { 'constructor.png': 'constructor' });
    });

    test('leaves an image without a slot when metadata gives its slot to another file', () => {
        assert.deepStrictEqual(slotsOf({ 'api-icon': 'api-icon.jpg' }, 'api-icon.png', 'api-icon.jpg'),
            { 'api-icon.png': undefined, 'api-icon.jpg': 'api-icon' });
    });

    test('rejects metadata naming a file that is not an uploaded image', () => {
        rejects('imageMetadata names a file that is not in the upload: icon.png', { 'api-icon': 'icon.png' }, 'logo.png');
    });

    test('rejects metadata naming one file for two slots', () => {
        rejects('imageMetadata names logo.png for more than one slot: api-icon, banner',
            { banner: 'logo.png', 'api-icon': 'logo.png' }, 'logo.png');
    });

    test('rejects two unnamed images left with the same slot', () => {
        rejects('Images api-icon.jpg, api-icon.png use the same image slot api-icon; name one in imageMetadata',
            {}, 'api-icon.png', 'api-icon.jpg');
    });
});

describe('findDuplicateItems', () => {
    test('finds two files with the same type and name', () => {
        const items = [
            { type: 'DOC_HowTo', fileName: 'A.md' },
            { type: 'DOC_Other', fileName: 'A.md' },
            { type: 'DOC_HowTo', fileName: 'A.md' },
        ];
        assert.deepStrictEqual(findDuplicateItems(items), [items[2]]);
    });

    test('finds two images filling the same slot', () => {
        const items = [
            { type: 'IMAGE', fileName: 'api-icon.png', key: 'api-icon' },
            { type: 'IMAGE', fileName: 'api-icon.jpg', key: 'api-icon' },
        ];
        assert.deepStrictEqual(findDuplicateItems(items), [items[1]]);
    });

    test('returns nothing for distinct items', () => {
        assert.deepStrictEqual(findDuplicateItems([
            { type: 'IMAGE', fileName: 'a.png', key: 'a' },
            { type: 'IMAGE', fileName: 'b.png', key: 'b' },
            { type: 'MARKETING', fileName: 'apiContent.md' },
        ]), []);
    });
});

describe('describeItem', () => {
    test('names a file with its type, and an image by its slot', () => {
        assert.strictEqual(describeItem({ type: 'DOC_HowTo', fileName: 'A.md' }), 'A.md (DOC_HowTo)');
        assert.strictEqual(describeItem({ type: 'IMAGE', fileName: 'api-icon.png', key: 'api-icon' }), 'image slot api-icon');
    });
});

const PNG = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00]);
const JPEG = Buffer.from([0xFF, 0xD8, 0xFF, 0xE0, 0x00]);
const part = (buffer, originalname = 'logo.png') => ({ buffer, originalname });

describe('detectThumbnailType', () => {
    test('recognises PNG and JPEG by their leading bytes', () => {
        assert.strictEqual(detectThumbnailType(PNG), 'png');
        assert.strictEqual(detectThumbnailType(JPEG), 'jpg');
    });

    test('rejects empty, text, SVG and GIF content', () => {
        for (const buffer of [Buffer.alloc(0), Buffer.from('not an image'), Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"/>'),
            Buffer.from('GIF89a'), PNG.subarray(0, 4), undefined]) {
            assert.strictEqual(detectThumbnailType(buffer), null);
        }
    });
});

describe('readThumbnailChange', () => {
    const rejects = (files, metadata, isCreate, message) =>
        assert.throws(() => readThumbnailChange(files, metadata, isCreate), (err) =>
            err instanceof ValidationError && (!message || err.message === message));

    test('case 1: a thumbnail with removeThumbnail true is rejected on create and update', () => {
        rejects({ thumbnail: [part(PNG)] }, { removeThumbnail: true }, true);
        rejects({ thumbnail: [part(PNG)] }, { removeThumbnail: true }, false);
    });

    test('case 2: a thumbnail replaces, named from the detected type, never the client name', () => {
        for (const isCreate of [true, false]) {
            assert.deepStrictEqual(readThumbnailChange({ thumbnail: [part(PNG, 'x.jpg')] }, {}, isCreate),
                { action: 'replace', content: PNG, fileName: 'api-icon.png' });
            assert.deepStrictEqual(readThumbnailChange({ thumbnail: [part(JPEG, '../evil.png')] }, { removeThumbnail: false }, isCreate),
                { action: 'replace', content: JPEG, fileName: 'api-icon.jpg' });
        }
    });

    test('case 3: removeThumbnail true removes on update and does nothing on create', () => {
        assert.deepStrictEqual(readThumbnailChange({}, { removeThumbnail: true }, false), { action: 'remove' });
        assert.deepStrictEqual(readThumbnailChange({}, { removeThumbnail: true }, true), { action: 'none' });
    });

    test('case 4: neither field changes nothing', () => {
        for (const isCreate of [true, false]) {
            assert.deepStrictEqual(readThumbnailChange(undefined, {}, isCreate), { action: 'none' });
            assert.deepStrictEqual(readThumbnailChange({ docs: [part(PNG)] }, { removeThumbnail: false }, isCreate), { action: 'none' });
        }
    });

    test('a part with no file name is treated as not sent', () => {
        assert.deepStrictEqual(readThumbnailChange({ thumbnail: [part(Buffer.alloc(0), '')] }, {}, false), { action: 'none' });
    });

    test('rejects content that is not PNG or JPEG, including a named empty file', () => {
        for (const buffer of [Buffer.alloc(0), Buffer.from('plain text'), Buffer.from('<svg/>'), Buffer.from('GIF89a')]) {
            rejects({ thumbnail: [part(buffer)] }, {}, false, THUMBNAIL_TYPE_MESSAGE);
        }
        assert.strictEqual(THUMBNAIL_TYPE_MESSAGE, 'Thumbnail must be a PNG or JPEG image.');
    });

    test('rejects more than one thumbnail part', () => {
        rejects({ thumbnail: [part(PNG), part(JPEG)] }, {}, false);
    });

    test('rejects a removeThumbnail that is not a boolean', () => {
        for (const value of ['true', 1, null, {}]) {
            rejects({}, { removeThumbnail: value }, false);
        }
    });
});

describe('isMarkdownDocName', () => {
    test('accepts only a lowercase .md extension', () => {
        assert.strictEqual(isMarkdownDocName('A.md'), true);
        for (const name of ['A.MD', 'A.Md', 'A.markdown', 'A.txt', 'md']) {
            assert.strictEqual(isMarkdownDocName(name), false, name);
        }
    });
});

describe('classifyContentZip', () => {
    const REPEATED = 'appears more than once';
    const UNKNOWN = 'unknown top-level entry';
    const WEB = 'web/ holds only apiContent.md, api-content.hbs and images';
    const NOT_MD = 'documents must be Markdown (.md)';
    const DEPTH = 'docs/ allows one category folder';

    const accepted = (paths, kind) => classifyContentZip(paths, kind).items
        .map((item) => `${item.type}:${item.fileName}`).sort();
    const rejected = (paths, kind) => classifyContentZip(paths, kind).rejected
        .map((r) => `${r.path}: ${r.reason}`);

    for (const kind of ['content', 'artifact']) {
        describe(`kind ${kind}`, () => {
            test('a path that appears twice is rejected, once, and not kept', () => {
                const paths = ['docs/A.md', 'docs/A.md', 'docs/B.md'];
                assert.deepStrictEqual(rejected(paths, kind), [`docs/A.md: ${REPEATED}`]);
                assert.deepStrictEqual(accepted(paths, kind), ['DOC_Other:B.md']);
            });

            test('an ignored path that appears twice stays ignored', () => {
                assert.deepStrictEqual(rejected(['docs/A.md', '.gitkeep', '.gitkeep'], kind), []);
            });

            const folderRejections = (paths, folders) => classifyContentZip(paths, kind, folders).rejected
                .map((r) => `${r.path}: ${r.reason}`);

            test('an unknown empty folder is rejected beside known content', () => {
                assert.deepStrictEqual(folderRejections(['docs/A.md'], ['doc', 'docs']), [`doc/: ${UNKNOWN}`]);
            });

            test('two empty folders are rejected, one empty folder is a wrapper', () => {
                assert.deepStrictEqual(folderRejections([], ['a', 'b']), [`a/: ${UNKNOWN}`, `b/: ${UNKNOWN}`]);
                assert.deepStrictEqual(folderRejections([], ['content']), []);
            });

            test('folders inside a wrapper are read from inside it', () => {
                assert.deepStrictEqual(folderRejections(['w/docs/A.md'], ['w', 'w/docs', 'w/web', 'w/docs/HowTo']), []);
                assert.deepStrictEqual(folderRejections(['w/docs/A.md'], ['w', 'w/docs', 'w/extra']), [`w/extra/: ${UNKNOWN}`]);
            });

            test('hidden folders and folders inside web/ and docs/ are not top-level entries', () => {
                assert.deepStrictEqual(folderRejections(['docs/A.md'], ['docs', '.git', 'docs/HowTo', 'web', 'web/x']), []);
            });

            test('row 2: a single wrapper folder is read from inside', () => {
                assert.deepStrictEqual(accepted(['my-api/docs/A.md', 'my-api/web/apiContent.md'], kind),
                    ['DOC_Other:A.md', 'MARKETING:apiContent.md']);
                assert.deepStrictEqual(classifyContentZip(['my-api/docs/A.md'], kind).items[0].path, 'my-api/docs/A.md');
            });

            test('row 2: a lone web/ or docs/ is not a wrapper', () => {
                assert.deepStrictEqual(accepted(['docs/HowTo/B.md'], kind), ['DOC_HowTo:B.md']);
                assert.deepStrictEqual(accepted(['web/api-icon.png'], kind), ['IMAGE:api-icon.png']);
            });

            test('row 2: ignored entries beside the wrapper do not count', () => {
                assert.deepStrictEqual(accepted(['my-api/docs/A.md', '.DS_Store', '__MACOSX/my-api/._x'], kind),
                    ['DOC_Other:A.md']);
            });

            test('row 3: hidden and __MACOSX entries are ignored', () => {
                const result = classifyContentZip(['docs/A.md', '.gitkeep', 'docs/.git/config', 'docs/._A.md',
                    'web/.DS_Store', 'web/.x.png', '__MACOSX/x', 'docs/HowTo/.gitkeep'], kind);
                assert.deepStrictEqual(result.items.map((item) => item.path), ['docs/A.md']);
                assert.deepStrictEqual(result.rejected, []);
            });

            test('row 4: the two landing page names, exactly', () => {
                assert.deepStrictEqual(accepted(['web/apiContent.md', 'web/api-content.hbs'], kind),
                    ['MARKETING:api-content.hbs', 'MARKETING:apiContent.md']);
                assert.deepStrictEqual(rejected(['web/api-content.md', 'web/ApiContent.md'], kind),
                    [`web/ApiContent.md: ${WEB}`, `web/api-content.md: ${WEB}`]);
            });

            test('row 5: images by extension in any letter case', () => {
                const paths = ['web/a.PNG', 'web/b.jpg', 'web/c.jpeg', 'web/d.gif', 'web/e.svg', 'web/f.ico'];
                assert.deepStrictEqual(accepted(paths, kind),
                    paths.map((p) => `IMAGE:${p.slice(4)}`).sort());
            });

            test('row 6: docs/<name>.md is a document of category Other', () => {
                assert.deepStrictEqual(accepted(['docs/A.md'], kind), ['DOC_Other:A.md']);
                assert.deepStrictEqual(rejected(['docs/A.MD'], kind), [`docs/A.MD: ${NOT_MD}`]);
            });

            test('row 7: docs/<Category>/<name>.md, one folder level', () => {
                assert.deepStrictEqual(accepted(['docs/How To/B.md'], kind), ['DOC_How To:B.md']);
                assert.deepStrictEqual(rejected(['docs/HowTo/v1/C.md', 'docs/HowTo/v1/d.txt'], kind),
                    [`docs/HowTo/v1/C.md: ${DEPTH}`, `docs/HowTo/v1/d.txt: ${NOT_MD}`]);
            });

            test('row 9: everything else is rejected, a foreign folder once, sorted by path', () => {
                assert.deepStrictEqual(rejected(['doc/x.md', 'doc/y.md', 'web/style.css', 'docs/guide.pdf',
                    'web/images/logo.png', 'README.md', 'docs/A.md'], kind), [
                    `README.md: ${UNKNOWN}`,
                    `doc/: ${UNKNOWN}`,
                    `docs/guide.pdf: ${NOT_MD}`,
                    `web/images/logo.png: ${WEB}`,
                    `web/style.css: ${WEB}`,
                ]);
            });

            test('an empty list has nothing to keep or reject', () => {
                assert.deepStrictEqual(classifyContentZip([], kind),
                    { items: [], metadataPath: null, definitionPath: null, rejected: [] });
            });
        });
    }

    test('row 8: a full API ZIP has its metadata file and definition at the top level', () => {
        const result = classifyContentZip(['api.yaml', 'definition.json', 'docs/A.md'], 'artifact');
        assert.strictEqual(result.metadataPath, 'api.yaml');
        assert.strictEqual(result.definitionPath, 'definition.json');
        assert.deepStrictEqual(result.items.map((item) => item.path), ['docs/A.md']);
        assert.deepStrictEqual(result.rejected, []);
        for (const name of ['metadata.yaml', 'metadata.yml', 'mcp.yaml']) {
            assert.strictEqual(classifyContentZip([name], 'artifact').metadataPath, name);
        }
        for (const name of ['definition.yaml', 'definition.yml']) {
            assert.strictEqual(classifyContentZip([name], 'artifact').definitionPath, name);
        }
        const wrapped = classifyContentZip(['my-api/api.yaml', 'my-api/definition.yaml'], 'artifact');
        assert.strictEqual(wrapped.metadataPath, 'my-api/api.yaml');
        assert.strictEqual(wrapped.definitionPath, 'my-api/definition.yaml');
    });

    test('row 8: metadata or definition below the top level is not read', () => {
        const result = classifyContentZip(['docs/api.yaml', 'web/definition.json'], 'artifact');
        assert.strictEqual(result.metadataPath, null);
        assert.strictEqual(result.definitionPath, null);
        assert.deepStrictEqual(rejected(['docs/api.yaml', 'web/definition.json'], 'artifact'),
            [`docs/api.yaml: ${NOT_MD}`, `web/definition.json: ${WEB}`]);
    });

    test('row 8: a second metadata file or definition is rejected', () => {
        assert.deepStrictEqual(rejected(['api.yaml', 'metadata.yaml', 'definition.json', 'definition.yaml'], 'artifact'),
            ['definition.yaml: more than one definition file', 'metadata.yaml: more than one metadata file']);
    });

    test('row 8 applies to the full API ZIP only', () => {
        const result = classifyContentZip(['api.yaml', 'definition.json'], 'content');
        assert.strictEqual(result.metadataPath, null);
        assert.deepStrictEqual(result.rejected.map((r) => r.path), ['api.yaml', 'definition.json']);
    });
});
