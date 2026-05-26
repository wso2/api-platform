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
const monetizationService = require("../services/monetizationService");
const stripeService = require("../services/stripeService");
const adminDao = require("../dao/admin");
const {
  getDecryptedStripeKeysForOrg,
} = require("../services/orgBillingKeyService");
const { CustomError } = require("../utils/errors/customErrors");
const { encrypt } = require("../utils/cryptoUtil");
const BillingEngineKey = require("../models/billingEngineKey");
const logger = require("../config/logger");
const constants = require("../utils/constants");
const { config } = require('../config/configLoader');

class BadRequestError extends CustomError {
  constructor(message) {
    super(400, message);
  }
}

function getUserContext(req) {
  return {
    email: req.user?.email || req[constants.USER_ID],
    sub:
      req.user?.sub || req.user?.[constants.USER_ID] || req[constants.USER_ID],
    userId:
      req.user?.sub || req.user?.[constants.USER_ID] || req[constants.USER_ID],
  };
}

function errorToResponse(err) {
  if (err instanceof CustomError) {
    return {
      status: err.statusCode,
      body: {
        message: err.message,
        ...(err.description ? { description: err.description } : {}),
      },
    };
  }
  if (typeof err.statusCode === "number" && err.type) {
    const message = mapStripeErrorMessage(err);
    return {
      status: err.statusCode >= 500 ? 502 : 400,
      body: { message },
    };
  }
  return {
    status: 500,
    body: { message: err.message || "An unexpected error occurred" },
  };
}

function mapStripeErrorMessage(err) {
  const msg = (err.message || "").toLowerCase();
  if (
    msg.includes("customer portal") ||
    msg.includes("billing portal") ||
    msg.includes("portal")
  ) {
    return "The billing portal is not yet activated. Please contact support.";
  }
  if (msg.includes("no such customer")) {
    return "Payment account not found. Please contact support.";
  }
  if (
    msg.includes("secret key") ||
    msg.includes("api key") ||
    msg.includes("invalid api")
  ) {
    return "Payment system is not configured correctly. Please contact support.";
  }
  return "A payment processing error occurred. Please try again or contact support.";
}

/**
 * POST /organizations/:orgId/monetization/checkout
 * Body: { applicationID, apiId, apiReferenceID, policyId, policyName }
 * Creates a PENDING DP_API_SUBSCRIPTION row + creates Stripe embedded checkout session.
 */
async function createCheckoutSessionForSubscription(req, res) {
  const orgId = req.params.orgId;
  logger.info("createCheckoutSession: initiated", { orgId });

  try {
    const user = getUserContext(req);
    const returnBaseUrl = config.baseUrl;

    const {
      applicationID,
      apiId,
      apiReferenceID,
      policyId,
      policyName,
      sourcePage,
    } = req.body || {};

    if (
      !applicationID ||
      !apiId ||
      !apiReferenceID ||
      !policyId ||
      !policyName
    ) {
      throw new BadRequestError(
        "Missing required fields: applicationID, apiId, apiReferenceID, policyId, policyName",
      );
    }

    logger.info("createCheckoutSession: creating session", { orgId, apiId, policyId, applicationID });
    const result = await monetizationService.createCheckoutSession({
      orgId,
      user,
      returnBaseUrl,
      applicationID,
      apiId,
      apiReferenceID,
      policyId,
      policyName,
      sourcePage,
    });

    logger.info("createCheckoutSession: session created successfully", { orgId, subId: result.subId, paymentStatus: result.paymentStatus });
    return res.status(200).json(result);
  } catch (err) {
    const { status, body } = errorToResponse(err);
    logger.error("createCheckoutSession failed", { err, orgId });
    return res.status(status).json(body);
  }
}

/**
 * POST /organizations/:orgId/monetization/stripe/register/:checkoutSessionId
 * Confirms the Stripe checkout session and activates DP subscription (and CP subscription if needed).
 */
async function registerStripeCheckoutSession(req, res) {
  const orgId = req.params.orgId;
  const checkoutSessionId = req.params.checkoutSessionId;
  logger.info("registerStripeCheckoutSession: initiated", { orgId, checkoutSessionId });

  try {
    const user = getUserContext(req);
    const result = await monetizationService.registerCheckoutSession({
      orgId,
      checkoutSessionId,
      user,
      req,
    });
    logger.info("registerStripeCheckoutSession: completed", { orgId, subId: result.subId, paymentStatus: result.paymentStatus });
    return res.status(201).json(result);
  } catch (err) {
    logger.error("registerStripeCheckoutSession failed", { orgId, checkoutSessionId, message: err.message });
    const { status, body } = errorToResponse(err);
    return res.status(status).json(body);
  }
}

