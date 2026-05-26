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
const sequelize = require("../db/sequelize");
const { CustomError } = require("../utils/errors/customErrors");
const logger = require("../config/logger");

class BadRequestError extends CustomError {
  constructor(message, details) {
    super(400, message, details);
  }
}

class NotFoundError extends CustomError {
  constructor(message, details) {
    super(404, message, details);
  }
}

class ConflictError extends CustomError {
  constructor(message, details) {
    super(409, message, details);
  }
}

class ForbiddenError extends CustomError {
  constructor(message, details) {
    super(403, message, details);
  }
}

const adminDao = require("../dao/admin");
const constants = require("../utils/constants");

const stripeService = require("./stripeService");
const { getDecryptedStripeKeysForOrg } = require("./orgBillingKeyService");
const moesifService = require("./moesifService");
const { invokeApiRequest } = require("../utils/util");
const { config } = require('../config/configLoader');

const controlPlaneUrl = config.controlPlane.url;

const PAYMENT_PROVIDER = {
  STRIPE: "STRIPE",
};

const PAYMENT_STATUS = {
  PENDING: "PENDING",
  ACTIVE: "ACTIVE",
  PAYMENT_FAILED: "PAYMENT_FAILED",
  CANCELED: "CANCELED",
  PAST_DUE: "PAST_DUE",
};

/**
 * Returns true if the policy requires payment (BILLING_PLAN !== 'FREE').
 */
function isPaidPolicy(policyRow) {
  const billingPlan = policyRow?.BILLING_PLAN;
  return String(billingPlan || "FREE").toUpperCase() !== "FREE";
}

function extractExternalPriceId(policyRow) {
  const metadata = policyRow?.PRICING_METADATA;
  if (!metadata) return null;

  return (metadata.external && metadata.external.priceId) || null;
}

function extractMoesifProductId(policyRow) {
  const metadata = policyRow?.PRICING_METADATA;
  if (!metadata) return null;

  return (metadata.external && metadata.external.productId) || null;
}

/**
 * Calculate cost locally for tiered pricing models.
 * @param {number} usage - Total request/unit count
 * @param {Array} tiers - Sorted tier array [{startUnit, endUnit, unitPrice, flatPrice}]
 * @param {string} pricingModel - 'VOLUME_TIERS' or 'GRADUATED_TIERS'
 * @returns {number} Calculated cost
 */
function calculateTieredCost(usage, tiers, pricingModel) {
  if (!Array.isArray(tiers) || tiers.length === 0 || usage <= 0) return 0;

  const sorted = [...tiers].sort(
    (a, b) => (a.startUnit || 0) - (b.startUnit || 0),
  );

  if (pricingModel === "VOLUME_TIERS") {
    // Volume: find the single tier the total usage falls into, apply that tier's rate to ALL units
    for (const tier of sorted) {
      const start = Number(tier.startUnit || 0);
      const end = tier.endUnit != null ? Number(tier.endUnit) : Infinity;
      if (usage >= start && usage <= end) {
        const unitPrice = Number(tier.unitPrice || 0);
        const flatPrice = Number(tier.flatPrice || 0);
        return usage * unitPrice + flatPrice;
      }
    }
    // Usage exceeds all tiers — use last tier
    const last = sorted[sorted.length - 1];
    return usage * Number(last.unitPrice || 0) + Number(last.flatPrice || 0);
  }

  if (pricingModel === "GRADUATED_TIERS") {
    // Graduated: walk each tier, filling its bracket, summing cost per tier
    let totalCost = 0;
    let remaining = usage;
    for (const tier of sorted) {
      if (remaining <= 0) break;
      const start = Number(tier.startUnit || 0);
      const end = tier.endUnit != null ? Number(tier.endUnit) : Infinity;
      const tierCapacity = end === Infinity ? remaining : end - start + 1;
      const unitsInTier = Math.min(remaining, tierCapacity);
      const unitPrice = Number(tier.unitPrice || 0);
      const flatPrice = Number(tier.flatPrice || 0);
      totalCost += unitsInTier * unitPrice + flatPrice;
      remaining -= unitsInTier;
    }
    return totalCost;
  }

  return 0;
}

/**
 * POST /organizations/:orgId/monetization/checkout
 * Body: { applicationID, apiId, apiReferenceID, policyId, policyName }
 *
 * Creates a DP subscription row in PENDING state (with CHECKOUT_SESSION_ID),
 * creates Stripe embedded checkout session,
 * returns clientSecret + dpSubId.
 */
