# Config changelog

One file per release, naming what changed in operator config.

Derived from `defaultConfig()` in `pkg/config`, dumped at each release tag, so
the values are the effective defaults the binary applies - not the shipped
template, which can differ.

## Fields

- `renamed` - old key moved to a new path
- `deprecated` - still read, but a newer prefix supersedes it
- `removed` - key no longer read, with the default it used to have
- `defaultChanged` - effective default differs; only matters if the key is unset
- `templateChanged` - shipped template moved but the code default did not; only
  affects an install that copies the template
- `semanticChange` - same key, same value, different meaning
- `nowRequired` - no longer defaulted
- `added` - new keys with their effective default

## Notes

Template and code defaults diverge in places. `analytics.enabled` is `true` in
the template and `false` in `defaultConfig()`; an operator who never sets it
gets `false`. That is why these files are generated from the struct.

`router.access_logs.json_fields` is a map, so its keys are not in the struct
dump. The 1.1.0 renames there come from the template.

`policy_engine.*` and `policy_configurations.*` are untyped maps too, so their
keys cannot be validated at all.

Env var names follow the key path uppercased, so a rename implies the env rename.

## Regenerating

Dump defaults at a tag with a throwaway in-package test that reflects over
`defaultConfig()` koanf tags, then diff consecutive dumps. Tags used: `v1.0.0`,
`v1.1.0`, `v1.2.0-rc3`, `v2026.09.24`, `v2026.10.0.1`.