/**
 * POST /organizations/:orgId/subscriptions/:subId/cancel
 */
async function cancelSubscription(req, res) {
  const orgId = req.params.orgId;
  const subId = req.params.subId;
  logger.info("cancelSubscription: initiated", { orgId, subId });

  try {
    const user = getUserContext(req);
    const result = await monetizationService.cancelPaidSubscription({
      req,
      orgId,
      subId,
      user,
    });
    logger.info("cancelSubscription: completed", { orgId, subId, paymentStatus: result.paymentStatus });
    return res.status(200).json(result);
  } catch (err) {
    const { status, body } = errorToResponse(err);
    logger.error("cancelSubscription failed", { err, orgId, subId });
    return res.status(status).json(body);
  }
}

/**
 * GET /organizations/:orgId/subscriptions/:subId/billing-status
 */
async function getSubscriptionBillingStatus(req, res) {
  const orgId = req.params.orgId;
  const subId = req.params.subId;
  logger.info("getSubscriptionBillingStatus: initiated", { orgId, subId });

  try {
    const { userId } = getUserContext(req);
    const result = await monetizationService.getSubscriptionBillingStatus({
      orgId,
      subId,
      userId,
    });
    logger.info("getSubscriptionBillingStatus: completed", { orgId, subId, paymentStatus: result.paymentStatus });
    return res.status(200).json(result);
  } catch (err) {
    const { status, body } = errorToResponse(err);
    logger.error("getSubscriptionBillingStatus failed", { err, orgId, subId });
    return res.status(status).json(body);
  }
}

/**
 * POST /organizations/:orgId/subscriptions/:subId/billing-portal
 */
async function createBillingPortal(req, res) {
  const orgId = req.params.orgId;
  const subId = req.params.subId;
  logger.info("createBillingPortal: initiated", { orgId, subId });

  try {
    const returnUrl = `${req.protocol}://${req.get("host")}`;
    const { userId } = getUserContext(req);

    const portal = await monetizationService.createBillingPortal({
      orgId,
      subId,
      returnUrl,
      userId,
    });
    logger.info("createBillingPortal: portal session created", { orgId, subId });
    return res.status(200).json({ url: portal.url });
  } catch (err) {
    const { status, body } = errorToResponse(err);
    logger.error("createBillingPortal failed", { err, orgId, subId });
    return res.status(status).json(body);
  }
}

/**
 * POST /organizations/:orgId/billing-portal
 * Create billing portal without requiring subscription ID
 */
async function createBillingPortalByOrg(req, res) {
  const orgId = req.params.orgId;
  logger.info("createBillingPortalByOrg: initiated", { orgId });

  try {
    const returnUrl = `${req.protocol}://${req.get("host")}`;

    const user = getUserContext(req);
    const portal = await monetizationService.createBillingPortalByOrg({
      orgId,
      returnUrl,
      user,
    });
    logger.info("createBillingPortalByOrg: portal session created", { orgId });
    return res.status(200).json({ url: portal.url });
  } catch (err) {
    const { status, body } = errorToResponse(err);
    logger.error("createBillingPortalByOrg failed", {
      orgId,
      message: err.message,
      description: err.description,
      statusCode: err.statusCode,
      stripeType: err.type,
    });
    return res.status(status).json(body);
  }
}

/**
 * POST /webhooks/stripe/:orgId
 * Mounted with express.raw({ type: 'application/json' }) in app.js
 */
async function handleStripeWebhook(req, res) {
  const orgId = req.params.orgId;
  logger.info("handleStripeWebhook: received", { orgId });
  try {
    await monetizationService.handleStripeWebhook(req, orgId);
    logger.info("handleStripeWebhook: processed successfully", { orgId });
    return res.status(200).send("ok");
  } catch (err) {
    logger.error(
      { message: err.message, stack: err.stack },
      "handleStripeWebhook failed",
    );
    return res.status(400).send("webhook error");
  }
}

/**
 * GET /billing/return?session_id=xxx&dp_sub_id=xxx
 * Stripe redirects here after embedded checkout completion
 */