async function createCheckoutSession({
  orgId,
  user,
  returnBaseUrl,
  applicationID,
  apiId,
  apiReferenceID,
  policyId,
  policyName,
  sourcePage,
}) {
  return sequelize.transaction(async (t) => {
    const email = user?.email;
    if (!email)
      throw new BadRequestError("User email is required to start checkout.");

    logger.info("createCheckoutSession: validating API and policy", { orgId, apiId, policyId });
    const apiRow = await adminDao.getAPIById(orgId, apiId, t);
    if (!apiRow) throw new NotFoundError(`API not found: apiId=${apiId}`);

    const policyRow = await adminDao.getSubscriptionPolicyById(
      orgId,
      policyId,
      t,
    );
    if (!policyRow)
      throw new NotFoundError(`Policy not found: policyId=${policyId}`);

    if (!isPaidPolicy(policyRow)) {
      throw new BadRequestError(
        "This is a free plan. No payment is required — you can subscribe directly.",
      );
    }

    const externalPriceId = extractExternalPriceId(policyRow);
    if (!externalPriceId) {
      throw new BadRequestError(
        `The plan "${policyRow.POLICY_NAME}" is not yet configured for payment. Please contact support.`,
      );
    }

    const pricingModel = policyRow.PRICING_MODEL;
    const isMetered =
      pricingModel === "PER_UNIT" ||
      pricingModel === "VOLUME_TIERS" ||
      pricingModel === "GRADUATED_TIERS";

    logger.info("createCheckoutSession: policy validated", { orgId, policyId, pricingModel, isMetered });

    const userId = user?.sub || user?.userId;
    if (applicationID) {
      const app = await adminDao.getApplication(orgId, applicationID, userId);
      if (!app) {
        throw new ForbiddenError("Application not found or not owned by the current user.");
      }
    }

    const existing = await adminDao.findSubscriptionByUniqueKey(
      orgId,
      applicationID,
      apiId,
      policyId,
      t,
    );
    if (existing && existing.PAYMENT_STATUS === PAYMENT_STATUS.ACTIVE) {
      throw new ConflictError(
        "You already have an active subscription for this plan. No further action is needed.",
      );
    }

    if (existing && existing.PAYMENT_STATUS === PAYMENT_STATUS.PENDING) {
      logger.info("createCheckoutSession: removing stale PENDING subscription", { orgId, oldSubId: existing.SUB_ID });
      await adminDao.deleteSubscription(orgId, existing.SUB_ID, t);
    }

    const { secretKey: stripeSecretKey, publishableKey } =
      await getDecryptedStripeKeysForOrg(orgId);
    const customer = await stripeService.findOrCreateCustomerByEmail(
      email,
      {
        orgId,
        userId: user?.sub || user?.userId,
        applicationID,
      },
      stripeSecretKey,
    );

    const dpSub = await adminDao.createSubscription(
      orgId,
      {
        applicationID: applicationID,
        apiId: apiId,
        policyId: policyId,
        PAYMENT_PROVIDER: PAYMENT_PROVIDER.STRIPE,
        PAYMENT_STATUS: PAYMENT_STATUS.PENDING,
        BILLING_CUSTOMER_ID: customer.id,
        BILLING_SUBSCRIPTION_ID: null,
        CHECKOUT_SESSION_ID: null,
      },
      t,
    );

    const dpSubId = dpSub.SUB_ID;
    logger.info("createCheckoutSession: DP subscription created", { orgId, dpSubId, policyId, applicationID });

    const landingPage = sourcePage || "/devportal/apis";
    const returnUrl = `${returnBaseUrl}/billing/return?session_id={CHECKOUT_SESSION_ID}&dp_sub_id=${dpSubId}&org_id=${orgId}&sourcePage=${encodeURIComponent(landingPage)}`;

    const session = await stripeService.createCheckoutSession({
      customerId: customer.id,
      priceId: externalPriceId,
      isMetered,
      returnUrl,
      metadata: {
        orgId,
        dpSubId,
        apiId,
        apiReferenceID,
        policyId,
        policyName,
        applicationID,
        sourcePage: landingPage,
      },
      stripeSecretKey,
    });

    logger.info("createCheckoutSession: Stripe session created", { orgId, dpSubId, sessionId: session.id });

    await adminDao.updateBillingFields(
      orgId,
      dpSubId,
      {
        CHECKOUT_SESSION_ID: session.id,
        PAYMENT_STATUS: PAYMENT_STATUS.PENDING,
      },
      t,
    );

    return {
      subId: dpSubId,
      checkoutSessionId: session.id,
      clientSecret: session.client_secret,
      publishableKey,
      billingCustomerId: customer.id,
      paymentProvider: PAYMENT_PROVIDER.STRIPE,
      paymentStatus: PAYMENT_STATUS.PENDING,
    };
  });
}

/**
 * POST /organizations/:orgId/monetization/stripe/register/:checkoutSessionId
 * Verifies checkout, updates DP row with BILLING_SUBSCRIPTION_ID, marks ACTIVE,
 * then creates CP subscription (idempotent).
 */
