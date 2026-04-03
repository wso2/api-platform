## Problem Statement

- Currently Platform API doesn’t support Subscriptions or Subscription Validation.
- Currently, Bijira Cloud GW subscriptions are bound to an application and we need to eliminate this dependency, to allow multiple applications to use the same subscription plan without having to purchase a new subscription. 
- Even though subscriptions are not needed for an API, the current solution mandates creation of a subscription to invoke an API.
- When the API is using a different security policy other than Oauth, unable to track the subscription information without a clientId.

### Who are we solving the problem for?

- Org Admins
- API Developers.
- Application developers.

### Why should it be solved?

- Organizations would want to purchase subscriptions and use the APIs irrespective of the application that is using the APIs. Decoupling the subscriptions from the application will allow an organization to use a subscription across applications.
- Every API will not require a subscription and needs to be invoked without creating a subscription.
- API subscription validation should not be dependent on the security policy used for the API.

## Use Cases

### Bijira Console use case

- Org admins need to enable default/custom subscription plans with or without usage plan.
- API Developers need to enable/disable subscription validation for an API.
- API Developers need to assign available subscription plans for an API.

### Developer portal use case

- App Developers need to invoke the APIs which don't require subscription without a subscription.
- App Developers need to invoke the APIs which require subscription by creating a subscription.

## Proposed Solution

### User Flow

1. Admin User Flow: Onboard and Select Subscription Plans for an organization.

- Go to Admin -> Subscription Plans to enable Subscription Plan for the org.

<img width="1728" height="911" alt="Screenshot 2026-03-05 at 19 28 28" src="https://github.com/user-attachments/assets/a261aa72-e954-4c30-895c-0d847700552f" />

- Can add custom subscription plans.

2. API Developers select subscription plans available for the API.

- Go to API -> Subscriptions Page to select the available subscriptions.
- When creating or updating an API with `subscriptionPlans`, platform-API validates that each plan exists in the organization and is ACTIVE; invalid plans return 400.

<img width="1728" height="668" alt="Screenshot 2026-03-05 at 19 33 57" src="https://github.com/user-attachments/assets/a5bf8290-46f6-4ba5-ada4-a1bfd04e6b55" />



3. API developers enable subscription validation for the API.

- Go to API -> Policies Page and select API level policies.
- Attach “subscription-validation” policy and save the API.
- Deploy and Publish the API

4. App Developers Discover this API and Subscribe.

- Go to Devportal -> Select API -> API Overview Page.

<img width="3091" height="1605" alt="image" src="https://github.com/user-attachments/assets/d717de04-8a81-4b74-9117-ae7d6f34d05b" />

- Select a Plan and Click Subscribe.

<img width="3091" height="1767" alt="image" src="https://github.com/user-attachments/assets/797336a4-235f-46ba-aa41-d9126347f83b" />

- Receive a subscription Token.

5. Go to Try out(documentation) and invoke with Subscription Token.

#### If this API has no Authorization enabled
- Invoke the API with Subscription Token

#### If this API has Authorization enabled with an OAuth2/API Key Policy.

- Create an App
- Generate relevant Authorization Token.
- Invoke the API with Both Subscription Token and Authorization Token.

<img width="3663" height="2230" alt="image" src="https://github.com/user-attachments/assets/0340f731-051d-4ba0-8a06-52009bbfb789" />

### Implementation

1.  Org Admins will have a way to define subscription plans relevant to self hosted GW for an Organization.

- Implement a default set of plans.(Ex: Gold, Silver, Bronze, Unlimited)
- Implement a UI/REST/DB to store to default and custom plans.
- Implement /subscription-plans REST API in platform-api and gateway-controller to store the plans and to select active plans for organization.

```
GET/POST/PUT/DELETE /subscription-plans?organizationId=1234
		{ 
"planName": "Gold"
       "billingPlan": "Free/Commerical"
       "stopOnQuotaReach": true
       "throttleLimitCount": 10000
       "throttleLimitUnit": "Min/Hour/Day/Month"
       "expiryTime": "2027-03-07"
       "status": "Active"
   		}
```
		
DB Changes

```
CREATE TABLE IF NOT EXISTS subscription_plans (
   uuid VARCHAR(40) PRIMARY KEY,
   plan_name VARCHAR(40) NOT NULL,
   billing_plan VARCHAR(255),
  		   stop_on_quota_reach BOOLEAN DEFAULT TRUE,
   throttle_limit_count VARCHAR(255),
   throttle_limit_unit VARCHAR(255),
   expiry_time DATETIME DEFAULT CURRENT_TIMESTAMP,

   organization_uuid VARCHAR(40) NOT NULL,
   status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
   created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
   updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
   FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
   UNIQUE(uuid, plan_name),
   CHECK (status IN ('ACTIVE', 'INACTIVE'))
);
```

- Re-use same existing subscription policies with modification to add expiry time.

<img width="1728" height="911" alt="Screenshot 2026-03-05 at 19 28 28" src="https://github.com/user-attachments/assets/514169f0-1723-427a-aa10-a9d7462d3795" />


2. API developers can select subscription plans available for the API.

- Implement Subscription Menu in the API to select the available plans for API.

<img width="1728" height="668" alt="Screenshot 2026-03-05 at 19 33 57" src="https://github.com/user-attachments/assets/4d6b1852-c783-427e-88df-ccf95a718bd6" />


- Re-use same business plans choreo-apim API JSON payload.

```
policies: [Gold, Silver]
```

- Implement an API yaml spec **subscriptionPlans** to store available subscription policies for API.