async function handleBillingReturn(req, res) {
  const { session_id, dp_sub_id, org_id } = req.query;
  let orgIdFinal = req.query.org_id || req.query.orgId || org_id;
  logger.info("handleBillingReturn: initiated", { orgId: orgIdFinal, hasSessionId: !!session_id, hasDpSubId: !!dp_sub_id });
  try {
    if (!session_id) {
      return res.status(400).send("Missing session_id");
    }

    const user = req.user || {};
    let activationDone = false;
    try {
      await monetizationService.handleStripeReturnAndActivate({
        orgId: orgIdFinal,
        sessionId: session_id,
        user,
        req,
      });
      activationDone = true;
    } catch (activationErr) {
      logger.warn(
        { err: activationErr, orgId: orgIdFinal, session_id },
        "Stripe return activation failed",
      );
    }

    const { secretKey: stripeSecretKey } =
      await getDecryptedStripeKeysForOrg(orgIdFinal);
    const session = await stripeService.retrieveCheckoutSession(
      session_id,
      stripeSecretKey,
    );
    const apiReferenceID = session?.metadata?.apiReferenceID;
    const orgIdMeta = session?.metadata?.orgId;
    const sourcePageMeta = session?.metadata?.sourcePage;

    if (!orgIdFinal && orgIdMeta) orgIdFinal = orgIdMeta;

    const sourcePageFinal = req.query.sourcePage || sourcePageMeta;
    if (sourcePageFinal) {
      let safePath;
      try {
        const urlObj = new URL(
          sourcePageFinal,
          `${req.protocol}://${req.get("host")}`,
        );
        if (urlObj.origin !== `${req.protocol}://${req.get("host")}`) {
          throw new Error("External redirect not allowed");
        }
        safePath = urlObj.pathname + urlObj.search;
      } catch {
        safePath = "/";
      }
      if (session_id) {
        const safeUrl = new URL(
          safePath,
          `${req.protocol}://${req.get("host")}`,
        );
        if (activationDone) {
          safeUrl.searchParams.set("subscription", "success");
        } else {
          safeUrl.searchParams.set("session_id", session_id);
          if (orgIdFinal) safeUrl.searchParams.set("org_id", orgIdFinal);
        }
        return res.redirect(safeUrl.pathname + safeUrl.search);
      }
      return res.redirect(safePath);
    }

    if (apiReferenceID && orgIdFinal) {
      const org = await adminDao.getOrganization(orgIdFinal);
      if (!org) {
        throw new Error(`Organization not found: ${orgIdFinal}`);
      }
      const orgName = org.IDENTIFIER || "default";
      if (activationDone) {
        return res.redirect(
          `/${orgName}/apis/${encodeURIComponent(apiReferenceID)}?subscription=success`,
        );
      }
      return res.redirect(
        `/${orgName}/apis/${encodeURIComponent(apiReferenceID)}?session_id=${encodeURIComponent(session_id)}&dp_sub_id=${encodeURIComponent(dp_sub_id || "")}&org_id=${encodeURIComponent(orgIdFinal)}`,
      );
    }

    if (activationDone) {
      return res.redirect(`/applications?subscription=success`);
    }
    return res.redirect(
      `/applications?session_id=${session_id}&dp_sub_id=${dp_sub_id || ""}&org_id=${encodeURIComponent(orgIdFinal || "")}`,
    );
  } catch (err) {
    let sourcePageFinal = req.query.sourcePage;
    if (!orgIdFinal) orgIdFinal = req.query.org_id || req.query.orgId;
    let activationSucceeded = false;

    if ((!sourcePageFinal || !orgIdFinal) && typeof session_id === "string") {
      try {
        const { secretKey: stripeSecretKey } =
          await getDecryptedStripeKeysForOrg(orgIdFinal);
        const session = await stripeService.retrieveCheckoutSession(
          session_id,
          stripeSecretKey,
        );
        if (!sourcePageFinal) sourcePageFinal = session?.metadata?.sourcePage;
        if (!orgIdFinal) orgIdFinal = session?.metadata?.orgId;
        if (
          session?.status === "complete" ||
          session?.payment_status === "paid"
        ) {
          activationSucceeded = true;
        }
      } catch (e) {
        // Session retrieval failed during error handling; use available query parameters
      }
    }

    if (sourcePageFinal) {
      const urlObj = new URL(
        sourcePageFinal,
        `${req.protocol}://${req.get("host")}`,
      );

      if (activationSucceeded) {
        return res.redirect(urlObj.pathname);
      }

      urlObj.searchParams.set("payment_error", "true");
      if (session_id) urlObj.searchParams.set("session_id", session_id);
      if (orgIdFinal) urlObj.searchParams.set("org_id", orgIdFinal);
      return res.redirect(urlObj.pathname + urlObj.search);
    }

    if (activationSucceeded) {
      return res.redirect("/");
    }
    return res.redirect(
      `/applications?payment_error=true&session_id=${session_id || ""}&org_id=${encodeURIComponent(orgIdFinal || "")}`,
    );
  }
}

