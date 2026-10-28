module github.com/wso2/api-platform/cli

go 1.26.10

require (
	github.com/spf13/cobra v1.10.2
	github.com/wso2/api-platform/gateway/gateway-controller v1.0.0
	golang.org/x/term v0.45.0
	golang.org/x/text v0.43.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/getkin/kin-openapi v0.152.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/oapi-codegen/runtime v1.5.0 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/wso2/api-platform/gateway/gateway-controller => ../../gateway/gateway-controller
