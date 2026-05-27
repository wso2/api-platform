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

const { CustomError } = require("../utils/errors/customErrors");
const logger = require("../config/logger");
const { invokeApiRequest } = require("../utils/util");
const { config } = require('../config/configLoader');

const controlPlaneUrl = config.controlPlane?.url;

class BadRequestError extends CustomError {
  constructor(message, details) {
    super(400, message, details);
  }
}

function getPeriodRange(period = "current") {
  const to = new Date();
  let from;

  if (period === "lastMonth") {
    const fom = new Date(Date.UTC(to.getUTCFullYear(), to.getUTCMonth() - 1, 1));
    const eom = new Date(Date.UTC(to.getUTCFullYear(), to.getUTCMonth(), 0, 23, 59, 59, 999));
    return { from: fom.toISOString(), to: eom.toISOString() };
  } else if (period === "last3months" || period === "last90") {
    from = new Date(to.getTime() - 90 * 24 * 60 * 60 * 1000);
  } else if (period === "last30") {
    from = new Date(to.getTime() - 30 * 24 * 60 * 60 * 1000);
  } else {
    from = new Date(Date.UTC(to.getUTCFullYear(), to.getUTCMonth(), 1));
  }

  return { from: from.toISOString(), to: to.toISOString() };
}

/**
 * Fetch usage and billing stats from APIM's Moesif usage endpoint.
 * APIM resolves the billing meter ID and Moesif subscription from the APIM subscription UUID,
 * then calls the Ballerina Moesif microservice which calls Moesif's /billing/reports.
 *
 * @param {object} params
 * @param {object} params.req             - Express request object (for auth token)
 * @param {string} params.subscriptionId  - APIM subscription UUID
 * @param {string} [params.from]          - ISO-8601 start date
 * @param {string} [params.to]            - ISO-8601 end date
 * @param {string} [params.period]        - Period shortcut (current|last30|last90), used if from/to not provided
 */
async function getUsageStats({ req, subscriptionId, from, to, period = "current" }) {
  if (!req) throw new BadRequestError("Request object is required for APIM usage call");
  if (!subscriptionId) throw new BadRequestError("subscriptionId is required");
  if (!controlPlaneUrl) throw new BadRequestError("controlPlane.url is not configured");

  // If from/to not provided, compute from period
  if (!from || !to) {
    const range = getPeriodRange(period);
    from = from || range.from;
    to = to || range.to;
  }

  const url = new URL(`${controlPlaneUrl}/billing/moesif/usage`);
  url.searchParams.set("subscriptionId", subscriptionId);
  url.searchParams.set("from", from);
  url.searchParams.set("to", to);

  logger.info("[moesifService] Calling APIM usage endpoint", { subscriptionId, from, to });

  const raw = await invokeApiRequest(req, "GET", url.toString());

  const response = typeof raw === "string" ? JSON.parse(raw) : raw;

  let totalUsage, estimatedCost, currency;

  if (Array.isArray(response)) {
    totalUsage = response.reduce((sum, r) => sum + (Number(r?.report_total_usage ?? r?.meter_usage ?? r?.totalUsage ?? 0) || 0), 0);
    estimatedCost = response.reduce((sum, r) => sum + (Number(r?.price_in_decimal ?? r?.estimatedCost ?? 0) || 0), 0);
    currency = response[0]?.currency || "USD";
  } else {
    totalUsage = Number(response?.totalUsage ?? response?.total_usage ?? response?.meter_usage ?? 0) || 0;
    estimatedCost = Number(response?.estimatedCost ?? response?.estimated_cost ?? response?.price_in_decimal ?? 0) || 0;
    currency = response?.currency || "USD";
  }

  logger.info("[moesifService] APIM usage response", { subscriptionId, totalUsage });

  return {
    total_requests: totalUsage,
    usage: totalUsage,
    estimated_cost: estimatedCost,
    currency: currency,
    billing_type: "METERED",
    avg_response_time: 0,
  };
}

module.exports = {
  getUsageStats,
  getPeriodRange,
};