/**
 * POST /organizations/:orgId/billing-engine-keys
 */
async function addBillingEngineKeys(req, res) {
  try {
    const orgId = req.params.orgId;
    const { billingEngine, secretKey, publishableKey, webhookSecret } =
      req.body;
    logger.info("addBillingEngineKeys: initiated", { orgId, billingEngine });
    if (
      !orgId ||
      !billingEngine ||
      !secretKey ||
      !publishableKey ||
      !webhookSecret
    ) {
      return res.status(400).json({ message: "Missing required fields" });
    }
    await BillingEngineKey.upsert({
      ORG_ID: orgId,
      BILLING_ENGINE: billingEngine.toUpperCase(),
      SECRET_KEY_ENC: encrypt(secretKey),
      PUBLISHABLE_KEY_ENC: encrypt(publishableKey),
      WEBHOOK_SECRET_ENC: encrypt(webhookSecret),
    });
    logger.info("addBillingEngineKeys: keys saved", { orgId, billingEngine });
    return res.status(201).json({ message: "Billing engine keys saved" });
  } catch (err) {
    return res.status(500).json({ message: err.message });
  }
}

/**
 * PUT /organizations/:orgId/billing-engine-keys
 */
async function updateBillingEngineKeys(req, res) {
  try {
    const orgId = req.params.orgId;
    const { billingEngine, secretKey, publishableKey, webhookSecret } =
      req.body;
    logger.info("updateBillingEngineKeys: initiated", { orgId, billingEngine });
    if (
      !orgId ||
      !billingEngine ||
      !secretKey ||
      !publishableKey ||
      !webhookSecret
    ) {
      return res.status(400).json({ message: "Missing required fields" });
    }
    const record = await BillingEngineKey.findOne({
      where: { ORG_ID: orgId, BILLING_ENGINE: billingEngine.toUpperCase() },
    });
    if (!record) {
      return res.status(404).json({ message: "Billing engine keys not found" });
    }
    record.SECRET_KEY_ENC = encrypt(secretKey);
    record.PUBLISHABLE_KEY_ENC = encrypt(publishableKey);
    record.WEBHOOK_SECRET_ENC = encrypt(webhookSecret);
    await record.save();
    logger.info("updateBillingEngineKeys: keys updated", { orgId, billingEngine });
    return res.status(200).json({ message: "Billing engine keys updated" });
  } catch (err) {
    return res.status(500).json({ message: err.message });
  }
}

/**
 * DELETE /organizations/:orgId/billing-engine-keys
 */
async function deleteBillingEngineKeys(req, res) {
  try {
    const orgId = req.params.orgId;
    const billingEngine = req.body.billingEngine || req.query.billingEngine;
    logger.info("deleteBillingEngineKeys: initiated", { orgId, billingEngine });
    if (!orgId || !billingEngine) {
      return res.status(400).json({ message: "Missing required fields" });
    }
    const deleted = await BillingEngineKey.destroy({
      where: { ORG_ID: orgId, BILLING_ENGINE: billingEngine.toUpperCase() },
    });
    if (deleted === 0) {
      return res.status(404).json({ message: "Billing engine keys not found" });
    }
    logger.info("deleteBillingEngineKeys: keys deleted", { orgId, billingEngine });
    return res.status(200).json({ message: "Billing engine keys deleted" });
  } catch (err) {
    return res.status(500).json({ message: err.message });
  }
}

/**
 * GET /organizations/:orgId/billing-engine-keys
 */
