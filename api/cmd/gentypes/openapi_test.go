package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// buildTestDoc builds the OpenAPI document from the real contract + route
// table and returns both the map form and its marshalled bytes.
func buildTestDoc(t *testing.T) (map[string]any, []byte) {
	t.Helper()
	doc, err := buildOpenAPI(contract, routes)
	if err != nil {
		t.Fatalf("buildOpenAPI: %v", err)
	}
	buf, err := marshalOpenAPI(doc)
	if err != nil {
		t.Fatalf("marshalOpenAPI: %v", err)
	}
	return doc, buf
}

func TestOpenAPIHeader(t *testing.T) {
	doc, _ := buildTestDoc(t)

	if got := doc["openapi"]; got != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", got)
	}
	info, ok := doc["info"].(map[string]any)
	if !ok {
		t.Fatalf("info is %T, want map", doc["info"])
	}
	if got := info["title"]; got != "pypx API" {
		t.Errorf("info.title = %v, want pypx API", got)
	}
	if info["version"] == "" || info["version"] == nil {
		t.Error("info.version must be non-empty")
	}
	if _, ok := doc["servers"].([]any); !ok {
		t.Errorf("servers is %T, want []any", doc["servers"])
	}
}

func TestOpenAPIAllContractSchemasPresent(t *testing.T) {
	doc, _ := buildTestDoc(t)

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatalf("components missing")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("components.schemas missing")
	}

	for _, v := range contract {
		name := reflect.TypeOf(v).Name()
		schema, ok := schemas[name].(map[string]any)
		if !ok {
			t.Errorf("components.schemas.%s missing", name)
			continue
		}
		if schema["type"] != "object" {
			t.Errorf("schema %s type = %v, want object", name, schema["type"])
		}
		if _, ok := schema["properties"].(map[string]any); !ok {
			t.Errorf("schema %s has no properties", name)
		}
	}
}

func TestOpenAPINullableUsesTypeArray(t *testing.T) {
	doc, _ := buildTestDoc(t)
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)

	// enrichment.PlatformCoverage / handler fields that are pointers must
	// render as OpenAPI 3.1 type arrays, never the 3.0 "nullable" keyword.
	if strings.Contains(string(mustJSON(t, schemas)), `"nullable"`) {
		t.Error("schemas use the OpenAPI 3.0 \"nullable\" keyword; 3.1 requires type arrays")
	}
}

func TestGoTypeToJSONSchemaNullability(t *testing.T) {
	registered := map[string]bool{"DateRange": true}

	// Scalar pointer → OpenAPI 3.1 type array.
	var sp *string
	got := goTypeToJSONSchema(reflect.TypeOf(sp), registered)
	typ, ok := got["type"].([]any)
	if !ok || len(typ) != 2 || typ[0] != "string" || typ[1] != "null" {
		t.Errorf("*string schema = %v, want type [string null]", got)
	}

	// Pointer to a registered struct → anyOf, since keywords beside $ref
	// cannot be relied on.
	type holder struct {
		D *dateRangeStub `json:"d"`
	}
	f, _ := reflect.TypeOf(holder{}).FieldByName("D")
	got = goTypeToJSONSchema(f.Type, map[string]bool{"dateRangeStub": true})
	branches, ok := got["anyOf"].([]any)
	if !ok || len(branches) != 2 {
		t.Fatalf("*struct schema = %v, want anyOf with 2 branches", got)
	}
	if branches[1].(map[string]any)["type"] != "null" {
		t.Errorf("second anyOf branch = %v, want {type: null}", branches[1])
	}
}

type dateRangeStub struct {
	Start string `json:"start"`
}

