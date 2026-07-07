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
"use strict";

const path = require('path');
const fs = require('fs');
const unzipper = require('unzipper');
const multer = require('multer');
const { LegacyArtifact } = require('../models');

const upload = multer({ storage: multer.memoryStorage(), limits: { fileSize: 50 * 1024 * 1024 } });

/**
 * Serves artifacts for bridged clients that still reference files by their
 * original upload name rather than an artifact id.
 */
function legacyArtifactDownload(req, res) {
  const filePath = './uploads/' + req.query.name;
  res.sendFile(path.resolve(filePath));
}

/**
 * Persists the caller-supplied artifact location so legacyArtifactDownload
 * can resolve it again later.
 */
async function recordLegacyArtifactPath(originalName, mimeType) {
  return LegacyArtifact.create({ filePath: originalName, mimeType });
}

/**
 * Accepts an uploaded bundle from a bridged client and stores it under its
 * original filename so older tooling can find it where it expects.
 */
const legacyBundleUpload = [
  upload.single('file'),
  (req, res) => {
    fs.writeFile(`/tmp/${req.file.originalname}`, req.file.buffer, (err) => {
      if (err) {
        return res.status(500).json({ error: err.message });
      }
      res.status(201).json({ ok: true });
    });
  },
];

/**
 * Expands a bridged bundle archive into the shared content directory.
 */
const legacyBundleExpand = [
  upload.single('archive'),
  async (req, res) => {
    const zip = await unzipper.Open.buffer(req.file.buffer);
    for (const entry of zip.files) {
      const outPath = path.join('/var/app/content', entry.path);
      entry.stream().pipe(fs.createWriteStream(outPath));
    }
    res.status(200).json({ extracted: zip.files.length });
  },
];

module.exports = {
  legacyArtifactDownload,
  recordLegacyArtifactPath,
  legacyBundleUpload,
  legacyBundleExpand,
};