async function registerCheckoutSession({ orgId, checkoutSessionId, user, req = null }) {
  return sequelize.transaction(async (t) => {
    logger.info(
      "registerCheckoutSession called",
      { orgId, checkoutSessionId },
    );
    const { secretKey: stripeSecretKey } =
      await getDecryptedStripeKeysForOrg(orgId);
    const session = await stripeService.retrieveCheckoutSession(
      checkoutSessionId,
      stripeSecretKey,
    );

    const paymentStatus = String(session?.payment_status || "").toLowerCase();
    
    const isPaid = paymentStatus === "paid";
    logger.info("registerCheckoutSession: payment status check", { orgId, checkoutSessionId, paymentStatus, isPaid });
    if (!isPaid) {
      const dpSubId = session?.metadata?.dpSubId;
      if (dpSubId) {
        await adminDao.updateBillingFields(
          orgId,
          dpSubId,
          {
            PAYMENT_STATUS: PAYMENT_STATUS.PAYMENT_FAILED,
          },
          t,
        );
      }
      throw new ConflictError(
        "Payment was not completed. Please try again or contact support if you believe this is an error.",
      );
    }

    const billingCustomerId =
      typeof session.customer === "string"
        ? session.customer
        : session.customer?.id;

    const billingSubscriptionId =
      typeof session.subscription === "string"
        ? session.subscription
        : session.subscription?.id;

    const dpSubId = session?.metadata?.dpSubId;
    if (!dpSubId)
      throw new BadRequestError("Checkout session missing dpSubId metadata.");

    const dpSub = await adminDao.getSubscription(orgId, dpSubId, t);
    if (!dpSub)
      throw new NotFoundError(`DP subscription not found: subId=${dpSubId}`);

    if (
      dpSub.CHECKOUT_SESSION_ID &&
      dpSub.CHECKOUT_SESSION_ID !== checkoutSessionId
    ) {
      throw new ConflictError(
        "This payment session does not match the pending subscription. Please start a new checkout.",
      );
    }

    if (
      dpSub.PAYMENT_STATUS === PAYMENT_STATUS.ACTIVE &&
      dpSub.CHECKOUT_SESSION_ID === checkoutSessionId
    ) {
      logger.info("registerCheckoutSession: already active, skipping", { orgId, dpSubId });
      return {
        subId: dpSubId,
        billingCustomerId: dpSub.BILLING_CUSTOMER_ID,
        billingSubscriptionId: dpSub.BILLING_SUBSCRIPTION_ID,
        paymentStatus: PAYMENT_STATUS.ACTIVE,
        paymentProvider: PAYMENT_PROVIDER.STRIPE,
      };
    }

    logger.info("registerCheckoutSession: activating subscription", { orgId, dpSubId });
    const updatedCount = await adminDao.updateBillingFields(
      orgId,
      dpSubId,
      {
        BILLING_CUSTOMER_ID: billingCustomerId || dpSub.BILLING_CUSTOMER_ID,
        BILLING_SUBSCRIPTION_ID: billingSubscriptionId,
        PAYMENT_PROVIDER: PAYMENT_PROVIDER.STRIPE,
        PAYMENT_STATUS: PAYMENT_STATUS.ACTIVE,
        CHECKOUT_SESSION_ID: checkoutSessionId,
      },
      t,
    );
    if (updatedCount !== 1) {
      logger.warn(
        "updateBillingFields did not update exactly one row",
        { orgId, dpSubId, updatedCount },
      );
    }

    if (req) {
      const applicationID = session.metadata?.applicationID;
      const apiReferenceID = session.metadata?.apiReferenceID;
      const policyId = session.metadata?.policyId;
      const policyName = session.metadata?.policyName;

      if (applicationID && apiReferenceID) {
        try {
          const sharedMappings = await adminDao.getApplicationKeyMapping(orgId, applicationID, true);
          const nonSharedMappings = await adminDao.getApplicationKeyMapping(orgId, applicationID, false);

          // Only proceed if the app already has a CP app reference (i.e. keys have been generated)
          const existingMapping =
            sharedMappings.find((m) => m.dataValues.CP_APP_REF) ||
            nonSharedMappings.find((m) => m.dataValues.CP_APP_REF);

          if (existingMapping) {
            const cpAppRef = existingMapping.dataValues.CP_APP_REF;
            const isShared = sharedMappings.some((m) => m.dataValues.CP_APP_REF === cpAppRef);

            // Only create if this API doesn't already have a mapping
            const existingApiMapping = await adminDao.getApplicationAPIMapping(
              orgId, applicationID, apiReferenceID, cpAppRef, isShared
            );

            if (existingApiMapping.length === 0) {
              logger.info("registerCheckoutSession: syncing paid subscription to CP", {
                orgId, applicationID, apiReferenceID, cpAppRef,
              });

              const resolvedPolicyName = policyName ||
                (policyId
                  ? (await adminDao.getSubscriptionPolicyById(orgId, policyId, t))?.dataValues?.POLICY_NAME
                  : undefined);

              const requestBody = {
                apiId: apiReferenceID,
                applicationId: cpAppRef,
                throttlingPolicy: resolvedPolicyName,
                billingMetadata: {
                  billingCustomerId: billingCustomerId || dpSub.BILLING_CUSTOMER_ID,
                  billingSubscriptionId: billingSubscriptionId,
                },
              };

              let cpSubscribeResponse;
              try {
                cpSubscribeResponse = await invokeApiRequest(
                  req, "POST", `${controlPlaneUrl}/subscriptions`, {}, requestBody
                );
              } catch (cpErr) {
                if (cpErr.statusCode === 409) {
                  const existing = await invokeApiRequest(
                    req, "GET",
                    `${controlPlaneUrl}/subscriptions?apiId=${apiReferenceID}&applicationId=${cpAppRef}`,
                    {}
                  );
                  cpSubscribeResponse = existing?.list?.[0];
                } else {
                  throw cpErr;
                }
              }

              if (cpSubscribeResponse) {
                await adminDao.createApplicationKeyMapping({
                  orgID: orgId,
                  appID: applicationID,
                  cpAppRef: cpSubscribeResponse.applicationId,
                  apiRefID: cpSubscribeResponse.apiId,
                  subscriptionRefID: cpSubscribeResponse.subscriptionId,
                  sharedToken: isShared,
                  tokenType: isShared
                    ? constants.TOKEN_TYPES.OAUTH
                    : constants.TOKEN_TYPES.API_KEY,
                }, t);
              }
            }
          }
        } catch (cpSyncErr) {
          logger.warn("registerCheckoutSession: CP subscription sync failed (non-fatal)", {
            orgId, applicationID, apiReferenceID, err: cpSyncErr.message,
          });
        }
      }
    }

    return {
      subId: dpSubId,
      billingCustomerId,
      billingSubscriptionId,
      paymentStatus: PAYMENT_STATUS.ACTIVE,
      paymentProvider: PAYMENT_PROVIDER.STRIPE,
    };
  });
}

