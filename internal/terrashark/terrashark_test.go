package terrashark_test

import (
	"strings"
	"testing"

	"github.com/arunim2405/terraclaw/internal/terrashark"
)

func TestVerifyAllReferencesEmbedded(t *testing.T) {
	if err := terrashark.Verify(); err != nil {
		t.Fatalf("Verify() = %v, want nil — embed drift means refs/ is missing a file declared in referenceNames", err)
	}
}

func TestAvailableReturnsKnownLogicalNames(t *testing.T) {
	got := terrashark.Available()
	want := []string{
		"coding",
		"dodonts",
		"examples-bad",
		"examples-good",
		"identity",
		"migrations",
		"modules",
		"secrets",
		"workflow",
	}
	if len(got) != len(want) {
		t.Fatalf("Available() length = %d, want %d\n got=%v\nwant=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Available()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReferenceReturnsNonEmptyBody(t *testing.T) {
	for _, name := range terrashark.Available() {
		body, err := terrashark.Reference(name)
		if err != nil {
			t.Errorf("Reference(%q) returned error: %v", name, err)
			continue
		}
		if strings.TrimSpace(body) == "" {
			t.Errorf("Reference(%q) returned empty body", name)
		}
	}
}

func TestReferenceUnknownName(t *testing.T) {
	_, err := terrashark.Reference("does-not-exist")
	if err == nil {
		t.Fatal("Reference(\"does-not-exist\") = nil, want error")
	}
}

func TestDesignGuidanceContainsCoreReferenceMarkers(t *testing.T) {
	block := terrashark.DesignGuidance()

	mustContain := []string{
		`<terrashark_guardrails stage="Stage 1 — blueprint design">`,
		`</terrashark_guardrails>`,
		`source="SKILL.md"`,
		`source="module-architecture.md"`,
		`source="coding-standards.md"`,
		`source="do-dont-patterns.md"`,
		`source="examples-bad.md"`,
		`source="examples-good.md"`,
		// Preamble anchors
		"HARD CONSTRAINTS",
		"for_each with stable BUSINESS keys",
	}
	for _, s := range mustContain {
		if !strings.Contains(block, s) {
			t.Errorf("DesignGuidance missing expected substring %q", s)
		}
	}
}

func TestCodingGuidanceContainsCoreReferenceMarkers(t *testing.T) {
	block := terrashark.CodingGuidance()

	mustContain := []string{
		`<terrashark_guardrails stage="Stage 2 — HCL emission">`,
		`</terrashark_guardrails>`,
		`source="coding-standards.md"`,
		`source="do-dont-patterns.md"`,
		`source="identity-churn.md"`,
		`source="secret-exposure.md"`,
		`source="migration-playbooks.md"`,
		`source="examples-bad.md"`,
		"required_version and required_providers",
		"moved blocks",
	}
	for _, s := range mustContain {
		if !strings.Contains(block, s) {
			t.Errorf("CodingGuidance missing expected substring %q", s)
		}
	}
}

// Each helper must be deterministic — same inputs, same output — so prompts
// are reproducible across runs (critical for golden-file reviews and caches).
func TestGuidanceIsDeterministic(t *testing.T) {
	a := terrashark.DesignGuidance()
	b := terrashark.DesignGuidance()
	if a != b {
		t.Error("DesignGuidance() is not deterministic across calls")
	}
	c := terrashark.CodingGuidance()
	d := terrashark.CodingGuidance()
	if c != d {
		t.Error("CodingGuidance() is not deterministic across calls")
	}
}
