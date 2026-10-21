# **UIs**

# 1. Summary

| Portal | Screen / Component | Change | Notes |
| :---- | :---- | :---- | :---- |
| Management Console | Create API wizard | Modified | Adds a WebSocket option alongside the existing REST/GraphQL/async options, with an AsyncAPI import step. |
| Management Console | Overview / Deploy / Edit / Publish pages | Modified | Mirror the existing GraphQL pages, with channels shown in place of REST resources. |
| Management Console | Develop page (policy picker) | Modified | Restricts the available policy list to handshake-only policies for this artifact kind, at both API and channel level. |
| Management Console | Test console | New | Environment/channel selection, connect/disconnect, send box, and a timestamped message log. |
| Management Console | Insights / Compliance / Observability pages | Modified | Mirror the existing GraphQL pages, showing connection metrics in place of request metrics. |
| Developer Portal | API documentation page | Modified | The existing AsyncAPI/WebSocket documentation viewer is extended to show live gateway URLs once the API is deployed. |
| Developer Portal | Try-it / tryout | Modified | An existing, currently unimplemented placeholder is completed: connect, send, and receive a message through the published gateway URL. |

# 2. User Flows

**Publisher: create, secure, and test a WebSocket API**

1. The publisher opens the Management Console's create flow and chooses WebSocket.
2. They import an AsyncAPI definition by file, URL, or paste, or start from a plain endpoint; the console shows the channels it derived before the API is created.
3. On the Develop page, they attach policies at the API or channel level; only policies valid for this artifact kind (handshake-only) are offered.
4. They deploy the API to a gateway from the Deploy page.
5. They open the Test console, select an environment and channel, connect with a generated test credential, and send and receive a message to confirm the deployment works.
6. They publish the API to the Developer Portal.

**Application developer: discover and try a WebSocket API**

1. The developer finds the API in the Developer Portal's catalog.
2. They open its documentation page, which shows the AsyncAPI definition with the live gateway URL.
3. They generate an API key.
4. They use the portal's try-it feature to connect, send a message, and see the reply.

![][image1]

# 3. Screen: Create API Wizard (Management Console)

**Purpose**: Let a publisher create a new WebSocket API from an AsyncAPI definition or a plain endpoint, previewing the derived channels before committing.

**Entry points**: The console's existing "create API" entry point, after choosing WebSocket as the API kind.

**Permissions**: Same creation permission already required to create any other API kind in this console; no additional permission is introduced for this kind specifically.

**Elements**

| Element | Type | Behaviour | Validation |
| :---- | :---- | :---- | :---- |
| Source selector (file / URL / paste / endpoint) | Input | Switches the active input method for supplying the AsyncAPI definition | NOT ANSWERED — per-field client-side validation rules are not specified in the inputs |
| Channel preview list | Display | Shows the channels derived from the supplied definition before the API is created | N/A |
| Create button | Action | Submits the definition and creates the API | Disabled until a definition has been supplied and parsed successfully |

**States**

| State | Presentation |
| :---- | :---- |
| Loading | While the supplied definition is being parsed and channels derived |
| Empty (no data yet) | No definition supplied yet; create is disabled |
| Populated | Derived channels shown, create enabled |
| Error | A generic message when the definition cannot be parsed, or is rejected because it has no WebSocket server and no explicit upstream override — no internal parser detail shown to the user |
| Read-only / no permission | NOT ANSWERED — not specified in the inputs |

**Messages**

| Trigger | Type | Text |
| :---- | :---- | :---- |
| Definition parsed successfully | Success | NOT ANSWERED — exact copy not specified in the inputs |
| Definition rejected (no WebSocket server, invalid document) | Inline error | NOT ANSWERED — exact copy not specified in the inputs |

# 4. Screen: Develop Page — Policy Attachment (Management Console)

**Purpose**: Let a publisher attach authentication, authorization, and other policies to the API or to an individual channel.

**Entry points**: The API's Develop page, reached from its overview once created.

**Permissions**: Same policy-management permission already required on the equivalent GraphQL API's Develop page.

**Elements**

| Element | Type | Behaviour | Validation |
| :---- | :---- | :---- | :---- |
| Policy picker (API level) | Input | Offers only policies valid for this artifact kind — handshake-only policies | A policy that requires the request or response message body is not offered here at all |
| Policy picker (channel level) | Input | Same restriction, scoped to one channel | Same |

**States**

| State | Presentation |
| :---- | :---- |
| Loading | While the available policy list loads |
| Empty (no data yet) | No policies attached yet |
| Populated | Attached policies shown, in order |
| Error | Generic message if a policy fails to attach; the specific validation reason (for example, naming a non-handshake-only policy) is shown without exposing internal detail |
| Read-only / no permission | NOT ANSWERED — not specified in the inputs |

**Messages**

| Trigger | Type | Text |
| :---- | :---- | :---- |
| Policy attached | Success | NOT ANSWERED — exact copy not specified in the inputs |
| Policy rejected (not handshake-only) | Inline error | Names the offending policy (per the underlying validation rule); exact copy NOT ANSWERED |

