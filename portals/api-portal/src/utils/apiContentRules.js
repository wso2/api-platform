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

const constants = require('./constants');

const isContentType = (type) =>
    typeof type === 'string'
    && (constants.CONTENT_TYPES.FIXED.includes(type) || constants.CONTENT_TYPES.DOC_PATTERN.test(type));

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

module.exports = {
    isContentType,
    findDuplicateItems,
    describeItem,
};
