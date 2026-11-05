# Rule: Boolean Configuration Keys Are Named Positively

## Context & Scope

Applies whenever adding a boolean key to any product configuration: a `config.toml` / `config-template.toml` key and the Go struct field that loads it (`koanf:"…"`), a Helm `values.yaml` value, or an environment variable that maps onto one. A negative name (`disable_x = false`) makes every reader invert it twice, and the default reads as the opposite of what it does. A positive name says what turning it on does, whichever way it defaults.

## Directives

1. **Name the feature, not its absence.** A new boolean is `enable_<feature>`, or `enabled` inside the feature's own table (`[router.access_logs] enabled = true`). Never `disable_<feature>`, `no_<feature>`, `skip_<feature>` or `<feature>_disabled`.
2. **Express an opt-out through the default, not the name.** A feature that is on unless an operator turns it off is `enable_<feature> = true` in the template, and the opt-out is setting it to `false`.
3. **A `true` default is set explicitly.** Go's zero value is `false`, so a key that defaults to `true` must be set in the config package's defaults (for example `defaultConfig()`), not left to the zero value. Add a test that loads a config without the key and one with its table present but empty, and asserts both still read `true`.
4. **Shipped negative keys stay as they are.** `disable_keep_alives`, `disable_ssl_verification` and `disable_path_normalization` are in customer config files; renaming a shipped key silently breaks them. Changing one needs a migration: accept the new positive key, keep reading the old one with a deprecation warning, and document both. Do not rename one as a drive-by. List-valued keys that follow an external convention, such as `no_proxy`, are not booleans and are out of scope.
5. **No deferring a violation behind a code comment.** Never ship a new negative key with a `// TODO: rename`-style comment. Name it positively before merging.

## Example

```toml
# BAD: the reader has to invert the name to learn the default is ON.
[policy_engine.llm_errors]
disable_openai_format = false

# GOOD: says what it does; the opt-out is false.
[policy_engine.llm_errors]
enable_openai_format = true
```

```go
// GOOD: a true default is set, not left to the zero value.
func defaultConfig() *Config {
	return &Config{PolicyEngine: PolicyEngine{
		LLMErrors: LLMErrorsConfig{EnableOpenAIFormat: true},
	}}
}
```

> **Verification Checklist before outputting code:**
> * Does a new boolean config key start with `disable_`, `no_` or `skip_`, or end in `_disabled`?
> * Does a key that defaults to `true` rely on Go's zero value instead of an explicit default, and is there a test for both the missing key and an empty table?
> * Is a shipped negative key being renamed without a migration path that still reads the old name?
> * Is a negative name "resolved" by a `TODO` comment instead of being named positively?