# 5. Screen: Test Console (Management Console)

**Purpose**: Let a publisher verify a deployed WebSocket API works, before publishing it, by connecting to it directly and exchanging messages.

**Entry points**: From a deployed API's page.

**Permissions**: Same permission already required to view a deployed API's details; no separate permission is introduced for using the test console.

**Elements**

| Element | Type | Behaviour | Validation |
| :---- | :---- | :---- | :---- |
| Environment selector | Input | Chooses which deployment (main/sandbox) to connect to | N/A |
| Channel selector | Input | Chooses which channel to connect to | N/A |
| Channel parameter inputs | Input | Filled in for any path parameters the selected channel declares | NOT ANSWERED — per-parameter validation rules not specified in the inputs |
| Connect / Disconnect button | Action | Opens or closes the connection, directly from the browser to the gateway, using a generated test key or token sent as a query parameter or in the WebSocket subprotocol negotiation (since the browser cannot set a custom header on the handshake) | N/A |
| Send box | Input | Sends a message over the open connection | N/A |
| Message log | Display | Timestamped log of sent and received messages | N/A |

**States**

| State | Presentation |
| :---- | :---- |
| Loading | While the connection is being established |
| Empty (no data yet) | No messages sent or received yet |
| Populated | Message log showing exchanged messages |
| Error | Generic message on a failed or rejected handshake — no internal rejection detail or raw credential shown |
| Read-only / no permission | NOT ANSWERED — not specified in the inputs |

**Messages**

| Trigger | Type | Text |
| :---- | :---- | :---- |
| Connected | Success | NOT ANSWERED — exact copy not specified in the inputs |
| Handshake rejected | Inline error | Generic message; the specific rejection reason is not shown to the user |

# 6. Screen: API Documentation Page (Developer Portal)

**Purpose**: Let an application developer read a WebSocket API's AsyncAPI definition, see its live connection URL, and obtain API keys.

**Entry points**: The portal's API catalog.

**Permissions**: Same discovery/subscription permission model already applied to every other API kind in the portal.

**Elements**

| Element | Type | Behaviour | Validation |
| :---- | :---- | :---- | :---- |
| AsyncAPI definition viewer | Display | Renders the API's channels and, once deployed, the live gateway URL | N/A |
| Generate API key button | Action | Issues a new API key for the subscribing application | Same validation already applied to API-key issuance for every other kind |

**States**

| State | Presentation |
| :---- | :---- |
| Loading | While the definition loads |
| Empty (no data yet) | N/A — a published API always has a definition |
| Populated | Definition and live URL shown |
| Error | Generic message if the definition cannot be loaded |
| Read-only / no permission | A developer without subscription permission sees the documentation but cannot generate a key — exact presentation NOT ANSWERED |

# 7. Screen: Try-it (Developer Portal)

**Purpose**: Let an application developer confirm a WebSocket API works before writing integration code against it, by connecting through the portal's try-it feature.

**Entry points**: The API's documentation page.

**Permissions**: Requires a valid API key for the subscribing application, the same as every other kind's try-it feature.

**Elements**

| Element | Type | Behaviour | Validation |
| :---- | :---- | :---- | :---- |
| Connect button | Action | Opens a connection to the published gateway URL | N/A |
| Send box | Input | Sends a message over the open connection | N/A |
| Message log | Display | Shows sent and received messages | N/A |

**States**

| State | Presentation |
| :---- | :---- |
| Loading | While the connection is being established |
| Empty (no data yet) | No messages exchanged yet |
| Populated | Message log showing exchanged messages |
| Error | Generic message on a failed or rejected handshake |
| Read-only / no permission | A developer without a valid API key cannot connect — exact presentation NOT ANSWERED |

**Messages**

| Trigger | Type | Text |
| :---- | :---- | :---- |
| Connected | Success | NOT ANSWERED — exact copy not specified in the inputs |
| Handshake rejected | Inline error | Generic message; specific rejection reason not shown |

# 8. Cross-cutting UI Concerns

* **Responsiveness**: NOT ANSWERED — not specified in the inputs.
* **Accessibility**: NOT ANSWERED — not specified in the inputs.
* **Internationalization**: NOT ANSWERED — not specified in the inputs; any new user-facing string introduced by the create wizard, develop page, test console, documentation page, or try-it feature would need to go through the existing resource-bundle mechanism, consistent with every other screen in these portals.
* **Output encoding**: The test console's message log and the try-it feature's message log both render content a counterparty sent over the connection — this is user-supplied data displayed in the UI and must be contextually encoded on output, the same as any other reflected or stored value in these portals.

# 9. Open UI Questions

| Question | Owner | Status |
| :---- | :---- | :---- |
| Exact success/error message copy for the create wizard, policy picker, test console, and try-it feature. | NOT ANSWERED | Open |
| Read-only/no-permission presentation for each screen above. | NOT ANSWERED | Open |
| Responsiveness, accessibility, and internationalization treatment for the new screens. | NOT ANSWERED | Open |

<!-- [image1]: <path-or-data-uri> -->
