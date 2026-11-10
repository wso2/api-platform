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

@publication-monetization @APIPORTAL
Feature: Publishing priced plans to an API Portal
  As an API publisher who prices plans in the control plane
  I want publishing an API to bring its priced plans and their meters to the API Portal
  So that the portal sells exactly the prices the control plane defined, and never a different one

  @APIP-048
  Scenario: Priced plans, meters and new limits reach the API Portal on publish, and portal-made ones reach the control plane
    # Publishing an API with a usage-priced plan brings the meter and the priced plan to the API Portal
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "priced-publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "priced-calls" and store it as "meterId"
    And I generate a unique resource name from "priced-plan" and store it as "planId"
    And I generate a unique resource name from "priced-api" and store it as "apiHandle"
    And I generate a unique API context from "/priced-publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","description":"Calls the plan charges for","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Metered ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]}}
      """
    Then the response status code should be 201
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planId}"
    And I generate a unique resource name from "priced-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}   |
      | version             | ${CTX:apiVersion}  |
      | subscriptionPlanIds | ["${CTX:planId}"]  |
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the JSON response field "usage.drafts" should be 1
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And I register API Portal resource "${CTX:planId}" at "/subscription-plans" for cleanup
    And I register API Portal resource "${CTX:meterId}" at "/meters" for cleanup
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planId}"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the JSON response field "usage.publications" should be 1

    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Priced calls ${CTX:meterId}"
    And the JSON response field "description" should be "Calls the plan charges for"
    And the JSON response field "filterFields[0].eventKey" should be "response.status"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}/pricing" as "admin"
    Then the response status code should be 200
    And the JSON response field "pricingType" should be "USAGE_BASED"
    And the JSON response field "prices[0].meterId" should be "${CTX:meterId}"
    And the JSON response field "prices[0].unitAmount" should be "0.02"
    And the JSON response field "prices[0].filters[0].values[0]" should be "200"

    # Publishing again sends the same price, which the portal already has.
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}   |
      | version             | ${CTX:apiVersion}  |
      | subscriptionPlanIds | ["${CTX:planId}"]  |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200

    # Publishing brings a new limit to a plan the API Portal already sells at the same price
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "priced-publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "priced-calls" and store it as "meterId"
    And I generate a unique resource name from "priced-plan" and store it as "planId"
    And I generate a unique resource name from "priced-api" and store it as "apiHandle"
    And I generate a unique API context from "/priced-publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","description":"Calls the plan charges for","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Metered ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]}}
      """
    Then the response status code should be 201
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planId}"
    And I generate a unique resource name from "priced-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}   |
      | version             | ${CTX:apiVersion}  |
      | subscriptionPlanIds | ["${CTX:planId}"]  |
    Given a unique API Portal resource is created at "/meters" as "admin" with body and stored as "meterId":
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","description":"Calls the plan charges for","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Metered ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":300,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]}}
      """
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response field "limits[0].limitCount" should be 600
    And the JSON response field "pricing.prices[0].unitAmount" should be "0.02"

    # A meter and a usage-priced plan made in the API Portal appear in the control plane, and leave with them
    Given the gateway services are running
    And a webhook subscriber targeting the control plane is registered in the API Portal for events "meter.*, subscription_plan.*"
    And I generate a unique resource name from "portal-made-calls" and store it as "meterId"
    And I generate a unique resource name from "portal-made-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/meters" as "admin" with body and stored as "meterId":
      """
      {"id":"${CTX:meterId}","displayName":"Portal-made calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    And the control plane meter "${CTX:meterId}" is registered for cleanup
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Portal-made ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":300,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.03","unitQuantity":1}]}}
      """
    And the control plane subscription_plan "${CTX:planId}" is registered for cleanup

    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}" until the JSON field "displayName" is "Portal-made calls ${CTX:meterId}"
    Then the JSON response field "filterFields[0].eventKey" should be "response.status"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}" until the JSON field "displayName" is "Portal-made ${CTX:planId}"
    Then the JSON response field "limits[0].limitCount" should be 300
    And the JSON response field "pricing.pricingType" should be "USAGE_BASED"
    And the JSON response field "pricing.prices[0].meterId" should be "${CTX:meterId}"
    And the JSON response field "pricing.prices[0].unitAmount" should be "0.03"

    # Renaming the plan in the portal renames it in the control plane; the price stays.
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON body:
      """
      {"id":"${CTX:planId}","displayName":"Portal-renamed ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":300,"timeUnit":"MINUTE"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}" until the JSON field "displayName" is "Portal-renamed ${CTX:planId}"
    Then the JSON response field "pricing.prices[0].unitAmount" should be "0.03"

    # Deleting the plan in the portal deletes it in the control plane.
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}" until the JSON field "planCount" is "0"
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 404

  @APIP-048
  Scenario: Conflicting prices and meters block the publish, and control-plane plan changes follow their rules
    # A plan the API Portal already sells at another price blocks the publish
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "priced-publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "priced-calls" and store it as "meterId"
    And I generate a unique resource name from "priced-plan" and store it as "planId"
    And I generate a unique resource name from "priced-api" and store it as "apiHandle"
    And I generate a unique API context from "/priced-publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","description":"Calls the plan charges for","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Metered ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]}}
      """
    Then the response status code should be 201
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planId}"
    And I generate a unique resource name from "priced-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}   |
      | version             | ${CTX:apiVersion}  |
      | subscriptionPlanIds | ["${CTX:planId}"]  |
    Given a unique API Portal resource is created at "/meters" as "admin" with body and stored as "meterId":
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Metered ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":600,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.05","unitQuantity":1}]}}
      """
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_PLAN_PRICING_CONFLICT"
    # The portal keeps its own price.
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}/pricing" as "admin"
    Then the JSON response field "prices[0].unitAmount" should be "0.05"

    # A meter the API Portal defines differently blocks the publish
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "priced-publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "priced-calls" and store it as "meterId"
    And I generate a unique resource name from "priced-plan" and store it as "planId"
    And I generate a unique resource name from "priced-api" and store it as "apiHandle"
    And I generate a unique API context from "/priced-publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","description":"Calls the plan charges for","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Metered ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]}}
      """
    Then the response status code should be 201
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planId}"
    And I generate a unique resource name from "priced-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}   |
      | version             | ${CTX:apiVersion}  |
      | subscriptionPlanIds | ["${CTX:planId}"]  |
    Given a unique API Portal resource is created at "/meters" as "admin" with body and stored as "meterId":
      """
      {"id":"${CTX:meterId}","displayName":"Priced calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Method","eventKey":"request.verb"}]}
      """
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_METER_CONFLICT"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A plan in use cannot be renamed, re-limited or deleted in the control plane, and the API Portal keeps its copy
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "publish-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "publish-api" and store it as "apiHandle"
    And I generate a unique API context from "/publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planHandle}" with body:
      """
      {"id": "${CTX:planHandle}", "displayName": "${CTX:planHandle}", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 10000}]}
      """
    Then the response status code should be 200
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planHandle}" with body:
      """
      {"id": "${CTX:planHandle}", "displayName": "${CTX:planHandle}-renamed", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 10000}]}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planHandle}" with body:
      """
      {"id": "${CTX:planHandle}", "displayName": "${CTX:planHandle}", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 5}]}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    When I send a "DELETE" request to the control plane at "/subscription-plans/${CTX:planHandle}"
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planHandle}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:planHandle}"
    And the JSON response field "limits[0].limitCount" should be 10000
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planHandle}" as "admin"
    Then the response status code should be 200
    And the JSON response field "limits[0].limitCount" should be 10000
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"

    # A plan change made while it is unused reaches the API Portal on the next publish
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "publish-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "publish-api" and store it as "apiHandle"
    And I generate a unique API context from "/publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-spare-plan" and store it as "sparePlan"
    And I create a subscription plan "${CTX:sparePlan}" allowing 20 requests per hour via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}                          |
      | version             | ${CTX:apiVersion}                         |
      | subscriptionPlanIds | ["${CTX:planHandle}", "${CTX:sparePlan}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 200
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:sparePlan}" with body:
      """
      {"id": "${CTX:sparePlan}", "displayName": "${CTX:sparePlan}", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 40}]}
      """
    Then the response status code should be 200
    # The control plane does not send the change on its own; the portal keeps its copy.
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:sparePlan}" as "admin"
    Then the response status code should be 200
    And the JSON response field "limits[0].limitCount" should be 20
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}                          |
      | version             | ${CTX:apiVersion}                         |
      | subscriptionPlanIds | ["${CTX:planHandle}", "${CTX:sparePlan}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:sparePlan}" as "admin"
    Then the JSON response field "limits[0].limitCount" should be 40
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:sparePlan}"
