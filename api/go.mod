module github.com/pypx/api

go 1.27

toolchain go1.27.3

require (
	github.com/getkin/kin-openapi v0.152.0
	github.com/go-chi/chi/v5 v5.5.0
	github.com/go-chi/cors v1.2.2
	github.com/pypx/goopy v0.0.0
	github.com/yuin/goldmark-highlighting/v3 v3.1.0
	github.com/yuin/goldmark/v2 v2.1.9
	golang.org/x/sync v0.24.0
	golang.org/x/time v0.16.0
	modernc.org/sqlite v1.61.0
)

require (
	github.com/alecthomas/chroma/v3 v3.0.0-alpha.5 // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/dlclark/regexp2/v2 v2.10.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-openapi/jsonpointer v1.0.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	modernc.org/libc v1.79.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

replace github.com/pypx/goopy => ../goopy
