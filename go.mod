module pkg.venceslau.dev/ynab

go 1.25

require github.com/stretchr/testify v1.11.1

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// The archived predecessor's tags (github.com/brunomvsouza/ynab.go)
// were cached by the module proxy under this path before their rename
// to archive/* — they were never valid versions of this module.
retract [v0.1.0, v1.5.0]