async function getUsage({ req, orgId, subId, from, to }) {
  logger.info("getUsage: initiated", { orgId, subId });
  const sub = await adminDao.getSubscriptionWithMeter(orgId, subId);
  if (!sub) throw new NotFoundError(`Subscription not found: subId=${subId}`);

  if (!sub.BILLING_CUSTOMER_ID || !sub.BILLING_SUBSCRIPTION_ID) {
    throw new ConflictError(
      "Billing is not registered for this subscription yet. Complete checkout first.",
    );
  }

  if (!sub.apimSubscriptionId) {
    throw new ConflictError(
      "No APIM subscription reference found. Ensure the subscription was properly synced with the control plane.",
    );
  }

  return moesifService.getUsageStats({
    req,
    subscriptionId: sub.apimSubscriptionId,
    from,
    to,
  });
}

/**
 * List invoices by customer
 */
async function listInvoices({ orgId, userId, period = "last3months" }) {
  logger.info("listInvoices: initiated", { orgId, period });
  const subs = await adminDao.listSubscriptionsByUser(orgId, userId);
  const any = subs.find(
    (s) =>
      s.BILLING_CUSTOMER_ID && s.PAYMENT_PROVIDER === PAYMENT_PROVIDER.STRIPE,
  );
  if (!any) return { invoices: [] };

  let created;
  const now = Math.floor(Date.now() / 1000);

  switch (period) {
    case "last3months":
      created = { gte: now - 90 * 24 * 60 * 60 };
      break;
    case "last6months":
      created = { gte: now - 180 * 24 * 60 * 60 };
      break;
    case "last12months":
      created = { gte: now - 365 * 24 * 60 * 60 };
      break;
    case "all":
      created = undefined;
      break;
    default:
      created = { gte: now - 90 * 24 * 60 * 60 };
  }

  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);
  const result = await stripeService.listInvoicesByCustomer(
    any.BILLING_CUSTOMER_ID,
    {
      limit: 100,
      created,
    },
    stripeSecretKey,
  );

  logger.info("listInvoices: completed", { orgId, period, invoiceCount: result.data?.length });

  return {
    invoices: (result.data || []).map((invoice) => ({
      id: invoice.id,
      number: invoice.number,
      created: invoice.created,
      amount: invoice.amount_due,
      currency: invoice.currency || "usd",
      status: invoice.status,
      hostedInvoiceUrl: invoice.hosted_invoice_url,
      invoicePdf: invoice.invoice_pdf,
    })),
  };
}

/**
 * Verify a subscription belongs to an application owned by userId.
 * Throws NotFoundError to avoid leaking subscription existence to other users.
 */
async function verifySubscriptionOwnership(orgId, subId, userId) {
  if (!userId) throw new ForbiddenError("User identity required for subscription ownership check");
  const sub = await adminDao.getSubscription(orgId, subId);
  if (!sub) throw new NotFoundError(`Subscription not found: subId=${subId}`);
  const app = await adminDao.getApplication(orgId, sub.APP_ID, userId);
  if (!app || app.CREATED_BY !== userId) {
    throw new NotFoundError(`Subscription not found: subId=${subId}`);
  }
}

async function listInvoicesBySubscription({ orgId, subId, userId }) {
  logger.info("listInvoicesBySubscription: initiated", { orgId, subId });
  await verifySubscriptionOwnership(orgId, subId, userId);
  const sub = await adminDao.getSubscription(orgId, subId);
  if (!sub?.BILLING_CUSTOMER_ID) return { data: [] };

  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);
  const invoices = await stripeService.listInvoicesByCustomer(
    sub.BILLING_CUSTOMER_ID,
    { limit: 50 },
    stripeSecretKey,
  );
  const filtered = (invoices.data || []).filter(
    (inv) => inv.subscription === sub.BILLING_SUBSCRIPTION_ID,
  );
  return { ...invoices, data: filtered };
}

async function getInvoice({ orgId, invoiceId, userId }) {
  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);
  const invoice = await stripeService.getInvoice(invoiceId, stripeSecretKey);

  if (userId && invoice?.customer) {
    const userSubs = await adminDao.listSubscriptionsByUser(orgId, userId);
    const customerIds = new Set(
      userSubs
        .filter((s) => s.BILLING_CUSTOMER_ID)
        .map((s) => s.BILLING_CUSTOMER_ID),
    );
    const invoiceCustomerId =
      typeof invoice.customer === "string"
        ? invoice.customer
        : invoice.customer?.id;
    if (!customerIds.has(invoiceCustomerId)) {
      throw new NotFoundError("Invoice not found");
    }
  }

  return invoice;
}

