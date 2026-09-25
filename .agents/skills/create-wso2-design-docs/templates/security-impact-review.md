# Security Impact Review Questionnaire

<!--
Fill every row. Write N/A where a section genuinely does not apply (e.g. no file uploads), but
never leave a row blank. A "Yes" answer should say how it is enforced and where, not just "yes".
If you are not 100% certain of an answer, write "NOT ANSWERED" rather than guessing. A guessed
answer is worse than an open one — it reads as verified to the reviewer.
See also the repo's .claude/rules/*.md security rules, which are the implementation-level form
of most of these questions.
-->

Please adhere to the WSO2 Secure Coding Guidelines for implementation details and best practices: [https://security.docs.wso2.com/en/latest/security-guidelines/secure-engineering-guidelines/secure-coding-guidlines/introduction/](https://security.docs.wso2.com/en/latest/security-guidelines/secure-engineering-guidelines/secure-coding-guidlines/introduction/)

| General Information |  |
| :---- | ----- |
| Submitted by |  |
| Submitted date |  |
| Product and version |  |
| Reviewed and approved by |  |
| Reviewed date |  |
| Github issue |  |
| Public PR |  |
| **Authorization & Access Control** (*Complete if introducing or modifying REST APIs)* |  |
| **Authentication**: Is authentication required for all exposed API endpoints? |  |
| **Authentication Mechanism:** How is authentication enforced?  |  |
| **Privilege Level Defined:** What privilege level is required for the endpoints? (Admin, Publisher, Subscriber, Internal/System, Anonymous) |  |
| **Least Privilege Enforcement:** Is access restricted to the minimum required privilege?(Lower-privileged roles must NOT gain unintended access.) |  |
| **Resource Ownership Check (BOLA / IDOR):** When accessing a resource using an ID in the URL, does the system verify that the authenticated user has permission to access that specific resource? *(Example: Preventing User A from accessing another user’s order via `/api/orders/{orderId}`)* |  |
| Is case sensitivity consistently enforced for user identities and role checks during authorization and other validations? |  |
| **Cross-Tenant Isolation:** Does the feature strictly enforce tenant boundaries? (e.g., preventing Tenant A from accessing Tenant B resources) |  |
| **Mass Assignment Protection:** Are strict DTOs / whitelists used to prevent users from submitting unintended fields (e.g., `is_admin: true`)? |  |
| **User Input & Web Protection** |  |
| **Output Encoding (XSS Prevention):** If user-provided data is rendered in the UI (stored or reflected), is it contextually encoded to prevent XSS? |  |
| **SSRF Prevention:** If the server fetches user-provided URLs, are they validated against a strict allow-list? |  |
| **XXE Prevention:** If parsing XML, are DTDs and external entities explicitly disabled using secure XML parser configurations? |  |
| **Database & Injection Prevention** *(Complete if interacting with the system Databases.)* |  |
| **Parameterized Queries:** Are prepared statements used everywhere in all the database queries? |  |
| **Dynamic Query Validation:** If table names, column names, or query fragments(eg: ‘dec’ and ‘ace’ sorting orders, multiple property map validation) are dynamic, are they validated against a hardcoded whitelist? |  |
| Do the queries can be performed by the minimum DB permissions recommended for the product?  |  |
| **File Processing & Resource Safety**  *(Complete if handling uploads, downloads, or parsing files.)* |  |
| **Path Traversal Protection:** Are filenames and externally provided paths sanitized to prevent path traversal attacks? *(e.g., blocking `../` in input paths/file names)* |  |
| **File Type Validation:** Is file type validated using content (magic bytes), not only extension? |  |
| **File Size Limits:** File Size Limits Enforced as per the requirement? |  |
| Are uploaded files stored outside executable directories? |  |
| **Zip Slip Protection:** Are archives extracted only into a controlled temporary directory with strict path validation to prevent directory traversal? |  |
| **Credential & Default Configuration Management** |  |
| **New Credentials Introduced:** Does this feature introduce any new passwords, keys, tokens, certificates, or other credentials? |  |
| **Default Credentials Handling:** If default credentials are introduced, are they securely generated (not hardcoded or weak defaults)? |  |
| **Product Hardening Documentation:** Are all new credentials (including defaults) documented in the Product Hardening Guide with clear instructions to reconfigure or rotate them before production use? |  |
| **Secure Coding & Operational Safety**  *(Mandatory for all implementations.)* |  |
| **Sensitive Data in Logs:** Any passwords, API keys, tokens, or PII logged?  If logging is mandatory, is proper masking applied? |  |
| **In-Memory Sensitive Data Handling (Heap Inspection Protection)** Are sensitive data (e.g., passwords, encryption keys, tokens) stored using mutable data types (e.g., `char[]`) instead of immutable types like `String`, and explicitly cleared from memory immediately after use to prevent heap inspection attacks? |  |
| **Response and error Handling:** Are client responses generic, and server information or stack traces remain internal? |  |
| **Secrets Management:** No hardcoded credentials. Stored in Secret Manager / Vault / Env. |  |
| **Third-Party Dependency Scan:** Have any newly introduced libraries been scanned for known vulnerabilities and received the required approvals? |  |

