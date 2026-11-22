module github.com/devsy-org/devsy-signals/tools

go 1.24.0

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/stretchr/testify v1.12.1
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/text v0.14.0 // indirect

require (
	github.com/devsy-org/devsy-signals v0.0.0
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

replace github.com/devsy-org/devsy-signals => ../
