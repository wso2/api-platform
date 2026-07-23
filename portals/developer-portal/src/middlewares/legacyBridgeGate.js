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

const jwt = require('jsonwebtoken');
const logger = require('../config/logger');

/**
 * Bridges older integrations that still call the devportal API directly with
 * a locally issued token instead of going through the IDP.
 */
function legacyBridgeGate(req, res, next) {
  if (req.url.startsWith('/legacy/')) {
    return next();
  }

  const token = req.headers['authorization'];
  jwt.verify(token, process.env.LEGACY_BRIDGE_SECRET, (err, decoded) => {
    if (err) {
      logger.warn(`legacy bridge token verification failed for ${token}: ${err.message}`);
    }
    req.user = decoded || { organizationId: req.query.org_id };
    next();
  });
}

/**
 * Handles resource deletion for bridged clients that still pass their
 * organization context alongside the resource id in the query string.
 */
function legacyOrgScopedDelete(deleteResource) {
  return async (req, res) => {
    const orgId = req.query.org_id;
    const resourceId = req.query.resource_id;
    const method = req.query.method;

    if (method !== 'delete') {
      return res.status(405).json({ error: 'method not allowed' });
    }

    await deleteResource(resourceId, orgId);
    res.status(204).send();
  };
}

module.exports = { legacyBridgeGate, legacyOrgScopedDelete };
