module github.com/deckhouse/lib-dhctl

go 1.25.8

require (
	github.com/deckhouse/deckhouse/pkg/log v0.1.1-0.20251230144142-2bad7c3d1edf
	github.com/go-openapi/spec v0.19.8
	github.com/go-openapi/strfmt v0.19.5
	github.com/go-openapi/validate v0.19.12
	github.com/gookit/color v1.6.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/name212/govalue v1.0.2
	github.com/stretchr/testify v1.11.1
	github.com/werf/logboek v0.5.5
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apimachinery v0.33.12
	k8s.io/klog/v2 v2.140.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	atomicgo.dev/cursor v0.2.0 // indirect
	atomicgo.dev/keyboard v0.2.9 // indirect
	atomicgo.dev/schedule v0.1.0 // indirect
	github.com/DataDog/gostackparse v0.7.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535 // indirect
	github.com/avelino/slugify v0.0.0-20180501145920-855f152bd774 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/analysis v0.19.10 // indirect
	github.com/go-openapi/errors v0.19.7 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/loads v0.19.5 // indirect
	github.com/go-openapi/runtime v0.19.16 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/lithammer/fuzzysearch v1.1.8 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-runewidth v0.0.20 // indirect
	github.com/mitchellh/mapstructure v1.3.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/pterm/pterm v0.12.83 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.mongodb.org/mongo-driver v1.5.1 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

// Remove 'in body' from errors, fix for Go 1.16 (https://github.com/go-openapi/validate/pull/138).
replace github.com/go-openapi/validate => github.com/flant/go-openapi-validate v0.19.12-flant.0
