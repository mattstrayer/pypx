package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pypx/goopy/docstring"
	"github.com/pypx/goopy/model"
	"golang.org/x/sync/semaphore"
)

func TestCleanDocstringText_Sphinx(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text: `Validate the given value.

Does some important work here.

:param value: The value to validate
:type value: str
:returns: None
:rtype: None
:raises ValueError: If value is empty`,
	}
	got := cleanDocstringText(ds)
	want := "Validate the given value.\n\nDoes some important work here."
	if got != want {
		t.Errorf("cleanDocstringText(sphinx) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_Google(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringGoogle,
		Text: `Fetch the queryset.

Extended description here.

Args:
    pk: Primary key
    qs: Queryset

Returns:
    QuerySet: The filtered queryset`,
	}
	got := cleanDocstringText(ds)
	want := "Fetch the queryset.\n\nExtended description here."
	if got != want {
		t.Errorf("cleanDocstringText(google) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_Plain(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringPlain,
		Text:  "Just a plain description with no fields.",
	}
	got := cleanDocstringText(ds)
	want := "Just a plain description with no fields."
	if got != want {
		t.Errorf("cleanDocstringText(plain) = %q, want %q", got, want)
	}
}

func TestCleanDocstringText_Nil(t *testing.T) {
	if got := cleanDocstringText(nil); got != "" {
		t.Errorf("cleanDocstringText(nil) = %q, want empty", got)
	}
}

func TestCleanDocstringText_NoFields(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text:  "One-liner with no field markers.",
	}
	got := cleanDocstringText(ds)
	if got != "One-liner with no field markers." {
		t.Errorf("unexpected: %q", got)
	}
}

