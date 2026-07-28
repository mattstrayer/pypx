package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// openAPIVersion is the spec version emitted. 3.1 is required for the
// ["T","null"] type-array nullability used below (3.0's `nullable` keyword is
// deliberately not emitted).
const openAPIVersion = "3.1.0"

// apiVersion is the version reported in info.version. The API is unversioned
// in its URLs; this tracks the shape of the contract.
const apiVersion = "1.0.0"

// fieldSchemaOverrides mirrors fieldTypeOverrides for the OpenAPI emitter:
// well-known string enumerations that Go models as a plain string.
// Key format: "TypeName.json_field_name".
var fieldSchemaOverrides = map[string][]string{
	"TypeSupport.status": {"typed", "stubs", "untyped"},
	"DocSymbol.kind":     {"function", "class", "exception"},
}

// buildOpenAPI assembles the OpenAPI 3.1 document for the given contract types
// and route table.
func buildOpenAPI(types []any, defs []routeDef) (map[string]any, error) {
	registered := make(map[string]bool, len(types))
	for _, v := range types {
		t := reflect.TypeOf(v)
		if t == nil || t.Kind() != reflect.Struct {
			return nil, fmt.Errorf("contract entry %T is not a struct", v)
		}
		registered[t.Name()] = true
	}

	schemas := make(map[string]any, len(types))
	for _, v := range types {
		t := reflect.TypeOf(v)
		schemas[t.Name()] = structSchema(t, registered)
	}

	paths := make(map[string]any, len(defs))
	usedIDs := make(map[string]bool, len(defs))
	for _, d := range defs {
		if _, dup := paths[d.Path]; dup {
			return nil, fmt.Errorf("duplicate route path %q", d.Path)
		}
		if d.ResponseRef != "" && !registered[d.ResponseRef] {
			return nil, fmt.Errorf("route %s references unknown schema %q", d.Path, d.ResponseRef)
		}
		paths[d.Path] = map[string]any{"get": operation(d, false, usedIDs)}

		if d.TextTwin {
			twin := d.Path + ".txt"
			if _, dup := paths[twin]; dup {
				return nil, fmt.Errorf("duplicate route path %q", twin)
			}
			paths[twin] = map[string]any{"get": operation(d, true, usedIDs)}
		}
	}

	return map[string]any{
		"openapi": openAPIVersion,
		"info": map[string]any{
			"title":   "pypx API",
			"version": apiVersion,
			"description": "A modern, agent-friendly frontend for the Python Package Index. " +
				"Every JSON endpoint listed here that has a `.txt` sibling also honours " +
				"`Accept: text/plain`, returning the plain-text representation instead. " +
				"All responses are cacheable; requests are rate limited to 30 req/s " +
				"(burst 60) per client, which is reported via the RateLimit-* headers " +
				"and a 429 response.",
			"license": map[string]any{
				"name":       "MIT",
				"identifier": "MIT",
			},
		},
		"servers": []any{
			map[string]any{"url": "https://pypx.app", "description": "Production"},
		},
		"paths":      paths,
		"components": map[string]any{"schemas": schemas},
	}, nil
}

// marshalOpenAPI renders the document as 2-space-indented JSON with a
// trailing newline.
func marshalOpenAPI(doc map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	// Encode already appends the trailing newline.
	return buf.Bytes(), nil
}

// operation builds the GET operation object for a route. When txt is true it
// describes the ".txt" twin: same parameters (plus TwinParams) but a
// text/plain response.
func operation(d routeDef, txt bool, usedIDs map[string]bool) map[string]any {
	path := d.Path
	if txt {
		path += ".txt"
	}
	text := txt || (d.ResponseRef == "" && d.Inline == nil)

	params := make([]paramDef, 0, len(d.Params)+len(d.TwinParams))
	params = append(params, d.Params...)
	if txt {
		params = append(params, d.TwinParams...)
	}

	summary := d.Summary
	if txt {
		// Rendered doc UIs list summaries only, so the twin must be
		// distinguishable from its JSON sibling at a glance.
		summary += " (text)"
	}

	op := map[string]any{
		"operationId": operationID(path, usedIDs),
		"summary":     summary,
		"description": description(d, txt, text),
		"responses":   responses(d, text),
	}
	if len(params) > 0 {
		op["parameters"] = parameters(params)
	}
	return op
}

// description composes the operation description from the route's own prose
// plus generated notes about the timeout and Accept negotiation.
func description(d routeDef, txt, text bool) string {
	var parts []string
	if d.Description != "" {
		parts = append(parts, d.Description)
	}
	switch {
	case txt:
		parts = append(parts, "Plain-text representation of GET "+d.Path+
			". Requesting the JSON path with `Accept: text/plain` returns this same body.")
	case d.TextTwin:
		parts = append(parts, "Sending `Accept: text/plain` returns the plain-text representation "+
			"served at `"+d.Path+".txt` instead of JSON.")
	case !text:
		parts = append(parts, "This route has no plain-text twin; `Accept: text/plain` is ignored.")
	}
	if d.Timeout != "" {
		parts = append(parts, "Server-side timeout: "+d.Timeout+".")
	}
	return strings.Join(parts, " ")
}

// parameters renders the OpenAPI parameter objects.
func parameters(defs []paramDef) []any {
	out := make([]any, 0, len(defs))
	for _, p := range defs {
		schema := map[string]any{"type": p.Type}
		if len(p.Enum) > 0 {
			enum := make([]any, len(p.Enum))
			for i, v := range p.Enum {
				enum[i] = v
			}
			schema["enum"] = enum
		}
		entry := map[string]any{
			"name":     p.Name,
			"in":       p.In,
			"required": p.Required || p.In == "path",
			"schema":   schema,
		}
		if p.Description != "" {
			entry["description"] = p.Description
		}
		out = append(out, entry)
	}
	return out
}