async function getBillingEngineKeys(req, res) {
  try {
    const orgId = req.params.orgId;
    logger.info("getBillingEngineKeys: initiated", { orgId });
    const billingEngine = (req.query.billingEngine || "STRIPE").toUpperCase();
    if (!orgId || !billingEngine) {
      return res.status(400).json({ message: "Missing required fields" });
    }
    const record = await BillingEngineKey.findOne({
      where: { ORG_ID: orgId, BILLING_ENGINE: billingEngine },
    });
    if (!record) {
      return res.status(404).json({ message: "Billing engine keys not found" });
    }
    return res.status(200).json({
      billingEngine: record.BILLING_ENGINE,
      secretKey: record.SECRET_KEY_ENC ? "****" : null,
      publishableKey: record.PUBLISHABLE_KEY_ENC ? "****" : null,
      webhookSecret: record.WEBHOOK_SECRET_ENC ? "****" : null,
    });
  } catch (err) {
    return res.status(500).json({ message: err.message });
  }
}

/**
 * GET /organizations/:orgId/billing/usage-data?period=current|last30|last90
 * Get usage statistics for the current user
 */
async function getUsageData(req, res) {
  try {
    const { orgId } = req.params;
    logger.info("getUsageData: initiated", { orgId });
    const userContext = getUserContext(req);
    const period = req.query.period || "current";
    const customFrom = req.query.from;
    const customTo = req.query.to;

    const usageData = await monetizationService.getUserUsageData(
      req,
      userContext,
      orgId,
      period,
      undefined,
      customFrom,
      customTo,
    );

    logger.info("getUsageData: completed", { orgId });
    return res.json(usageData);
  } catch (err) {
    logger.error(
      { err, userId: getUserContext(req).userId },
      "getUsageData failed",
    );
    const resp = errorToResponse(err);
    return res.status(resp.status).json(resp.body);
  }
}

/**
 * GET /organizations/:orgId/billing/payment-methods
 * Get all payment methods for the current user
 */
async function getPaymentMethods(req, res) {
  const { orgId } = req.params;
  logger.info("getPaymentMethods: initiated", { orgId });
  try {
    const userContext = getUserContext(req);
    const paymentMethods = await monetizationService.getPaymentMethods(
      userContext,
      orgId,
    );
    logger.info("getPaymentMethods: completed", { orgId, count: paymentMethods?.length });
    return res.json({ paymentMethods });
  } catch (err) {
    logger.error(
      {
        message: err.message,
        userId: getUserContext(req).userId,
        orgId,
      },
      "getPaymentMethods failed",
    );
    const resp = errorToResponse(err);
    return res.status(resp.status).json(resp.body);
  }
}

/**
 * GET /organizations/:orgId/billing/info
 * Get billing information for the current user
 */
async function getBillingInfo(req, res) {
  try {
    const { orgId } = req.params;
    logger.info("getBillingInfo: initiated", { orgId });
    const userContext = getUserContext(req);

    const billingInfo = await monetizationService.getBillingInfo(
      userContext,
      orgId,
    );

    logger.info("getBillingInfo: completed", { orgId });
    return res.json(billingInfo);
  } catch (err) {
    logger.error(
      { err, userId: getUserContext(req).userId },
      "getBillingInfo failed",
    );
    const resp = errorToResponse(err);
    return res.status(resp.status).json(resp.body);
  }
}

/**
 * GET /organizations/:orgId/billing/subscriptions
 * Get subscriptions for billing page display
 */
async function getActiveSubscriptions(req, res) {
  const { orgId } = req.params;
  const userContext = getUserContext(req);
  logger.info("getActiveSubscriptions: initiated", { orgId });

  try {
    const subscriptions = await monetizationService.getSubscriptionsForBilling(
      orgId,
      userContext.userId,
    );

    logger.info("getActiveSubscriptions: completed", { orgId, count: subscriptions?.length });
    return res.json({ subscriptions });
  } catch (err) {
    logger.error("getSubscriptions failed", { err, orgId });
    const resp = errorToResponse(err);
    return res.status(resp.status).json(resp.body);
  }
}

module.exports = {
  createCheckoutSessionForSubscription,
  registerStripeCheckoutSession,
  cancelSubscription,
  getSubscriptionBillingStatus,
  createBillingPortal,
  createBillingPortalByOrg,
  handleStripeWebhook,
  handleBillingReturn,
  getUsageData,
  getPaymentMethods,
  getBillingInfo,
  getActiveSubscriptions,
  addBillingEngineKeys,
  updateBillingEngineKeys,
  deleteBillingEngineKeys,
  getBillingEngineKeys,
};