func TestCleanDocstringText_NumPy(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringNumpy,
		Text: `Compute the result.

More detail here.

Parameters
----------
x : int
    The input value.

Returns
-------
int
    The output value.`,
	}
	got := cleanDocstringText(ds)
	want := "Compute the result.\n\nMore detail here."
	if got != want {
		t.Errorf("cleanDocstringText(numpy) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_SphinxProseColon(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text: `Return the result, which is an integer.

Note: this always returns a positive value.

:param x: The input
:type x: int`,
	}
	got := cleanDocstringText(ds)
	// "Note: this..." starts with "Note" not ":", so it must not be truncated.
	if !strings.Contains(got, "Note: this always returns a positive value.") {
		t.Errorf("prose colon was incorrectly truncated: %q", got)
	}
	// The :param line must be stripped.
	if strings.Contains(got, ":param") {
		t.Errorf(":param line not stripped: %q", got)
	}
}

func TestCleanDocstringText_EmptyText(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text:  "",
	}
	got := cleanDocstringText(ds)
	if got != "" {
		t.Errorf("cleanDocstringText(empty text) = %q, want empty", got)
	}
}

func TestConvertFunction_DocstringTypeBackfill(t *testing.T) {
	fn := &model.Function{
		Name: "validate",
		Parameters: []*model.Parameter{
			{
				Name: "value",
				Kind: model.ParamPositionalOrKeyword,
				// No annotation type
				DocParam: &model.DocParam{
					Name:        "value",
					Type:        "str",
					Description: "The value to validate",
				},
			},
			{
				Name: "strict",
				Kind: model.ParamPositionalOrKeyword,
				// Has annotation — should NOT be overridden
				Type: &model.TypeExpr{Raw: "bool"},
				DocParam: &model.DocParam{
					Name: "strict",
					Type: "int", // docstring says int but annotation says bool — annotation wins
				},
			},
		},
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Text:  "Validate the value.\n\n:param value: The value\n:type value: str",
			Returns: &model.DocReturn{
				Type:        "None",
				Description: "Nothing returned",
			},
		},
	}

	sym := convertFunction(fn, nil)

	// value param: annotation absent, docstring type used
	if sym.Parameters[0].Type != "str" {
		t.Errorf("param[0].Type = %q, want str", sym.Parameters[0].Type)
	}
	if sym.Parameters[0].Description != "The value to validate" {
		t.Errorf("param[0].Description = %q", sym.Parameters[0].Description)
	}

	// strict param: annotation present, annotation wins
	if sym.Parameters[1].Type != "bool" {
		t.Errorf("param[1].Type = %q, want bool (annotation wins)", sym.Parameters[1].Type)
	}

	// Returns: no annotation on fn, docstring return used
	if sym.Returns == nil {
		t.Fatal("Returns should not be nil")
	}
	if sym.Returns.Type != "None" {
		t.Errorf("Returns.Type = %q, want None", sym.Returns.Type)
	}
	if sym.Returns.Description != "Nothing returned" {
		t.Errorf("Returns.Description = %q", sym.Returns.Description)
	}
}

func TestConvertFunction_AnnotationTakesPrecedence(t *testing.T) {
	fn := &model.Function{
		Name:    "fetch",
		Returns: &model.TypeExpr{Raw: "QuerySet"},
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Returns: &model.DocReturn{
				Type:        "list",
				Description: "A list of things",
			},
		},
	}

	sym := convertFunction(fn, nil)

	// Annotation return type wins over docstring
	if sym.Returns == nil || sym.Returns.Type != "QuerySet" {
		t.Errorf("Returns.Type = %v, want QuerySet", sym.Returns)
	}
	// Description comes from docstring
	if sym.Returns.Description != "A list of things" {
		t.Errorf("Returns.Description = %q", sym.Returns.Description)
	}
}

func TestConvertFunction_Raises(t *testing.T) {
	fn := &model.Function{
		Name: "delete",
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Text:  "Delete the object.\n\n:raises ValueError: If pk is None\n:raises PermissionError: If not allowed",
			Raises: []*model.DocRaises{
				{Type: "ValueError", Description: "If pk is None"},
				{Type: "PermissionError", Description: "If not allowed"},
			},
		},
	}

	sym := convertFunction(fn, nil)

	if len(sym.Raises) != 2 {
		t.Fatalf("Raises len = %d, want 2", len(sym.Raises))
	}
	if sym.Raises[0].Type != "ValueError" {
		t.Errorf("Raises[0].Type = %q, want ValueError", sym.Raises[0].Type)
	}
	if sym.Raises[0].Description != "If pk is None" {
		t.Errorf("Raises[0].Description = %q", sym.Raises[0].Description)
	}
	if sym.Raises[1].Type != "PermissionError" {
		t.Errorf("Raises[1].Type = %q", sym.Raises[1].Type)
	}
}

func TestConvertFunction_NoRaises(t *testing.T) {
	fn := &model.Function{Name: "noop"}
	sym := convertFunction(fn, nil)
	if sym.Raises != nil {
		t.Errorf("Raises should be nil for function with no docstring raises, got %v", sym.Raises)
	}
}

func TestConvertClass_Methods(t *testing.T) {
	cls := &model.Class{
		Name: "QuerySet",
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Text:  "A lazy database lookup for a set of objects.",
		},
		BaseClasses: []*model.TypeRef{{Name: "object"}},
		Methods: []*model.Function{
			{
				Name: "filter",
				Parameters: []*model.Parameter{
					{Name: "self", Kind: model.ParamPositionalOrKeyword},
					{
						Name: "kwargs",
						Kind: model.ParamVarKeyword,
						DocParam: &model.DocParam{
							Name:        "kwargs",
							Type:        "Any",
							Description: "Field lookups",
						},
					},
				},
				Docstring: &model.Docstring{
					Style: model.DocstringSphinx,
					Text:  "Return a new QuerySet filtered by the given lookups.",
				},
			},
			{
				Name: "first",
				Docstring: &model.Docstring{
					Style:   model.DocstringSphinx,
					Text:    "Return the first object matched.\n\n:rtype: Model or None",
					Returns: &model.DocReturn{Type: "Model or None"},
				},
			},
		},
	}

	sym := convertClass(cls, make(stubIndex), "mymod")

	if len(sym.Methods) != 2 {
		t.Fatalf("Methods len = %d, want 2", len(sym.Methods))
	}
	if sym.Methods[0].Name != "filter" {
		t.Errorf("Methods[0].Name = %q, want filter", sym.Methods[0].Name)
	}
	if sym.Methods[0].Kind != "function" {
		t.Errorf("Methods[0].Kind = %q, want function", sym.Methods[0].Kind)
	}
	// Docstring type backfill on method param (kwargs has no annotation, docstring type used)
	kwargsParam := sym.Methods[0].Parameters[1]
	if kwargsParam.Type != "Any" {
		t.Errorf("kwargs.Type = %q, want Any", kwargsParam.Type)
	}
	// Return type from docstring on method
	if sym.Methods[1].Returns == nil || sym.Methods[1].Returns.Type != "Model or None" {
		t.Errorf("first.Returns = %v, want type 'Model or None'", sym.Methods[1].Returns)
	}
	// Class docstring still present
	if sym.Docstring != "A lazy database lookup for a set of objects." {
		t.Errorf("class Docstring = %q", sym.Docstring)
	}
}

func TestConvertClass_NoMethods(t *testing.T) {
	cls := &model.Class{Name: "Empty"}
	sym := convertClass(cls, make(stubIndex), "mymod")
	if sym.Methods != nil {
		t.Errorf("Methods should be nil for class with no methods, got %v", sym.Methods)
	}
}

func TestConvertClass_ConstructorParameters(t *testing.T) {
	cls := &model.Class{
		Name: "Client",
		Constructor: &model.Function{
			Name: "__init__",
			Parameters: []*model.Parameter{
				{Name: "url", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "str"}, DocParam: &model.DocParam{Description: "Base URL"}},
				{Name: "timeout", Kind: model.ParamPositionalOrKeyword, Default: "5"},
			},
		},
	}
	sym := convertClass(cls, make(stubIndex), "mymod")
	if len(sym.Parameters) != 2 {
		t.Fatalf("Parameters len = %d, want 2", len(sym.Parameters))
	}
	if sym.Parameters[0].Name != "url" || sym.Parameters[0].Type != "str" || sym.Parameters[0].Description != "Base URL" {
		t.Errorf("unexpected param[0]: %+v", sym.Parameters[0])
	}
	if sym.Parameters[1].Default != "5" {
		t.Errorf("param[1].Default = %q, want 5", sym.Parameters[1].Default)
	}
}

func TestConvertClass_NoConstructorEmptyParameters(t *testing.T) {
	sym := convertClass(&model.Class{Name: "Plain"}, make(stubIndex), "mymod")
	if sym.Parameters == nil || len(sym.Parameters) != 0 {
		t.Errorf("Parameters = %#v, want empty non-nil slice", sym.Parameters)
	}
}

func TestConvertFunction_StubTypeBackfill(t *testing.T) {
	// Source function has no type annotation; stub has type info.
	src := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword},
		},
	}
	stub := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "QuerySet[_QS]"},
	}

	sym := convertFunction(src, stub)

	if sym.Returns == nil {
		t.Fatal("convertFunction: Returns should be backfilled from stub")
	}
	if sym.Returns.Type != "QuerySet[_QS]" {
		t.Errorf("convertFunction: Returns.Type = %q, want %q", sym.Returns.Type, "QuerySet[_QS]")
	}
	// self has no type — stub has none either
	if sym.Parameters[0].Type != "" {
		t.Errorf("convertFunction: self.Type = %q, want \"\"", sym.Parameters[0].Type)
	}
	// kwargs filled from stub
	if sym.Parameters[1].Type != "Any" {
		t.Errorf("convertFunction: kwargs.Type = %q, want \"Any\"", sym.Parameters[1].Type)
	}
}

func TestConvertFunction_SourceAnnotationWinsOverStub(t *testing.T) {
	src := &model.Function{
		Name: "save",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "value", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "str"}},
		},
		Returns: &model.TypeExpr{Raw: "None"},
	}
	stub := &model.Function{
		Name: "save",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "value", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "int"},
	}

	sym := convertFunction(src, stub)

	// Source annotation wins
	if sym.Parameters[1].Type != "str" {
		t.Errorf("convertFunction: value.Type = %q, want \"str\" (source wins)", sym.Parameters[1].Type)
	}
	if sym.Returns == nil || sym.Returns.Type != "None" {
		t.Errorf("convertFunction: Returns.Type = %q, want \"None\" (source wins)", sym.Returns.Type)
	}
}

func TestConvertFunction_NilStub(t *testing.T) {
	src := &model.Function{
		Name:       "simple",
		Parameters: []*model.Parameter{{Name: "x", Kind: model.ParamPositionalOrKeyword}},
	}
	// Should not panic with nil stub
	sym := convertFunction(src, nil)
	if sym.Name != "simple" {
		t.Errorf("convertFunction with nil stub: Name = %q, want \"simple\"", sym.Name)
	}
}

func TestConvertFunction_DocstringReturnWinsOverStub(t *testing.T) {
	src := &model.Function{
		Name: "fetch",
		Docstring: &model.Docstring{
			Returns: &model.DocReturn{Type: "Response"},
		},
	}
	stub := &model.Function{
		Name:    "fetch",
		Returns: &model.TypeExpr{Raw: "Any"},
	}
	sym := convertFunction(src, stub)
	if sym.Returns == nil || sym.Returns.Type != "Response" {
		t.Errorf("docstring return should win over stub: Returns = %v", sym.Returns)
	}
}

func TestConvertFunction_SignatureReflectsStubTypes(t *testing.T) {
	src := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword},
		},
	}
	stub := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "QuerySet"},
	}

	sym := convertFunction(src, stub)

	if !strings.Contains(sym.Signature, "kwargs: Any") {
		t.Errorf("Signature should contain stub-filled type: %q", sym.Signature)
	}
	if !strings.Contains(sym.Signature, "-> QuerySet") {
		t.Errorf("Signature should contain stub-filled return type: %q", sym.Signature)
	}
}

// ttlCache is a Cacher that honours the ttl passed to Get, like the SQLite
// cache: fresh = age < ttl. Set records the time from the injected clock.
type ttlCache struct {
	mu   sync.Mutex
	rows map[string]ttlRow
	sets []string
}

type ttlRow struct {
	val []byte
	at  time.Time
}

func newTTLCache() *ttlCache { return &ttlCache{rows: map[string]ttlRow{}} }

func (c *ttlCache) put(key string, val []byte, age time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows[key] = ttlRow{val: val, at: time.Now().Add(-age)}
}

func (c *ttlCache) Get(key string, ttl time.Duration) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.rows[key]
	if !ok {
		return nil, false, nil
	}
	return r.val, time.Since(r.at) < ttl, nil
}

func (c *ttlCache) Set(key string, val []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets = append(c.sets, key)
	c.rows[key] = ttlRow{val: val, at: time.Now()}
	return nil
}

func (c *ttlCache) Delete(key string) error { return nil }
func (c *ttlCache) Close() error            { return nil }

func (c *ttlCache) wrote(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range c.sets {
		if k == key {
			return true
		}
	}
	return false
}

func docsCacheHandler(c *ttlCache, ex func(context.Context, string, string) (*model.Package, error)) *DocsHandler {
	return &DocsHandler{
		cache:       c,
		extract:     ex,
		budget:      semaphore.NewWeighted(extractionBudget),
		workTimeout: docsExtractTimeout,
	}
}

func mustJSON(t *testing.T, v DocsResponse) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDocsCache_ErrorKeyYoungerThanTTLShortCircuits(t *testing.T) {
	c := newTTLCache()
	c.put(DocsErrKey("p", "1"), []byte("boom"), time.Minute)
	calls := 0
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls++
		return &model.Package{}, nil
	})
	_, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
	}
	if calls != 0 {
		t.Errorf("extract called %d times, want 0", calls)
	}
}

func TestDocsCache_ErrorKeyOlderThanTTLRetries(t *testing.T) {
	c := newTTLCache()
	c.put(DocsErrKey("p", "1"), []byte("boom"), 10*time.Minute)
	calls := 0
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls++
		return nil, errors.New("still broken")
	})
	_, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if !errors.Is(err, ErrDocsExtractionFailed) {
		t.Fatalf("err = %v", err)
	}
	if calls != 1 {
		t.Errorf("extract called %d times, want 1", calls)
	}
}

func TestDocsCache_UnavailableYoungerThan24hServed(t *testing.T) {
	c := newTTLCache()
	c.put(DocsCacheKey("p", "1"), mustJSON(t, DocsResponse{Available: false}), time.Hour)
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		t.Fatal("extract must not run")
		return nil, nil
	})
	resp, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if err != nil || resp.Available {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestDocsCache_UnavailableOlderThan24hReExtracted(t *testing.T) {
	c := newTTLCache()
	c.put(DocsCacheKey("p", "1"), mustJSON(t, DocsResponse{Available: false}), 25*time.Hour)
	calls := 0
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		calls++
		return &model.Package{}, nil
	})
	if _, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("extract called %d times, want 1", calls)
	}
}

func TestDocsCache_AvailableOlderThan24hServed(t *testing.T) {
	c := newTTLCache()
	c.put(DocsCacheKey("p", "1"), mustJSON(t, DocsResponse{Available: true}), 48*time.Hour)
	h := docsCacheHandler(c, func(context.Context, string, string) (*model.Package, error) {
		t.Fatal("extract must not run")
		return nil, nil
	})
	resp, _, err := h.fetchDocsAtVersion(context.Background(), "p", "1")
	if err != nil || !resp.Available {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestDocsCache_ErrorKeyWriteByCause(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name      string
		ctx       context.Context
		extract   func(context.Context, string, string) (*model.Package, error)
		wantWrite bool
	}{
		{"client cancel error", context.Background(),
			func(context.Context, string, string) (*model.Package, error) {
				return nil, fmt.Errorf("download: %w", context.Canceled)
			}, false},
		{"client cancel with partial package", cancelled,
			func(context.Context, string, string) (*model.Package, error) {
				return &model.Package{}, nil
			}, false},
		{"deadline exceeded", context.Background(),
			func(context.Context, string, string) (*model.Package, error) {
				return nil, fmt.Errorf("download: %w", context.DeadlineExceeded)
			}, true},
		{"generic error", context.Background(),
			func(context.Context, string, string) (*model.Package, error) {
				return nil, errors.New("bad wheel")
			}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTTLCache()
			h := docsCacheHandler(c, tc.extract)
			_, _, err := h.fetchDocsAtVersion(tc.ctx, "p", "1")
			if !errors.Is(err, ErrDocsExtractionFailed) {
				t.Fatalf("err = %v, want ErrDocsExtractionFailed", err)
			}
			if got := c.wrote(DocsErrKey("p", "1")); got != tc.wantWrite {
				t.Errorf("error key written = %v, want %v", got, tc.wantWrite)
			}
		})
	}
}

func TestCleanDocstringText_IndentedSourceDocstring(t *testing.T) {
	raw := "Fetch the queryset.\n\n    Extended description here.\n\n    Args:\n        pk: Primary key\n\n    Returns:\n        QuerySet: The filtered queryset\n    "
	got := cleanDocstringText(docstring.Parse(raw))
	want := "Fetch the queryset.\n\nExtended description here."
	if got != want {
		t.Errorf("cleanDocstringText(indented) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_SphinxRoleLineKept(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text:  "Summary.\n\n:class:`Foo` does things.\n\n:param x: The input",
	}
	got := cleanDocstringText(ds)
	want := "Summary.\n\n:class:`Foo` does things."
	if got != want {
		t.Errorf("cleanDocstringText = %q, want %q", got, want)
	}
}

func TestCleanDocstringText_KeepsExamplesAndNotesGoogle(t *testing.T) {
	raw := "Fetch it.\n\n    Args:\n        pk: Primary key\n\n    Returns:\n        int: The value\n\n    Examples:\n        >>> fetch(1)\n        1\n\n    Note:\n        Be careful.\n    "
	got := cleanDocstringText(docstring.Parse(raw))
	want := "Fetch it.\n\nExamples:\n    >>> fetch(1)\n    1\n\nNote:\n    Be careful."
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestCleanDocstringText_KeepsExamplesAndNotesNumpy(t *testing.T) {
	raw := "Compute.\n\n    Parameters\n    ----------\n    x : int\n        Input.\n\n    Returns\n    -------\n    int\n        Output.\n\n    Examples\n    --------\n    >>> compute(1)\n    1\n\n    Notes\n    -----\n    Be careful.\n    "
	got := cleanDocstringText(docstring.Parse(raw))
	want := "Compute.\n\nExamples\n--------\n>>> compute(1)\n1\n\nNotes\n-----\nBe careful."
	if got != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestCleanDocstringText_GoogleTrailingProse(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringGoogle,
		Text:  "Summary.\n\nArgs:\n    x: Input.\n\nTrailing prose stays.",
	}
	got := cleanDocstringText(ds)
	want := "Summary.\n\nTrailing prose stays."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCleanDocstringText_SphinxFieldsBetweenProse(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text:  "First paragraph.\n\n:param x: The input,\n    continued here.\n:type x: int\n\nSecond paragraph.",
	}
	got := cleanDocstringText(ds)
	want := "First paragraph.\n\nSecond paragraph."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCleanDocstringText_NumpyProseReturnsWithoutDashes(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringNumpy,
		Text:  "Summary.\n\nReturns\n\nthis line is prose.\n\nParameters\n----------\nx : int\n    Input.",
	}
	got := cleanDocstringText(ds)
	want := "Summary.\n\nReturns\n\nthis line is prose."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertFunction_YieldsFillsReturnDescription(t *testing.T) {
	fn := &model.Function{
		Name:      "gen",
		Docstring: &model.Docstring{Yields: &model.DocReturn{Type: "int", Description: "The next value."}},
	}
	sym := convertFunction(fn, nil)
	if sym.Returns == nil || sym.Returns.Description != "The next value." {
		t.Errorf("Returns = %+v", sym.Returns)
	}
}
