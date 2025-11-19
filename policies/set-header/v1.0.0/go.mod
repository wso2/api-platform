// T080: SetHeader policy go.mod
module github.com/yourorg/policy-engine/policies/set-header/v1.0.0

go 1.23

require github.com/yourorg/policy-engine v0.0.0

// Local development: replace with parent module path
replace github.com/yourorg/policy-engine => ../../../src
