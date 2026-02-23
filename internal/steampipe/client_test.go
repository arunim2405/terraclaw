package steampipe_test

import (
	"testing"

	"github.com/arunim2405/terraclaw/internal/steampipe"
)

func TestResourceString(t *testing.T) {
	r := steampipe.Resource{
		Provider: "aws",
		Type:     "aws_s3_bucket",
		Name:     "my-bucket",
		ID:       "my-bucket",
		Region:   "us-east-1",
	}
	got := r.String()
	if got == "" {
		t.Error("Resource.String() returned empty string")
	}
	// Should contain provider, type, name and region.
	for _, substr := range []string{"aws", "aws_s3_bucket", "my-bucket", "us-east-1"} {
		if !containsStr(got, substr) {
			t.Errorf("Resource.String() = %q, missing %q", got, substr)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findStr(s, sub))
}

func findStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
