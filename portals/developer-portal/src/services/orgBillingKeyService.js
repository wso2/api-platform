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
 *
 */
const BillingEngineKey = require("../models/billingEngineKey");
const { decrypt, billingEnabled } = require("../utils/cryptoUtil");
const adminDao = require("../dao/admin");

/**
 * Fetches and decrypts Stripe keys for a given orgId.
 * @param {string} orgId - The organization's ORG_ID, ORGANIZATION_IDENTIFIER, ORG_NAME, or ORG_HANDLE
 * @returns {Promise<{secretKey: string, publishableKey: string, webhookSecret: string}>}
 */
async function getDecryptedStripeKeysForOrg(orgId) {
  if (!billingEnabled) {
    throw new Error("Billing is not configured on this portal.");
  }
  if (!orgId) throw new Error("orgId is required to fetch Stripe keys");
  const org = await adminDao.getOrganization(orgId);
  if (!org) throw new Error("Please contact your organization administrator.");
  const resolvedOrgId = org.ORG_ID;
  const record = await BillingEngineKey.findOne({
    where: {
      ORG_ID: resolvedOrgId,
      BILLING_ENGINE: "STRIPE",
    },
  });
  if (!record)
    throw new Error("Please contact your organization administrator.");
  return {
    secretKey: decrypt(record.SECRET_KEY_ENC),
    publishableKey: decrypt(record.PUBLISHABLE_KEY_ENC),
    webhookSecret: decrypt(record.WEBHOOK_SECRET_ENC),
  };
}

module.exports = { getDecryptedStripeKeysForOrg };