```
      apiVersion: [gateway.api-platform.wso2.com/v1alpha1](http://gateway.api-platform.wso2.com/v1alpha1)
      kind: RestApi
      metadata:
        name: weather-api-v1.0
      spec:
        displayName: Weather-API
        version: v1.0
        context: /weather/$version
        subscriptionPlans:
          - Gold
          - Silver
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: POST
            path: /alerts/active
```
3. API developers can enable subscription validation for the API.


- Implement an API level policy to enable subscription validation.

```
      apiVersion: [gateway.api-platform.wso2.com/v1alpha1](http://gateway.api-platform.wso2.com/v1alpha1)
      kind: RestApi
      metadata:
        name: weather-api-v1.0
      spec:
        displayName: Weather-API
        version: v1.0
        context: /weather/$version
        subscriptionPlans:
          - Gold
	  - Silver
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: POST
            path: /alerts/active
        policies:
              - name: subscription-validation
                version: v0
                params:
                  subscriptionKeyHeader: Subscription-Key
```                   


This policy will be available in the policy hub and API developers can attach it to the API to enable subscription validation.

**Option 1**

Attach this policy automatically when subscription plans are getting enabled for the API.

**Option 2**

API developers need to manually attach the subscription validation policy to the API.


4. App developers can invoke the API without subscription if the subscription validation is not enabled for that API.

- Only OAuth2/API-Key/Basic Authorization token validation will happen(existing behaviour).

5. App developers select the subscription plan and subscribe to the API without an Application from the API Overview Page.

<img width="3060" height="640" alt="image" src="https://github.com/user-attachments/assets/af236e22-1c14-4cb1-8981-34cbd3129791" />

- This option is only available for Self Hosted GW APIs.
- Subscription Token will be generated(Opaque String) and will be visible in the API Overview UI.
- Implement UI/REST CRUD/DB for Subscription Token in Platform-API+Choreo APIM (existing Choreo `/subscriptions` flows that require `applicationId` do not match this contract; here **`applicationId` is optional**).
- **Create subscription (Platform-API JSON):** **`apiId`** and **`subscriberId`** are **required**. **`subscriptionPlanId`**, **`applicationId`**, and **`status`** are **optional**. The server generates **`subscriptionToken`** (opaque); clients do **not** send it on `POST`.
- Subscription token is stored in the platform-api DB (encrypted at rest), propagated to the gateway, and stored in the gateway DB for validation.

Following REST APIs persist subscriptions (Platform-API uses **camelCase** in JSON; the database uses **snake_case** columns below).

```
POST   /api/v1/subscriptions
         Body (required):  apiId, subscriberId
         Body (optional): subscriptionPlanId, applicationId, status
         Response: includes subscriptionToken (generated), subscriptionPlanId (if set), subscriberId, apiId, etc.

GET    /api/v1/subscriptions?apiId={apiId}&subscriberId={subscriberId}&applicationId={applicationId}&status={status}&limit={n}&offset={n}
         Query parameters are all optional filters; apiId may be API UUID or handle. subscriberId must be non-empty when provided (min length 1).

GET    /api/v1/subscriptions/{subscriptionId}?subscriberId={subscriberId}   (required; must match subscription)
PUT    /api/v1/subscriptions/{subscriptionId}?subscriberId={subscriberId}     e.g. update status
DELETE /api/v1/subscriptions/{subscriptionId}?subscriberId={subscriberId}
```

DB changes (column names match `schema.postgres.sql` / `schema.sqlite.sql`; JSON field `subscriberId` maps to **`subscriber_id`**, `subscriptionPlanId` to **`subscription_plan_uuid`**, etc.)

```
CREATE TABLE IF NOT EXISTS subscriptions (
   uuid VARCHAR(40) PRIMARY KEY,
   api_uuid VARCHAR(40) NOT NULL,
   subscriber_id VARCHAR(255) NOT NULL,
   application_id VARCHAR(255),
   subscription_token VARCHAR(512) NOT NULL,
   subscription_token_hash VARCHAR(64) NOT NULL,
   subscription_plan_uuid VARCHAR(40),
   organization_uuid VARCHAR(40) NOT NULL,
   status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
   created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
   updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
   FOREIGN KEY (api_uuid) REFERENCES rest_apis(uuid) ON DELETE CASCADE,
   FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
   FOREIGN KEY (subscription_plan_uuid, organization_uuid)
     REFERENCES subscription_plans(uuid, organization_uuid) ON DELETE RESTRICT,
   FOREIGN KEY (api_uuid, organization_uuid)
     REFERENCES artifacts(uuid, organization_uuid) ON DELETE CASCADE,
   UNIQUE(api_uuid, subscription_token_hash),
   UNIQUE(api_uuid, subscriber_id, organization_uuid),
   CHECK (status IN ('ACTIVE', 'INACTIVE', 'REVOKED'))
);
```

7. App Developers Invoke the Subscribed API.

- User need to send the **Subscription Token** in the API request, either:
  - **Header:** `Subscription-Key: subscription-Token` (default)
  - **Cookie:** When `subscriptionKeyCookie` is configured, e.g. `Cookie: sub-key=subscription-Token`

- During the API request flow,

#### If the API has subscription validation enabled

- If the request has a subscription-key request header (or token in the configured cookie), use it to check SubscriptionDataStore and validate the subscription for that particular API.
- If the request doesn’t have a subscription-key request header, fall back to validating against **`applicationId`** carried in request metadata (e.g. `x-wso2-application-id` from OAuth2/JWT/API-Key policy claim mappings). This is legacy behaviour for older APIs; it is separate from the optional **`applicationId`** field on the subscription record.
- If this Subscription has a usage limit, rate limit based on the counts.
- Implement subscription-rate-limit keys in SubscriptionDataStore

Ratelimit Key : Subscription Token
Ratelimit Count: Available Count

#### If the API subscription validation is disabled

- No subscription validation happens.