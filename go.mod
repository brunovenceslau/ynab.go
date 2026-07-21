module pkg.venceslau.dev/ynab

go 1.25

require (
	github.com/getkin/kin-openapi v0.143.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Retract everything below the rewrite. v0.1.0 is this module's own
// pre-rewrite tag (the old api/ code, go.mod already declaring this
// path) — the proxy's current @latest, and the most dangerous entry.
// v1.0.0-v1.5.0 are the predecessor's tags: those without a go.mod
// the proxy resolves under this path too, those with one are
// path-mismatch-blocked. Retract removes the whole range from @latest;
// an explicit pin still downloads with a warning, which is why this
// exists.
retract [v0.1.0, v1.5.0]