// responses builds the responses object: the 200 body plus the error statuses
// the router and handlers can actually produce. Errors are always plain text
// (net/http's http.Error).
func responses(d routeDef, text bool) map[string]any {
	var success map[string]any
	switch {
	case text:
		success = map[string]any{
			"description": "Plain-text representation.",
			"content": map[string]any{
				"text/plain": map[string]any{"schema": map[string]any{"type": "string"}},
			},
		}
	default:
		var schema map[string]any
		switch {
		case d.Inline != nil:
			schema = d.Inline
		case d.ArrayOf:
			schema = map[string]any{
				"type":  "array",
				"items": map[string]any{"$ref": "#/components/schemas/" + d.ResponseRef},
			}
		default:
			schema = map[string]any{"$ref": "#/components/schemas/" + d.ResponseRef}
		}
		success = map[string]any{
			"description": "Success.",
			"content":     map[string]any{"application/json": map[string]any{"schema": schema}},
		}
	}

	out := map[string]any{"200": success}

	// A {name} route validates the package name before doing any work
	// (handler.validateName), so it can 400 even with no query parameters.
	named := strings.Contains(d.Path, "{name}")
	switch {
	case named && hasRequiredQuery(d):
		out["400"] = textResponse("Invalid package name, or a missing or invalid query parameter.")
	case named:
		out["400"] = textResponse("Invalid package name.")
	case hasRequiredQuery(d):
		out["400"] = textResponse("Missing or invalid query parameter.")
	}
	if named {
		out["404"] = textResponse("Package not found on PyPI.")
	}
	out["429"] = textResponse("Rate limit exceeded. See the RateLimit-* and Retry-After headers.")
	out["default"] = textResponse("Error. Upstream failures surface as 502; internal failures as 500.")

	return out
}

func hasRequiredQuery(d routeDef) bool {
	for _, p := range d.Params {
		if p.In == "query" && p.Required {
			return true
		}
	}
	return false
}

func textResponse(desc string) map[string]any {
	return map[string]any{
		"description": desc,
		"content": map[string]any{
			"text/plain": map[string]any{"schema": map[string]any{"type": "string"}},
		},
	}
}

// operationID derives a unique, stable camelCase operationId from a path,
// e.g. "/api/packages/{name}.txt" → "getApiPackagesByNameTxt".
func operationID(path string, used map[string]bool) string {
	var b strings.Builder
	b.WriteString("get")
	word := true // next alphanumeric run starts a new capitalized word
	trailingSlash := strings.HasSuffix(path, "/") && path != "/"

	for _, r := range path {
		switch {
		case r == '{':
			b.WriteString("By")
			word = true
		case r == '}':
			word = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if word {
				b.WriteRune(unicode.ToUpper(r))
				word = false
			} else {
				b.WriteRune(r)
			}
		default:
			word = true
		}
	}
	if trailingSlash {
		b.WriteString("Index")
	}

	id := b.String()
	candidate := id
	for i := 2; used[candidate]; i++ {
		candidate = fmt.Sprintf("%s%d", id, i)
	}
	used[candidate] = true
	return candidate
}

// structSchema renders a registered struct as a JSON Schema object.
func structSchema(t reflect.Type, registered map[string]bool) map[string]any {
	props := make(map[string]any, t.NumField())
	var required []any

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, omit, skip := parseJSONTag(f)
		if skip {
			continue
		}
		var schema map[string]any
		if enum, ok := fieldSchemaOverrides[t.Name()+"."+tag]; ok {
			vals := make([]any, len(enum))
			for j, v := range enum {
				vals[j] = v
			}
			schema = map[string]any{"type": "string", "enum": vals}
		} else {
			schema = goTypeToJSONSchema(f.Type, registered)
		}
		props[tag] = schema
		if !omit {
			required = append(required, tag)
		}
	}

	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

// goTypeToJSONSchema converts a Go type into an OpenAPI 3.1 (JSON Schema)
// fragment. It mirrors goTypeToTS: pointers become nullable, registered
// structs become $refs, unregistered structs are inlined.
func goTypeToJSONSchema(t reflect.Type, registered map[string]bool) map[string]any {
	// Unwrap pointer: *T → nullable T.
	if t.Kind() == reflect.Pointer {
		return nullable(goTypeToJSONSchema(t.Elem(), registered))
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		// []byte marshals to a base64 string.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		// Nullability matches the TypeScript contract (goTypeToTS): only
		// pointers are nullable, so a nil slice is documented as an array.
		return map[string]any{
			"type":  "array",
			"items": goTypeToJSONSchema(t.Elem(), registered),
		}
	case reflect.Map:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": goTypeToJSONSchema(t.Elem(), registered),
		}
	case reflect.Struct:
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return map[string]any{"type": "string", "format": "date-time"}
		}
		if registered[t.Name()] {
			return map[string]any{"$ref": "#/components/schemas/" + t.Name()}
		}
		return structSchema(t, registered)
	case reflect.Interface:
		return map[string]any{}
	}
	return map[string]any{}
}

// nullable widens a schema to also admit null, using OpenAPI 3.1 type arrays.
// Schemas that carry no plain "type" (notably $refs) are wrapped in anyOf,
// since keywords alongside $ref cannot be relied on.
func nullable(schema map[string]any) map[string]any {
	typ, ok := schema["type"].(string)
	if !ok {
		if len(schema) == 0 {
			// The empty schema already admits null.
			return schema
		}
		return map[string]any{"anyOf": []any{schema, map[string]any{"type": "null"}}}
	}
	out := make(map[string]any, len(schema))
	for k, v := range schema {
		out[k] = v
	}
	out["type"] = []any{typ, "null"}
	return out
}