func TestOpenAPIPackagesPathAndTextTwin(t *testing.T) {
	doc, _ := buildTestDoc(t)
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths missing")
	}

	// JSON route.
	pkg, ok := paths["/api/packages/{name}"].(map[string]any)
	if !ok {
		t.Fatalf("paths[/api/packages/{name}] missing")
	}
	get, ok := pkg["get"].(map[string]any)
	if !ok {
		t.Fatalf("get operation missing on /api/packages/{name}")
	}
	ref := digString(get, "responses", "200", "content", "application/json", "schema", "$ref")
	if ref != "#/components/schemas/PackageResponse" {
		t.Errorf("200 schema $ref = %q, want #/components/schemas/PackageResponse", ref)
	}
	// The negotiation behaviour must be documented on the JSON operation.
	if desc, _ := get["description"].(string); !strings.Contains(desc, "Accept: text/plain") {
		t.Errorf("description does not document Accept negotiation: %q", desc)
	}
	// Path parameter must be declared.
	params, _ := get["parameters"].([]any)
	if len(params) == 0 {
		t.Fatal("no parameters on /api/packages/{name}")
	}
	first := params[0].(map[string]any)
	if first["name"] != "name" || first["in"] != "path" || first["required"] != true {
		t.Errorf("unexpected path param: %v", first)
	}

	// .txt sibling.
	txt, ok := paths["/api/packages/{name}.txt"].(map[string]any)
	if !ok {
		t.Fatalf("paths[/api/packages/{name}.txt] missing")
	}
	txtGet, ok := txt["get"].(map[string]any)
	if !ok {
		t.Fatalf("get operation missing on /api/packages/{name}.txt")
	}
	typ := digString(txtGet, "responses", "200", "content", "text/plain", "schema", "type")
	if typ != "string" {
		t.Errorf("text/plain 200 schema type = %q, want string", typ)
	}
	if _, hasJSON := digMap(txtGet, "responses", "200", "content")["application/json"]; hasJSON {
		t.Error(".txt route must not advertise application/json")
	}
}

func TestOpenAPICoversEveryRegisteredRoute(t *testing.T) {
	doc, _ := buildTestDoc(t)
	paths := doc["paths"].(map[string]any)

	// Routes registered in api/cmd/server/main.go that must all be documented,
	// including .txt-only endpoints and the alias root.
	want := []string{
		"/api", "/api/", "/api/health",
		"/api/packages/{name}", "/api/packages/{name}.txt",
		"/api/packages/{name}/versions",
		"/api/packages/{name}/dependencies",
		"/api/packages/{name}/changelog", "/api/packages/{name}/changelog.txt",
		"/api/packages/{name}/stats", "/api/packages/{name}/stats.txt",
		"/api/packages/{name}/security", "/api/packages/{name}/security.txt",
		"/api/packages/{name}/summary.txt",
		"/api/packages/{name}/extras", "/api/packages/{name}/extras.txt",
		"/api/packages/{name}/docs", "/api/packages/{name}/docs.txt",
		"/api/packages/{name}/docs/{symbol}",
		"/api/packages/{name}/symbols.txt",
		"/api/packages/{name}/diff", "/api/packages/{name}/diff.txt",
		"/api/search", "/api/search.txt",
		"/api/compare", "/api/compare.txt",
		"/api/popular", "/api/popular.txt",
		"/api/sitemap/popular", "/api/sitemap/cached",
		"/llms.txt",
	}
	for _, p := range want {
		if _, ok := paths[p]; !ok {
			t.Errorf("paths[%s] missing", p)
		}
	}
	if len(paths) != len(want) {
		t.Errorf("paths has %d entries, want %d", len(paths), len(want))
	}
}

func TestOpenAPISymbolRouteHasNoNegotiationNote(t *testing.T) {
	doc, _ := buildTestDoc(t)
	paths := doc["paths"].(map[string]any)

	// docs/{symbol} is suffix-only: the negotiation middleware deliberately
	// excludes it, so its description must not claim Accept negotiation.
	get := digMap(paths, "/api/packages/{name}/docs/{symbol}", "get")
	desc, _ := get["description"].(string)
	if strings.Contains(desc, "Accept: text/plain") {
		t.Errorf("docs/{symbol} must not document Accept negotiation: %q", desc)
	}
}

func TestOpenAPIRoundTrips(t *testing.T) {
	_, buf := buildTestDoc(t)

	var round map[string]any
	if err := json.Unmarshal(buf, &round); err != nil {
		t.Fatalf("unmarshal generated document: %v", err)
	}
	if round["openapi"] != "3.1.0" {
		t.Errorf("round-tripped openapi = %v", round["openapi"])
	}
	if !strings.HasSuffix(string(buf), "\n") {
		t.Error("generated document must end with a newline")
	}
}

func TestOpenAPIValidates(t *testing.T) {
	_, buf := buildTestDoc(t)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(buf)
	if err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// --- helpers ---

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return buf
}

// digMap walks nested map[string]any keys, returning an empty map on any miss.
func digMap(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return map[string]any{}
		}
		cur = next
	}
	return cur
}

// digString walks nested maps and returns the final key as a string.
func digString(m map[string]any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	leaf := digMap(m, keys[:len(keys)-1]...)
	s, _ := leaf[keys[len(keys)-1]].(string)
	return s
}
