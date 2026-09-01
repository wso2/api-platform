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
