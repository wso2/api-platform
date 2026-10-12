# **UIs**

<!--
Every screen the feature adds or changes, with the states a user can land in. One "Screen"
section per screen. Attach mockups/screenshots as images referenced from here.
-->

# 1. Summary

| Portal | Screen / Component | Change | Notes |
| :---- | :---- | :---- | :---- |
| \<Developer Portal / Management Portal / AI Workspace\> | \<screen\> | New / Modified | \<...\> |

# 2. User Flows

<!-- The path a user takes end to end, including where they enter and where they end up. -->

**\<Flow name\>**

1. \<step - what the user does\>
2. \<step - what the system shows\>

![][image1]

# 3. Screen: \<name\>

**Purpose**: \<what the user accomplishes here.\>

**Entry points**: \<how the user reaches this screen.\>

**Permissions**: \<which role/scope sees this screen; what a lower-privileged user sees instead.\>

![][image2]

**Elements**

| Element | Type | Behaviour | Validation |
| :---- | :---- | :---- | :---- |
| \<field / button / table column\> | \<input / action / display\> | \<what it does\> | \<client-side rules, max length\> |

**States**

<!-- Do not skip empty, loading, and error — they are the states most often left unspecified. -->

| State | Presentation |
| :---- | :---- |
| Loading | \<...\> |
| Empty (no data yet) | \<message and the primary call to action\> |
| Populated | \<...\> |
| Error | \<generic user-facing message; no stack traces or internal identifiers\> |
| Read-only / no permission | \<what is hidden vs. disabled\> |

**Messages**

| Trigger | Type | Text |
| :---- | :---- | :---- |
| \<action succeeded\> | Success | \<text\> |
| \<validation failed\> | Inline error | \<text\> |

# 4. Cross-cutting UI Concerns

* **Responsiveness**: \<breakpoints or layouts that need special handling.\>
* **Accessibility**: \<keyboard navigation, labels, contrast.\>
* **Internationalization**: \<new strings added to the resource bundle; no concatenated sentences.\>
* **Output encoding**: \<any user-supplied value rendered here, and how it is escaped/sanitized.\>

# 5. Open UI Questions

| Question | Owner | Status |
| :---- | :---- | :---- |
| \<question\> | \<name\> | Open / Resolved |

<!-- [image1]: <path-or-data-uri> -->
<!-- [image2]: <path-or-data-uri> -->