async function createBillingPortal({ orgId, subId, returnUrl, userId }) {
  logger.info("createBillingPortal: initiated", { orgId, subId });
  await verifySubscriptionOwnership(orgId, subId, userId);
  const sub = await adminDao.getSubscription(orgId, subId);

  if (!sub) {
    throw new NotFoundError(`Subscription not found: subId=${subId}`);
  }

  if (!sub.BILLING_CUSTOMER_ID) {
    logger.warn(
      "Subscription missing BILLING_CUSTOMER_ID",
      { orgId, subId },
    );
    throw new BadRequestError(
      "This subscription does not have billing information. Please ensure the subscription was created through Stripe checkout.",
    );
  }

  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);
  return stripeService.createCustomerPortalSession({
    customerId: sub.BILLING_CUSTOMER_ID,
    returnUrl,
    stripeSecretKey,
  });
}

async function createBillingPortalByOrg({ orgId, returnUrl, user }) {
  logger.info("createBillingPortalByOrg: initiated", { orgId });
  const subscriptions = await adminDao.listSubscriptionsByUser(orgId, user?.userId);
  const stripeSub = subscriptions.find(
    (s) =>
      s.PAYMENT_PROVIDER === PAYMENT_PROVIDER.STRIPE && s.BILLING_CUSTOMER_ID,
  );

  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);

  let customerId;
  if (stripeSub?.BILLING_CUSTOMER_ID) {
    customerId = stripeSub.BILLING_CUSTOMER_ID;
  } else {
    const email = user?.email;
    if (!email) {
      throw new BadRequestError(
        "Unable to open the payment portal: no billing account found and user email is unavailable.",
      );
    }
    const customer = await stripeService.findOrCreateCustomerByEmail(
      email,
      { orgId },
      stripeSecretKey,
    );
    customerId = customer.id;
  }

  try {
    return await stripeService.createCustomerPortalSession({
      customerId,
      returnUrl,
      stripeSecretKey,
    });
  } catch (stripeErr) {
    logger.error(
      "createBillingPortalByOrg: Stripe portal session creation failed",
      {
        orgId,
        stripeType: stripeErr.type,
        stripeCode: stripeErr.code,
      },
    );
    throw stripeErr;
  }
}

async function cancelPaidSubscription({ req, orgId, subId, user }) {
  logger.info("cancelPaidSubscription: initiated", { orgId, subId });
  await verifySubscriptionOwnership(orgId, subId, user?.sub);

  let billingSubscriptionId;
  let billingCustomerId;
  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);

  await sequelize.transaction(async (t) => {
    const sub = await adminDao.getSubscription(orgId, subId, t);
    if (!sub) throw new NotFoundError(`Subscription not found: subId=${subId}`);
    if (!sub.BILLING_SUBSCRIPTION_ID)
      throw new BadRequestError("No billing subscription id found to cancel.");

    billingSubscriptionId = sub.BILLING_SUBSCRIPTION_ID;
    billingCustomerId = sub.BILLING_CUSTOMER_ID;

    await adminDao.updateBillingFields(
      orgId,
      subId,
      {
        PAYMENT_STATUS: PAYMENT_STATUS.CANCELED,
      },
      t,
    );
  });

  logger.info("cancelPaidSubscription: canceling Stripe subscription", { orgId, subId, billingSubscriptionId });
  try {
    await stripeService.cancelSubscription(
      billingSubscriptionId,
      stripeSecretKey,
    );
  } catch (stripeErr) {
    if (
      stripeErr?.code === "subscription_already_canceled" ||
      (stripeErr?.statusCode === 400 &&
        String(stripeErr?.message).includes("already canceled"))
    ) {
      logger.warn(
        "Stripe subscription already canceled — continuing",
        { subId },
      );
    } else {
      throw stripeErr;
    }
  }

  setImmediate(() => {
    (async () => {
      try {
        if (req && billingCustomerId && billingSubscriptionId) {
          const url =
            `${controlPlaneUrl}/billing/moesif/subscription` +
            `?companyId=${encodeURIComponent(billingCustomerId)}` +
            `&subscriptionId=${encodeURIComponent(billingSubscriptionId)}`;
          await invokeApiRequest(req, "DELETE", url);
        }
      } catch (err) {
        logger.warn(
          "Moesif subscription cleanup failed (non-fatal)",
          { subId, err: err.message },
        );
      }
    })().catch(err => logger.warn("Moesif cleanup promise rejected", { subId, err: err.message }));
  });

  logger.info("cancelPaidSubscription: subscription canceled", { orgId, subId });
  return { subId, paymentStatus: PAYMENT_STATUS.CANCELED };
}

/**
 * Cancel a Stripe subscription directly by billing ID.
 * Used when the DP row has already been deleted and only the Stripe subscription remains.
 */
async function cancelStripeByBillingId(orgId, billingSubscriptionId) {
  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);
  try {
    await stripeService.cancelSubscription(
      billingSubscriptionId,
      stripeSecretKey,
    );
  } catch (err) {
    if (
      err?.code === "subscription_already_canceled" ||
      (err?.statusCode === 400 &&
        String(err?.message).includes("already canceled"))
    ) {
      logger.warn(
        "Stripe subscription already canceled",
        { billingSubscriptionId },
      );
    } else {
      throw err;
    }
  }
}

/**
 * Stripe webhook handler delegate.
 * - Update PAYMENT_STATUS to PAST_DUE/CANCELED/ACTIVE based on invoice/payment events.
 */
