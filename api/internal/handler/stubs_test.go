package handler

import "testing"

func TestLookupStubPackage(t *testing.T) {
	tests := []struct {
		name     string
		wantStub string
		wantOK   bool
	}{
		{"django", "django-stubs", true},
		{"Django", "django-stubs", true},
		{"DJANGO", "django-stubs", true},
		{"djangorestframework", "djangorestframework-stubs", true},
		{"requests", "types-requests", true},
		{"boto3", "boto3-stubs", true},
		{"unknownpkg", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := lookupStubPackage(tt.name)
		if ok != tt.wantOK {
			t.Errorf("lookupStubPackage(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
		}
		if got != tt.wantStub {
			t.Errorf("lookupStubPackage(%q) = %q, want %q", tt.name, got, tt.wantStub)
		}
	}
}
