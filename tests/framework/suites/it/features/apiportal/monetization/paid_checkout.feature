# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@APIPORTAL @api-portal-paid-checkout
Feature: Buying a flat rate plan in the API Portal
  As a developer
  I want to pay for a plan through checkout and stop paying when I unsubscribe
  So that I get the plan I paid for, and the portal follows what Stripe says about my payments

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-BILLING-007..012 and 016..019, at the
  # API level. The paid plan is published from the control plane with billing set up in this
  # runner's own portal; Stripe and Moesif are the testbench services, reset before every scenario.
  # The api-portal-billing block has no gateway: the gateway's handling of a paid subscription is
  # covered by management-portal/monetization/paid_subscription_access.feature.

  Background:
    Given I send a "POST" request to the "stripe" service at "/test/reset"
    And I send a "POST" request to the "moesif" service at "/test/reset"
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "paid-checkout-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "paid-checkout-plan" and store it as "planHandle"
    And I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planHandle}","displayName":"Paid ${CTX:planHandle}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"HOUR","limitCount":10000}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"29"}}
      """
    And the response status code should be 201
    And I generate a unique resource name from "paid-checkout-api" and store it as "apiHandle"
    And I generate a unique API context from "/paid-checkout" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I generate a unique resource name from "paid-checkout-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    And I set up billing in the API Portal
    And the response status code should be 201
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    And I register API Portal resource "${CTX:planHandle}" at "/subscription-plans" for cleanup

  @APIP-BILLING-007
  Scenario: A developer pays for a flat rate plan and is subscribed once Stripe confirms the payment
    # Publishing the paid plan created it in Moesif, which created its price in Stripe.
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the JSON response array field "plans" should have 1 item
    And the JSON response array field "prices" should have 1 item
    And the JSON response field "prices[0].pricing_model" should be "flat"
    And the JSON response field "prices[0].price_in_decimal" should be "29"
    And the JSON response field "prices[0].currency" should be "usd"
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response field "prices[0].unit_amount" should be 2900

    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "OPEN"
    And the JSON response field "publishableKey" should be "pk_test_apiportalbilling"
    And the JSON response should have field "clientSecret"
    And I store the JSON response field "id" as "checkoutId"
    # Starting it again shows the same open checkout.
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    Then the response status code should be 201
    And the JSON response field "id" should be "${CTX:checkoutId}"
    # Nothing is subscribed before the payment.
    When I send an authenticated API Portal "GET" request to "/checkout-sessions/${CTX:checkoutId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "status" should be "OPEN"
    And the JSON response field "subscriptionId" should be "null"

    When the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"
    Then the JSON response field "status" should be "COMPLETED"
    And the JSON response field "artifactId" should be "${CTX:apiHandle}"
    And the JSON response field "subscriptionPlanId" should be "${CTX:planHandle}"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the response status code should be 200
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response field "billingStatus" should be "ACTIVE"
    And the JSON response field "byUsage" should be false
    And the JSON response field "endsAt" should be "null"
    And the JSON response field "currentPeriod.end" should not equal "null"

    # One subscription per API: a second checkout is refused.
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    Then the response status code should be 409
    # The first invoice is the developer's.
    When I send an authenticated API Portal "GET" request to "/billing/invoices" as "developer"
    Then the response status code should be 200
    And the JSON response array field "invoices" should have 1 item
    And the JSON response field "invoices[0].status" should be "paid"
    And the JSON response field "invoices[0].amount" should be "29.00"
    # The revenue report counts the paid subscription.
    When I send an authenticated API Portal "GET" request to "/billing-reports/revenue" as "admin"
    Then the response status code should be 200
    And the JSON response field "configured" should be true
    And the JSON response field "paidSubscriptions" should be 1
    # Billing is not removed while a paid subscription is active.
    When I send an authenticated API Portal "DELETE" request to "/billing-configuration" as "admin"
    Then the response status code should be 409
    And the JSON response field "code" should be "BILLING_SUBSCRIPTIONS_ACTIVE"

  @APIP-BILLING-008
  Scenario: The subscription's billing status follows Stripe as payments fail and are paid
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    And the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"

    # A renewal fails: Stripe retries it, and the subscription is past due.
    When I send a "POST" request to the "stripe" service at "/test/subscriptions/${CTX:checkoutId}" with body:
      """
      {"status":"past_due"}
      """
    Then the response status should be 200
    And the JSON response field "deliveries[0].status" should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the JSON response field "billingStatus" should be "PAST_DUE"
    And the JSON response field "status" should be "PAST_DUE"

    # Stripe stops retrying while the subscription is still past due: payment is pending.
    When I send a "POST" request to the "stripe" service at "/test/subscriptions/${CTX:checkoutId}" with body:
      """
      {"status":"past_due","retriesEnded":true}
      """
    Then the response status should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the JSON response field "billingStatus" should be "UNPAID"
    And the JSON response field "status" should be "PAYMENT_PENDING"
    When I send an authenticated API Portal "GET" request to "/billing-reports/revenue" as "admin"
    Then the JSON response field "paymentIssues.paymentPending" should be 1

    # The developer pays: active again.
    When I send a "POST" request to the "stripe" service at "/test/subscriptions/${CTX:checkoutId}" with body:
      """
      {"status":"active"}
      """
    Then the response status should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the JSON response field "billingStatus" should be "ACTIVE"

    # Stripe ends the subscription after failed payments: it is kept, with payment pending.
    When I send a "POST" request to the "stripe" service at "/test/subscriptions/${CTX:checkoutId}" with body:
      """
      {"status":"canceled"}
      """
    Then the response status should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the response status code should be 200
    And the JSON response field "billingStatus" should be "CANCELLED"
    And the JSON response field "status" should be "PAYMENT_PENDING"

  @APIP-BILLING-009
  Scenario: Unsubscribing from a flat rate plan ends it at the end of the period already paid for
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    And the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response should have field "endsAt"
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array "subscriptions" item with "metadata.wso2_checkout" equal to "${CTX:checkoutId}" should have "cancel_at_period_end" equal to "true"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the JSON response should have field "endsAt"
    And the JSON response field "endsAt" should not equal "null"
    # The plan runs until then, so the API can't be bought again yet.
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    Then the response status code should be 409
    And the JSON response field "message" should contain "You can subscribe again after that."

    # The period ends: Stripe ends the subscription and the portal removes it.
    When I send a "POST" request to the "stripe" service at "/test/subscriptions/${CTX:checkoutId}/period-end"
    Then the response status should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 404

  @APIP-BILLING-010
  Scenario: Ending a flat rate subscription now cancels it in Stripe at once, with no refund
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    And the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}?immediate=true" as "developer"
    Then the response status code should be 200
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array "subscriptions" item with "metadata.wso2_checkout" equal to "${CTX:checkoutId}" should have "status" equal to "canceled"
    And the JSON response array field "refunds" should have 0 items
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 404
    # Nothing paid is left, so billing can be removed.
    When I send an authenticated API Portal "DELETE" request to "/billing-configuration" as "admin"
    Then the response status code should be 204

  @APIP-BILLING-011
  Scenario: A checkout the developer leaves unpaid ends without a subscription
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    When I send a "POST" request to the "stripe" service at "/test/checkout-sessions/${CTX:checkoutId}/expire"
    Then the response status should be 200
    And the JSON response field "deliveries[0].status" should be 200
    When I send an authenticated API Portal "GET" request to "/checkout-sessions/${CTX:checkoutId}" as "developer"
    Then the response status code should be 404
    # A new checkout can start.
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    Then the response status code should be 201
    And the JSON response field "id" should not equal "${CTX:checkoutId}"

  @APIP-BILLING-012 @type:negative
  Scenario: Stripe's events are accepted only when they are signed and recent, and repeating one changes nothing
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    And the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"
    When I send a "POST" request to the "stripe" service at "/test/events/resend" with body:
      """
      {"type":"checkout.session.completed","signature":"invalid"}
      """
    Then the JSON response field "deliveries[0].status" should be 400
    When I send a "POST" request to the "stripe" service at "/test/events/resend" with body:
      """
      {"type":"checkout.session.completed","signature":"stale"}
      """
    Then the JSON response field "deliveries[0].status" should be 400
    When I send a "POST" request to the "stripe" service at "/test/events/resend" with body:
      """
      {"type":"checkout.session.completed"}
      """
    Then the JSON response field "deliveries[0].status" should be 200
    # The repeated event did not subscribe the developer twice.
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:apiHandle}" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    # An event for an organization without billing is refused.
    When I send an unauthenticated API Portal "POST" request to "/api-portal/billing/stripe/webhooks/00000000-0000-4000-8000-000000000000"
    Then the response status code should be 404

  @APIP-BILLING-016 @type:negative
  Scenario Outline: A checkout <problem> is refused, and nothing is started in Stripe
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON values:
      | artifactId         | <api>  |
      | subscriptionPlanId | <plan> |
    Then the response status code should be <status>
    And the response body should contain "<message>"
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array field "checkoutSessions" should have 0 items
    # The same API and plan, named as they are, start a checkout.
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON values:
      | artifactId         | ${CTX:apiHandle}  |
      | subscriptionPlanId | ${CTX:planHandle} |
    Then the response status code should be 201
    And the JSON response field "status" should be "OPEN"

    Examples:
      | problem                         | api                             | plan                            | status | message                                   |
      | that names no plan              | ${CTX:apiHandle}                |                                 | 400    | artifactId and subscriptionPlanId are required |
      | for a plan the API doesn't offer | ${CTX:apiHandle}                | ${UNIQUE:unoffered-plan}        | 400    | Subscription plan not found for this API |
      | for an API that doesn't exist   | ${UNIQUE:missing-api}           | ${CTX:planHandle}               | 404    | API not found                             |

  @APIP-BILLING-017 @type:negative
  Scenario: A free plan is subscribed to directly, not bought through checkout
    # This block's portal does not create its default plans, and its role mapping is the
    # product's, so the administrator creates the free plan and the API that offers it.
    Given I generate a unique resource name from "free-plan" and store it as "freePlanId"
    And I generate a unique resource name from "free-api" and store it as "freeApiId"
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "freePlanId":
      """
      {"id":"${CTX:freePlanId}","displayName":"${CTX:freePlanId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"MINUTE"}]}
      """
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "admin" with values:
      | id                | ${CTX:freeApiId}               |
      | name              | Free REST API                  |
      | subscriptionPlans | [{"id":"${CTX:freePlanId}"}]   |
    And the response status code should be 201
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON values:
      | artifactId         | ${CTX:freeApiId}  |
      | subscriptionPlanId | ${CTX:freePlanId} |
    Then the response status code should be 400
    And the JSON response field "code" should be "FREE_PLAN"
    And the JSON response field "message" should be "This plan is free: subscribe to it directly."
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array field "checkoutSessions" should have 0 items
    # Subscribing to it directly works.
    When a developer subscription for API "${CTX:freeApiId}" using plan "${CTX:freePlanId}" is created and stored as "freeSubscriptionId"
    Then the response status code should be 201
    And the JSON response field "subscriptionPlanName" should be "${CTX:freePlanId}"

  @APIP-BILLING-018 @type:negative
  Scenario: A paid subscription's plan can't be changed or suspended here, and its usage is read only for known periods
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON values:
      | artifactId         | ${CTX:apiHandle}  |
      | subscriptionPlanId | ${CTX:planHandle} |
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    And the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"
    # Moving off a paid plan is done by unsubscribing and buying the new plan.
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | artifactId | ${CTX:apiHandle}  |
      | planId     | ${CTX:planHandle} |
    Then the response status code should be 409
    And the JSON response field "code" should be "PAID_PLAN_REQUIRES_CHECKOUT"
    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | INACTIVE |
    Then the response status code should be 409
    And the JSON response field "code" should be "PAID_PLAN_REQUIRES_CHECKOUT"
    And the JSON response field "message" should be "A paid subscription cannot be suspended. Unsubscribe to stop it."
    # The subscription is still active on its plan, and Stripe still bills it.
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the response status code should be 200
    And the JSON response field "status" should be "ACTIVE"
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array "subscriptions" item with "metadata.wso2_checkout" equal to "${CTX:checkoutId}" should have "status" equal to "active"
    # Usage is read for the known periods and intervals only.
    When I send an authenticated API Portal "GET" request to "/usage?period=NEXT_MONTH" as "developer"
    Then the response status code should be 400
    And the JSON response field "message" should contain "period must be one of"
    When I send an authenticated API Portal "GET" request to "/usage?interval=MINUTE" as "developer"
    Then the response status code should be 400
    And the JSON response field "message" should contain "interval must be one of"
    When I send an authenticated API Portal "GET" request to "/usage?period=LAST_7_DAYS&interval=DAY" as "developer"
    Then the response status code should be 200
    And the JSON response field "usageAvailable" should be true

  @APIP-BILLING-019
  Scenario: A subscriber pages through their own invoices and opens their billing page, and only an administrator reads the revenue reports
    # Nobody has bought a plan yet: there is no billing page to open.
    When I send an authenticated API Portal "POST" request to "/billing/billing-portal-sessions" as "developer" with JSON body:
      """
      {}
      """
    Then the response status code should be 409
    And the JSON response field "message" should be "Billing details are available once you subscribe to a paid plan."
    Given I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON values:
      | artifactId         | ${CTX:apiHandle}  |
      | subscriptionPlanId | ${CTX:planHandle} |
    And the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    And the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"

    # The subscriber's invoices, a page at a time.
    When I send an authenticated API Portal "GET" request to "/billing/invoices?period=LAST_3_MONTHS&limit=1" as "developer"
    Then the response status code should be 200
    And the JSON response field "period" should be "LAST_3_MONTHS"
    And the JSON response field "canManageBilling" should be true
    And the JSON response array field "invoices" should have 1 item
    And the JSON response field "invoices[0].amount" should be "29.00"
    And the JSON response field "invoices[0].subscription" should not equal "null"
    When I send an authenticated API Portal "GET" request to "/billing/invoices?period=LAST_WEEK" as "developer"
    Then the response status code should be 400
    And the JSON response field "message" should contain "period must be one of"
    When I send an authenticated API Portal "GET" request to "/billing/invoices?limit=0" as "developer"
    Then the response status code should be 400
    And the JSON response field "message" should contain "limit must be at least 1"
    When I send an authenticated API Portal "GET" request to "/billing/invoices?limit=1.5" as "developer"
    Then the response status code should be 400
    And the JSON response field "message" should contain "limit"
    When I send an authenticated API Portal "GET" request to "/billing/invoices?after=not-a-cursor" as "developer"
    Then the response status code should be 400
    And the JSON response field "message" should be "after is not in the expected format"

    # The subscriber's billing page at Stripe.
    When I send an authenticated API Portal "POST" request to "/billing/billing-portal-sessions" as "developer" with JSON body:
      """
      {}
      """
    Then the response status code should be 201
    And the JSON response field "url" should contain "https://billing.stripe.test/"

    # The administrator's revenue reports.
    When I send an authenticated API Portal "GET" request to "/billing-reports/revenue" as "admin"
    Then the response status code should be 200
    And the JSON response field "configured" should be true
    And the JSON response field "paidSubscriptions" should be 1
    And the JSON response field "paidApis" should be 1
    When I send an authenticated API Portal "GET" request to "/billing-reports/invoices" as "admin"
    Then the response status code should be 200
    And the JSON response array "invoices" should contain an item with "total" equal to "29.00"
    And the JSON response array "invoices" should contain an item with "subscriptionId" equal to "${CTX:subscriptionId}"
    When I send an authenticated API Portal "GET" request to "/billing-reports/invoices?from=2026-02-01&to=2026-01-01" as "admin"
    Then the response status code should be 400
    And the JSON response field "message" should be "from must be before to"
    When I send an authenticated API Portal "GET" request to "/billing-reports/invoices?from=not-a-date" as "admin"
    Then the response status code should be 400
    And the JSON response field "message" should contain "from and to must be dates"
    # A subscriber can't read them.
    When I send an authenticated API Portal "GET" request to "/billing-reports/revenue" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/billing-reports/invoices" as "developer"
    Then the response status code should be 403