async function handleStripeWebhook(req, orgId) {
  if (!orgId) {
    throw new Error("Missing orgId in webhook URL");
  }
  logger.info("handleStripeWebhook: verifying event", { orgId });
  const { secretKey, webhookSecret } =
    await getDecryptedStripeKeysForOrg(orgId);
  const event = await stripeService.verifyAndConstructWebhookEvent(
    req,
    secretKey,
    webhookSecret,
  );
  logger.info("handleStripeWebhook: event verified", { orgId, eventType: event?.type });
  await stripeService.applyWebhookToLocalState(event, { adminDao });
  logger.info("handleStripeWebhook: event processed", { orgId, eventType: event?.type });
  return true;
}

async function getSubscriptionBillingStatus({ orgId, subId, userId }) {
  logger.info("getSubscriptionBillingStatus: initiated", { orgId, subId });
  await verifySubscriptionOwnership(orgId, subId, userId);
  const sub = await adminDao.getSubscription(orgId, subId);
  if (!sub) throw new NotFoundError(`Subscription not found: subId=${subId}`);
  return {
    subId,
    paymentProvider: sub.PAYMENT_PROVIDER,
    paymentStatus: sub.PAYMENT_STATUS,
    billingCustomerId: sub.BILLING_CUSTOMER_ID,
    billingSubscriptionId: sub.BILLING_SUBSCRIPTION_ID,
  };
}

/**
 * Get user usage data for billing page
 */
async function getUserUsageData(
  req,
  userContext,
  orgId,
  period = "current",
  t,
  customFrom,
  customTo,
) {
  const empty = {
    totalRequests: 0,
    activeSubscriptions: 0,
    estimatedCost: 0,
    avgResponseTime: 0,
    subscriptions: [],
  };

  try {
    logger.info("getUserUsageData called", { orgId, period });
    const userId = userContext?.userId || userContext?.sub;
    if (!userId) {
      logger.warn("No userId found in userContext", { orgId });
      return empty;
    }

    const subscriptions = await adminDao.getUserSubscriptionsWithDetails(
      orgId,
      userId,
    );

    const ACTIVE_STATUSES = new Set(["ACTIVE", "CANCEL_SCHEDULED"]);

    const activeSubs = (subscriptions || []).filter((sub) => {
      const paymentStatus = sub.PAYMENT_STATUS;
      return (
        ACTIVE_STATUSES.has(paymentStatus) ||
        paymentStatus === null ||
        paymentStatus === undefined ||
        paymentStatus === ""
      );
    });

    if (activeSubs.length === 0) return empty;

    logger.info("Active subscriptions", { activeSubs: activeSubs.length });

    const rows = await Promise.all(
      activeSubs.map(async (sub) => {
        const subId = sub?.ID || sub?.SUB_ID || sub?.id;

        try {
          const pricingModel = sub?.pricingModel || "FLAT";
          const unitAmount = Number(sub?.unitAmount || 0);
          const flatAmount = Number(sub?.flatAmount || 0);

          let requests = 0;
          let avgRt = 0;
          let cost = 0;
          let currency = "USD";

          if (pricingModel === "FLAT") {
            // Flat plan: no Moesif usage needed
            cost = flatAmount;
          } else {
            const apimSubId = sub?.apimSubscriptionId || "";
            if (!apimSubId) {
              logger.warn(
                "Metered subscription missing APIM subscription reference",
                { orgId, userId, subId, policyId: sub.POLICY_ID },
              );
              cost = 0;
              requests = 0;
            } else {
              let from, to;
              if (customFrom || customTo) {
                from = customFrom ? `${customFrom}T00:00:00.000Z` : moesifService.getPeriodRange(period).from;
                to = customTo ? `${customTo}T23:59:59.999Z` : new Date().toISOString();
              } else {
                ({ from, to } = moesifService.getPeriodRange(period));
              }
              const usage = await moesifService.getUsageStats({
                req,
                subscriptionId: apimSubId,
                from,
                to,
              });
              requests =
                Number(usage?.total_requests ?? usage?.usage ?? 0) || 0;
              avgRt = Number(usage?.avg_response_time ?? 0) || 0;
              currency = usage?.currency || "USD";

              if (sub.BILLING_SUBSCRIPTION_ID && sub.PAYMENT_PROVIDER === PAYMENT_PROVIDER.STRIPE) {
                try {
                  const { secretKey: stripeSecretKey } = await getDecryptedStripeKeysForOrg(orgId);
                  const upcomingInvoice = await stripeService.getUpcomingInvoice(
                    sub.BILLING_SUBSCRIPTION_ID,
                    stripeSecretKey,
                    sub.BILLING_CUSTOMER_ID,
                  );
                  if (upcomingInvoice) {
                    const amountCents = upcomingInvoice.amount_due ?? upcomingInvoice.total ?? 0;
                    cost = amountCents / 100; // Stripe amounts are in cents
                    currency = (upcomingInvoice.currency || currency).toUpperCase();
                  }
                } catch (stripeErr) {
                  logger.warn("Failed to get Stripe upcoming invoice", { subId, err: stripeErr.message });
                }
              }

            }
          }

          const usageRow = {
            apiName: sub.apiName || "Unknown API",
            applicationName: sub.applicationName || "Unknown App",
            planName: sub.planName || "Unknown Plan",
            requests,
            pricingModel,
            cost,
            currency,
            avgResponseTime: avgRt,
          };
          return usageRow;
        } catch (err) {
          logger.error(
            "Failed to build usage row",
            { subId: sub?.ID, policyId: sub?.POLICY_ID, apiId: sub?.API_ID, err },
          );
          return null;
        }
      }),
    );

    const subscriptionData = rows.filter(Boolean);
    const totalRequests = subscriptionData.reduce(
      (sum, r) => sum + (r.requests || 0),
      0,
    );
    const estimatedCost = subscriptionData.reduce(
      (sum, r) => sum + (r.cost || 0),
      0,
    );

    const rtRows = subscriptionData.filter(
      (r) => r.avgResponseTime && r.avgResponseTime > 0,
    );
    const avgResponseTime =
      rtRows.length > 0
        ? Math.round(
            rtRows.reduce((sum, r) => sum + r.avgResponseTime, 0) /
              rtRows.length,
          )
        : 0;

    const currency =
      subscriptionData.find((r) => r.currency)?.currency || "USD";

    return {
      totalRequests,
      activeSubscriptions: subscriptionData.length,
      estimatedCost,
      currency,
      avgResponseTime,
      subscriptions: subscriptionData,
    };
  } catch (err) {
    logger.error("getUserUsageData failed", { err, orgId });
    return empty;
  }
}

