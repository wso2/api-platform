# Cloud scenario lists

One scenario list per cloud product feature, `<CODE>.yaml`, for the scenarios under
`suites/cloud`. A list has the same shape as the on-prem lists in `../on-prem/`, with one layer
per entry, `cloud`, in place of `it` and `ui`:

```yaml
code: <CODE>
title: <feature title>
scenarios:
  - id: <CODE>-001
    title: "<action>; <expected result>"
    priority: P1
    cloud:
      status: automated        # automated, known-issue (with issue), exempt (with reason) or planned
      refs:
        - feature: suites/cloud/features/<file>.feature
          scenario: "<exact scenario name>"
```

Tag each cloud scenario with every ID it covers and its feature with the list's code, as the
on-prem suites do. The rules are in "Scenario Lists" in `../../CLAUDE.md`.
