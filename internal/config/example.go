package config

import _ "embed"

//go:embed config.example.yaml
var exampleYAML string

// ExampleYAML returns the embedded config.example.yaml template.
func ExampleYAML() string {
	return exampleYAML
}