/**
 * Get payment methods for the user
 */
async function getPaymentMethods(userContext, orgId) {
  try {
    logger.info("getPaymentMethods called", { orgId });
    // Get all subscriptions for the organization to find the Stripe customer ID
    const subscriptions = await adminDao.listSubscriptionsByUser(orgId, userContext?.userId);
    logger.info(
      "Listing subscriptions for payment methods",
      { orgId, subscriptionsCount: subscriptions?.length },
    );

    const { secretKey: stripeSecretKey } =
      await getDecryptedStripeKeysForOrg(orgId);

    // Find a subscription with a Stripe customer ID
    const stripeSubscription = (subscriptions || []).find(
      (s) =>
        s.BILLING_CUSTOMER_ID && s.PAYMENT_PROVIDER === PAYMENT_PROVIDER.STRIPE,
    );

    let customerId;
    if (stripeSubscription?.BILLING_CUSTOMER_ID) {
      customerId = stripeSubscription.BILLING_CUSTOMER_ID;
    } else {
      const email = userContext?.email;
      if (!email) {
        logger.warn(
          "No Stripe customer ID and no user email available",
          { orgId },
        );
        return [];
      }
      const customer = await stripeService.findOrCreateCustomerByEmail(
        email,
        { orgId },
        stripeSecretKey,
      );
      customerId = customer.id;
    }
    logger.info(
      "Using Stripe customer for payment methods",
      { orgId },
    );
    const paymentMethods = await stripeService.listPaymentMethods(
      customerId,
      stripeSecretKey,
    );
    logger.info(
      "Payment methods fetched",
      { orgId, paymentMethodsCount: paymentMethods?.length },
    );
    const stripeCustomer = await stripeService.getCustomer(
      customerId,
      stripeSecretKey,
    );
    const defaultPaymentMethodId =
      stripeCustomer.invoice_settings?.default_payment_method;
    return paymentMethods.map((pm) => ({
      id: pm.id,
      brand: pm.card?.brand || "card",
      last4: pm.card?.last4,
      expMonth: pm.card?.exp_month,
      expYear: pm.card?.exp_year,
      isDefault: pm.id === defaultPaymentMethodId,
    }));
  } catch (err) {
    logger.error("getPaymentMethods failed", { err, orgId });
    throw err;
  }
}

/**
 * Get billing information
 */
async function getBillingInfo(userContext, orgId) {
  try {
    const org = await adminDao.getOrganization(orgId);

    const subscriptions = await adminDao.listSubscriptionsByUser(orgId, userContext?.userId);

    if (!subscriptions || subscriptions.length === 0) {
      return {
        organizationName: org?.NAME,
        email: userContext.email,
      };
    }

    const stripeSubscription = subscriptions.find(
      (s) =>
        s.BILLING_CUSTOMER_ID && s.PAYMENT_PROVIDER === PAYMENT_PROVIDER.STRIPE,
    );

    if (!stripeSubscription || !stripeSubscription.BILLING_CUSTOMER_ID) {
      return {
        organizationName: org?.NAME,
        email: userContext.email,
      };
    }

    const customerId = stripeSubscription.BILLING_CUSTOMER_ID;

    const { secretKey: stripeSecretKey } =
      await getDecryptedStripeKeysForOrg(orgId);
    const customer = await stripeService.getCustomer(
      customerId,
      stripeSecretKey,
    );

    return {
      organizationName: customer.name || org?.NAME,
      email: customer.email || userContext.email,
      address: customer.address,
      taxId: customer.tax_ids?.data?.[0]?.value,
    };
  } catch (err) {
    logger.error("getBillingInfo failed", { err });
    throw err;
  }
}

/**
 * Get active subscriptions formatted for billing page display
 */
