# Govern APIs using API Designer

You can use API Designer to assess and improve your OpenAPI 3.x specification for AI readiness, design quality, and secure production use.

![Governance Report](./images/analyze-view.png)

## Why this matters

- AI agents rely on clear descriptions, examples, and predictable API behavior.
- Early governance helps you improve specification quality before publishing.

## What you can review

API Designer applies governance checks using Spectral rulesets and AI. You can review:

- Overall scores and summary metrics.
- Category-level breakdown to quickly identify weak areas.
- Violations grouped by rule, severity, or area of the specification.

## Reports in API Designer

API Designer includes the following reports:

| Report focus | What it emphasizes |
|---------------|-------------------|
| **WSO2 REST API AI Readiness Guidelines** | Readiness for AI agents and automated clients |
| **WSO2 REST API Design Guidelines** | REST design quality and consistency |
| **OWASP API Security Top 10** | API security best-practice checks |

## AI readiness first

AI readiness is a core governance area in API Designer. It helps you identify specification issues that block reliable agent usage.

Focus on these first:

- Missing or weak operation ids, summaries and descriptions.
- Incomplete request and response examples.
- Ambiguous parameter or schema intent.
- Contract gaps that reduce machine readability.

For each issue, review the provided details to understand what is wrong and why it affects agent behavior.

### AI-assisted analysis (AI readiness)

For **AI readiness**, API Designer combines **rule-based** results with **AI-assisted** narrative analysis: not only *what* failed a rule, but *why it matters* for agents and tools (for example ambiguous descriptions, weak examples, or gaps that cause wrong calls).

You can run AI analysis on demand and get AI feedback to improve your specification.

## How to fix findings

You can resolve findings using:

1. **GitHub Copilot Chat with the `api-design` skill**  
   Use this for targeted fixes or rule-driven remediation.
2. **Form-based editing in Design**  
   Use this for precise changes to operations, parameters, responses, and schemas.

## Important

- Some fixes are **breaking** (for example path renames). Treat those as API **versioning** decisions, not silent edits.
- Security items may require **real** URLs, policies, or auth server metadata—do not invent production values with AI.

## Related topics

- [Design APIs with API Designer](./design-apis.md) — create and refine your specification
- [End-to-end tutorial](./end-to-end-tutorial.md) — full flow from draft to governance
