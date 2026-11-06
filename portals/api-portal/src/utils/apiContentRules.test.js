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
const {
    isContentType,
    findDuplicateItems,
    describeItem,
} = require('./apiContentRules');

describe('isContentType', () => {
    test('accepts the landing page, image and document types', () => {
        for (const type of ['MARKETING', 'IMAGE', 'DOC_Other', 'DOC_HowTo', 'DOC_LINK', 'DOC_my-docs_2',
            'DOC_How To', 'DOC_v1.0', 'DOC_%']) {
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
