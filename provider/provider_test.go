package provider

import (
	"errors"
	"testing"
	"time"

	"github.com/mrz1836/postmark"
)

// TestProviderBuilds ensures the provider's schema can be inferred from all
// resource definitions. Provider() panics on a schema-generation error, so this
// catches malformed pulumi struct tags, duplicate tokens, and similar regressions.
func TestProviderBuilds(t *testing.T) {
	_ = Provider()
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"signature code", postmark.APIError{ErrorCode: errCodeSignatureNotFound, Message: "x"}, true},
		{"domain code", postmark.APIError{ErrorCode: errCodeDomainNotFound, Message: "x"}, true},
		{"template code", postmark.APIError{ErrorCode: errCodeTemplateNotFound, Message: "x"}, true},
		{"bad token is not notfound", postmark.APIError{ErrorCode: errCodeBadAPIToken, Message: "Bad token"}, false},
		{"message fallback", postmark.APIError{ErrorCode: 600, Message: "Server does not exist"}, true},
		{"wrapped notfound", errors.New("looking up: " + postmark.APIError{ErrorCode: 510, Message: "gone"}.Error()), false},
		{"http 404 string", errors.New("request failed with status 404: nope"), true},
		{"other error", errors.New("boom"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotFound(tc.err); got != tc.want {
				t.Fatalf("isNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestParseTemplateResourceID(t *testing.T) {
	tests := []struct {
		id         string
		wantServer int
		wantTmpl   string
		wantErr    bool
	}{
		{"12345/678", 12345, "678", false},
		{"12345/my-alias", 12345, "my-alias", false},
		{"678", 0, "", true},
		{"/678", 0, "", true},
		{"abc/678", 0, "", true},
		{"", 0, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			s, tmpl, err := parseTemplateResourceID(tc.id)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && (s != tc.wantServer || tmpl != tc.wantTmpl) {
				t.Fatalf("got (%d, %q), want (%d, %q)", s, tmpl, tc.wantServer, tc.wantTmpl)
			}
		})
	}
}

func TestTemplateResourceID(t *testing.T) {
	got := templateResourceID(postmark.Template{TemplateID: 678, AssociatedServerID: 12345})
	if got != "12345/678" {
		t.Fatalf("templateResourceID = %q, want 12345/678", got)
	}
}

func TestRetryAfter(t *testing.T) {
	if retryAfter("") != 0 {
		t.Fatal("empty should be 0")
	}
	if retryAfter("3") != 3*time.Second {
		t.Fatal("numeric seconds should parse")
	}
	if retryAfter("not-a-date") != 0 {
		t.Fatal("unparseable should be 0")
	}
}

func TestDeref(t *testing.T) {
	if deref[string](nil, "fallback") != "fallback" {
		t.Fatal("nil should yield fallback")
	}
	v := "set"
	if deref(&v, "fallback") != "set" {
		t.Fatal("non-nil should yield value")
	}
	if *ptr(42) != 42 {
		t.Fatal("ptr round-trip")
	}
}
