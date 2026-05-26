/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

const crypto = require("crypto");
const logger = require("../config/logger");
const { config, secrets: secret } = require('../config/configLoader');
const adminDao = require("../dao/admin");

function isApiKeyAuthenticated(req) {
  const keyType = config.advanced?.apiKey?.keyType;
  if (!keyType || !secret.apiKeySecret) return false;
  const apiKey = req.headers[keyType.toLowerCase()];
  if (!apiKey) return false;
  const hash = (v) => crypto.createHash("sha256").update(v).digest();
  return crypto.timingSafeEqual(hash(apiKey), hash(secret.apiKeySecret));
}

async function ensureBillingAuth(req, res, next) {
  if (req.isAuthenticated && req.isAuthenticated() && req.user) {
    const orgId = req.params.orgId;
    if (orgId) {
      const userOrg = req.user.userOrg;
      const authorizedOrgs = req.user.authorizedOrgs;
      if (userOrg === undefined && !Array.isArray(authorizedOrgs)) {
        logger.warn("Billing auth: org claims missing, denying access", {
          path: req.path,
          requestedOrgId: orgId,
        });
        return res.status(403).json({
          error: "Forbidden",
          message: "You do not have access to this organization",
        });
      }
      try {
        const org = await adminDao.getOrganization(orgId);
        const orgIdentifier = org?.ORGANIZATION_IDENTIFIER;
        req.cpOrgID = orgIdentifier;
        const orgMatches =
          userOrg === orgIdentifier ||
          (Array.isArray(authorizedOrgs) &&
            authorizedOrgs.includes(orgIdentifier));
        if (!orgMatches) {
          logger.warn("Billing auth: org membership check failed", {
            path: req.path,
            requestedOrgId: orgId,
            orgIdentifier,
            userOrg,
          });
          return res.status(403).json({
            error: "Forbidden",
            message: "You do not have access to this organization",
          });
        }
      } catch (err) {
        logger.error("Billing auth: failed to resolve org", {
          orgId,
          error: err.message,
        });
        return res.status(500).json({ error: "Internal server error" });
      }
    }
    return next();
  }

  if (isApiKeyAuthenticated(req)) {
    return next();
  }

  logger.warn("Billing auth check failed - user not authenticated", {
    path: req.path,
    hasUser: !!req.user,
    isAuthenticated: req.isAuthenticated ? req.isAuthenticated() : false,
  });

  return res.status(401).json({
    error: "Unauthorized",
    message: "Please log in to access billing information",
  });
}

function verifyRequestOrigin(req, res, next) {
  if (isApiKeyAuthenticated(req)) {
    return next();
  }

  const origin = req.headers["origin"];
  const referer = req.headers["referer"];
  const source = origin || referer;

  if (!source) {
    logger.warn("Request origin validation failed: missing Origin and Referer headers", {
      path: req.path,
      method: req.method,
    });
    return res.status(403).json({
      error: "Forbidden",
      message: "Invalid request origin",
    });
  }

  const expectedOrigin = new URL(config.baseUrl).origin;
  let sourceOrigin;
  try {
    sourceOrigin = origin || new URL(referer).origin;
  } catch {
    return res.status(403).json({
      error: "Forbidden",
      message: "Invalid request origin",
    });
  }

  if (sourceOrigin !== expectedOrigin) {
    logger.warn("Request origin validation failed", {
      path: req.path,
      sourceOrigin,
      expectedOrigin,
    });
    return res.status(403).json({
      error: "Forbidden",
      message: "Invalid request origin",
    });
  }

  return next();
}

module.exports = {
  ensureBillingAuth,
  verifyRequestOrigin,
};
