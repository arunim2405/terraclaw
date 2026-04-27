// Package terrashark embeds the TerraShark failure-mode references and
// exposes them as prompt-injectable guidance blocks for terraclaw's two-stage
// Terraform generation pipeline.
//
// The TerraShark skill is a failure-mode-first prompt that teaches an LLM
// how to think about Terraform problems (identity churn, secret exposure,
// blast radius, CI drift, compliance gaps) before it generates any HCL.
// For terraclaw's import-focused workflow we pull in the subset of
// references that directly improve blueprint design (Stage 1) and HCL
// emission (Stage 2).
//
// Source of truth: https://github.com/LukasNiessen/terrashark (MIT).
// The embedded Markdown files live under refs/ and are copied verbatim
// from that upstream repo (see .agents/skills/terrashark/ for the full
// skill bundle including references not bundled into the binary).
package terrashark

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

// references embeds every *.md file under refs/ into the binary.
//
//go:embed refs/*.md
var references embed.FS

const refsDir = "refs"

// referenceNames maps a short logical name to the embedded filename. Kept
// private so callers don't depend on filenames; they compose via the
// guidance helpers below.
var referenceNames = map[string]string{
	"workflow":      "SKILL.md",
	"coding":        "coding-standards.md",
	"dodonts":       "do-dont-patterns.md",
	"identity":      "identity-churn.md",
	"secrets":       "secret-exposure.md",
	"examples-good": "examples-good.md",
	"examples-bad":  "examples-bad.md",
	"modules":       "module-architecture.md",
	"migrations":    "migration-playbooks.md",
}

// cache memoizes loaded reference bodies since they never change at runtime.
var (
	cacheMu   sync.RWMutex
	refsCache = make(map[string]string)
)

// Reference returns the raw Markdown body for the given logical name
// (e.g. "workflow", "coding", "identity"). Returns an error if the logical
// name is unknown or the embedded file cannot be read.
func Reference(name string) (string, error) {
	filename, ok := referenceNames[name]
	if !ok {
		return "", fmt.Errorf("terrashark: unknown reference %q", name)
	}

	cacheMu.RLock()
	if cached, hit := refsCache[filename]; hit {
		cacheMu.RUnlock()
		return cached, nil
	}
	cacheMu.RUnlock()

	data, err := references.ReadFile(refsDir + "/" + filename)
	if err != nil {
		return "", fmt.Errorf("terrashark: read embedded %s: %w", filename, err)
	}
	body := string(data)

	cacheMu.Lock()
	refsCache[filename] = body
	cacheMu.Unlock()

	return body, nil
}

// Available reports the logical reference names embedded in the binary.
// Useful for diagnostics and tests.
func Available() []string {
	out := make([]string, 0, len(referenceNames))
	for name := range referenceNames {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Verify asserts every declared reference is actually embedded. Call from
// tests or doctor-style checks at startup.
func Verify() error {
	entries, err := fs.ReadDir(references, refsDir)
	if err != nil {
		return fmt.Errorf("terrashark: read embed dir: %w", err)
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		present[e.Name()] = true
	}
	for logical, filename := range referenceNames {
		if !present[filename] {
			return fmt.Errorf("terrashark: missing embedded reference %q (logical %q)", filename, logical)
		}
	}
	return nil
}

// designReferenceOrder is the sequence used when assembling the Stage 1
// (blueprint design) guidance block. Ordering matters: workflow first
// establishes the diagnostic frame, then module boundaries, then the rule
// banks, then concrete examples last so they anchor the preceding rules.
var designReferenceOrder = []string{
	"workflow",
	"modules",
	"coding",
	"dodonts",
	"examples-bad",
	"examples-good",
}

// codingReferenceOrder is the sequence used for Stage 2 (HCL emission).
// Rules banks first, then failure-mode deep-dives that directly shape the
// emitted code, then a concrete bad-example bank for last-mile grounding.
var codingReferenceOrder = []string{
	"coding",
	"dodonts",
	"identity",
	"secrets",
	"migrations",
	"examples-bad",
}

// DesignGuidance returns a prompt-ready block for Stage 1 (blueprint
// design). The block is wrapped in <terrashark_guardrails> so it composes
// cleanly with terraclaw's existing XML-style prompt sections.
//
// The returned string is safe to concatenate into BuildStage1SystemPrompt
// output. On embed error (should never happen in a successful build) it
// returns a comment block indicating the failure — the caller can still
// proceed with the unenhanced prompt.
func DesignGuidance() string {
	return renderGuidance("Stage 1 — blueprint design", designPreamble(), designReferenceOrder)
}

// CodingGuidance returns a prompt-ready block for Stage 2 (HCL emission).
// Same shape and safety semantics as DesignGuidance.
func CodingGuidance() string {
	return renderGuidance("Stage 2 — HCL emission", codingPreamble(), codingReferenceOrder)
}

func designPreamble() string {
	return strings.TrimSpace(`
These are the TerraShark failure-mode guardrails for the blueprint design
stage. Treat them as HARD CONSTRAINTS for your blueprint output. They
reduce the highest-impact hallucinations the model tends to make when
mapping live resources to modules:

- prefer pinned registry / trusted modules over hand-rolled resources
- never invent attribute names — if unsure, flag as unverified or leave
  the resource for a local module with the CLI-fetched config
- every multi-instance group must use for_each with stable BUSINESS keys,
  not list indexes (no count for collections)
- the blueprint IS the design contract; once produced, Stage 2 cannot add
  resources, rename addresses, or change iteration strategy without a
  moved-block plan
- secrets must be replaced with REDACTED + a sensitive variable reference
- every blueprint decision must be traceable to one of the failure modes
  below (identity churn, secret exposure, blast radius)

Load and apply the following reference bodies in order.
`)
}

func codingPreamble() string {
	return strings.TrimSpace(`
These are the TerraShark failure-mode guardrails for the HCL emission
stage. Treat them as HARD CONSTRAINTS for every .tf file you write:

- emit required_version and required_providers in versions.tf with
  bounded version constraints — never float providers
- use for_each with business keys for any collection; count is ONLY for
  singleton 0/1 toggles
- add moved blocks for every rename, restructure, or iteration-model
  change — before any terraform apply or import retry
- never write plaintext secret defaults, committed tfvars secrets, or
  provisioner credential echoes; mark secret-bearing outputs sensitive
- when a registry module is in the blueprint, adjust MODULE INPUTS — do
  not reach into the module's internal resources
- attribute names you are unsure about MUST come from a data source, a
  CLI-fetched config, or be flagged as a TODO; do NOT fabricate defaults

Load and apply the following reference bodies in order.
`)
}

// renderGuidance composes the final prompt block. Missing references are
// surfaced as inline comments rather than silently dropped so tests catch
// embed drift.
func renderGuidance(label, preamble string, order []string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<terrashark_guardrails stage=%q>\n", label)
	buf.WriteString(preamble)
	buf.WriteString("\n\n")

	for _, name := range order {
		body, err := Reference(name)
		if err != nil {
			fmt.Fprintf(&buf, "<!-- terrashark: reference %q missing: %v -->\n\n", name, err)
			continue
		}
		filename := referenceNames[name]
		fmt.Fprintf(&buf, "<reference name=%q source=%q>\n", name, filename)
		buf.WriteString(strings.TrimSpace(body))
		buf.WriteString("\n</reference>\n\n")
	}

	buf.WriteString("</terrashark_guardrails>\n")
	return buf.String()
}