async function getSubscriptionsForBilling(orgId, userId) {
  try {
    // Get subscriptions with all details using JOIN query
    const subscriptions = await adminDao.getUserSubscriptionsWithDetails(
      orgId,
      userId,
    );

    if (!subscriptions || subscriptions.length === 0) {
      return [];
    }

    const formattedSubs = await Promise.all(
      subscriptions.map(async (sub) => {
        try {
          let amount = 0;
          let currency = "usd";
          let nextBillingDate = null;
          let billingCycle = sub.billingPlan || sub.billingCycle || "N/A";
          let status = sub.PAYMENT_STATUS || "";

          if (
            sub.BILLING_SUBSCRIPTION_ID &&
            sub.PAYMENT_PROVIDER === PAYMENT_PROVIDER.STRIPE
          ) {
            try {
              const { secretKey: stripeSecretKey } =
                await getDecryptedStripeKeysForOrg(orgId);
              const stripeSub = await stripeService.getSubscription(
                sub.BILLING_SUBSCRIPTION_ID,
                stripeSecretKey,
              );
              if (stripeSub) {
                nextBillingDate = stripeSub.current_period_end
                  ? stripeSub.current_period_end * 1000
                  : null;
                const interval =
                  stripeSub.items.data[0]?.price?.recurring?.interval;
                if (interval) {
                  billingCycle =
                    interval.charAt(0).toUpperCase() + interval.slice(1);
                }
                // Use upcoming invoice for actual billed amount (includes metered usage)
                const upcomingInvoice = await stripeService.getUpcomingInvoice(
                  sub.BILLING_SUBSCRIPTION_ID,
                  stripeSecretKey,
                  sub.BILLING_CUSTOMER_ID,
                );
                if (upcomingInvoice) {
                  amount = upcomingInvoice.amount_due ?? upcomingInvoice.total ?? 0;
                  currency = upcomingInvoice.currency || stripeSub.currency || "usd";
                } else {
                  // Fallback to unit_amount if no upcoming invoice
                  amount = stripeSub.items.data[0]?.price?.unit_amount || 0;
                  currency = stripeSub.currency || stripeSub.items.data[0]?.price?.currency || "usd";
                }
              }
            } catch (err) {
              logger.warn(
                "Failed to get Stripe subscription details",
                { err, subId: sub.id },
              );
            }
          }

          return {
            id: sub.id,
            apiName: sub.apiName || "Unknown API",
            applicationName: sub.applicationName || "Unknown App",
            planName: sub.planName || "Unknown Plan",
            billingCycle,
            amount,
            currency,
            nextBillingDate,
            status:
              status === "ACTIVE"
                ? "active"
                : status?.toLowerCase() || "unknown",
          };
        } catch (err) {
          logger.warn(
            "Failed to enhance subscription with Stripe data",
            { err, subId: sub.id },
          );
          return {
            id: sub.id,
            apiName: sub.apiName || "Unknown API",
            applicationName: sub.applicationName || "Unknown App",
            planName: sub.planName || "Unknown Plan",
            billingCycle: sub.billingPlan || sub.billingCycle || "N/A",
            amount: 0,
            nextBillingDate: null,
            status: sub.status?.toLowerCase() || "unknown",
          };
        }
      }),
    );

    return formattedSubs;
  } catch (err) {
    logger.error("getSubscriptionsForBilling failed", { err, orgId });
    throw err;
  }
}

/**
 * Handles Stripe return and ensures paid subscription is activated.
 * Call this from your controller when handling Stripe return URL.
 */
async function handleStripeReturnAndActivate({ orgId, sessionId, user, req = null }) {
  logger.info("handleStripeReturnAndActivate: initiated", { orgId, sessionId });
  const { secretKey: stripeSecretKey } =
    await getDecryptedStripeKeysForOrg(orgId);
  const session = await stripeService.retrieveCheckoutSession(
    sessionId,
    stripeSecretKey,
  );
  const dpSubId = session?.metadata?.dpSubId;
  if (!dpSubId) {
    throw new BadRequestError("Stripe session missing dpSubId metadata.");
  }
  const dpSub = await adminDao.getSubscription(orgId, dpSubId);
  if (!dpSub) {
    throw new NotFoundError(`DP subscription not found: subId=${dpSubId}`);
  }
  const policyRow = await adminDao.getSubscriptionPolicyById(
    orgId,
    dpSub.POLICY_ID,
  );
  if (
    isPaidPolicy(policyRow) &&
    dpSub.PAYMENT_STATUS === PAYMENT_STATUS.PENDING
  ) {
    logger.info("handleStripeReturnAndActivate: activating pending subscription", { orgId, dpSubId, sessionId });
    return module.exports.registerCheckoutSession({
      orgId,
      checkoutSessionId: sessionId,
      user,
      req,
    });
  }
  return {
    subId: dpSubId,
    paymentStatus: dpSub.PAYMENT_STATUS,
    paymentProvider: dpSub.PAYMENT_PROVIDER,
    billingCustomerId: dpSub.BILLING_CUSTOMER_ID,
    billingSubscriptionId: dpSub.BILLING_SUBSCRIPTION_ID,
  };
}

module.exports = {
  createCheckoutSession,
  registerCheckoutSession,
  getUsage,
  listInvoices,
  listInvoicesBySubscription,
  getInvoice,
  createBillingPortal,
  createBillingPortalByOrg,
  cancelPaidSubscription,
  cancelStripeByBillingId,
  handleStripeWebhook,
  getSubscriptionBillingStatus,
  getUserUsageData,
  getPaymentMethods,
  getBillingInfo,
  getSubscriptionsForBilling,
  handleStripeReturnAndActivate,
};
