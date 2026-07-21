module ai-workspace-bff

go 1.26.5

require (
	github.com/knadh/koanf/parsers/toml/v2 v2.2.0
	github.com/knadh/koanf/providers/confmap v1.0.0
	github.com/knadh/koanf/providers/file v1.2.1
	github.com/knadh/koanf/v2 v2.3.2
	github.com/wso2/api-platform/common v0.0.0
)

require (
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

replace github.com/wso2/api-platform/common => ../../../common

replace github.com/wso2/go-httpkit => ../../../httpkit
