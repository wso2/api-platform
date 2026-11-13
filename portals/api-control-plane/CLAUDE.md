# Project Guidelines

## UI development

All UI work in this portal — pages, components, forms, listings, navigation, theming, Oxygen UI
component/theming API, and MUI→Oxygen migration — follows the `apicp-ui` skill
([.claude/skills/apicp-ui/SKILL.md](.claude/skills/apicp-ui/SKILL.md)). Invoke it with `/apicp-ui`,
or read it directly before touching anything under `src/`.

The full upstream Oxygen UI reference ships with the package at
`node_modules/@wso2/oxygen-ui/.ai/{components,patterns,theming,migration}.md`; the compiled types in
`node_modules/@wso2/oxygen-ui/dist/**/*.d.ts` are the ground truth when the docs and the package
disagree.

## Feature-usage tracking

Whenever UI work adds or changes a control (a button, confirm, wizard step, copy action, or a page
or dialog that commits something), run the `feature-usage-tracking` skill
([.claude/skills/feature-usage-tracking/SKILL.md](.claude/skills/feature-usage-tracking/SKILL.md)).
It decides whether the control is a feature, adds it to `src/featureUsage/api-platform-taxonomy.yaml`,
regenerates `taxonomy.generated.ts`, and wires the `trackFeature` call.
