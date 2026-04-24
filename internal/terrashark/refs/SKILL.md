---
name: terrashark
description: "Prevent Terraform/OpenTofu hallucinations by diagnosing and fixing failure modes: identity churn, secret exposure, blast-radius mistakes, CI drift, and compliance gate gaps. Use when generating, reviewing, refactoring, or migrating IaC and when building delivery/testing pipelines."
license: MIT
compatibility: claude-code, codex, opencode
---

# Terrashark: Failure-Mode Workflow for Terraform/OpenTofu

Run this workflow top to bottom. Diagnose before you generate. Do not skip steps 2, 5a, and 5c — they are the primary hallucination guards.

## 1) Capture execution context

Record before writing code:
- runtime (`terraform` or `tofu`) and exact version (the "runtime floor")
- provider(s), target platform, and state backend
- execution path (local CLI, CI, HCP Terraform/TFE, Atlantis)
- environment criticality (dev/shared/prod)
- whether new code joins an existing stack (inherit conventions) or starts fresh

If any field is unknown, state the assumption explicitly and proceed — never silently guess.

## 2) Diagnose failure mode(s)

Select at least one based on intent and risk:
- identity churn: addressing instability, refactor breakage
- secret exposure: secrets in state, logs, defaults, artifacts
- blast radius: oversized stacks, weak boundaries, unsafe applies
- CI drift: version mismatch, unreviewed applies, missing artifacts
- compliance gate gaps: missing policies, approvals, audit controls

If none apply at first glance, pick the closest: every IaC change carries blast-radius and identity-churn risk by default.

## 3) Load only the relevant reference file(s)

Primary references (load per diagnosed mode):
- `references/identity-churn.md`
- `references/secret-exposure.md`
- `references/blast-radius.md`
- `references/ci-drift.md`
- `references/compliance-gates.md`

Supplemental (load only when directly relevant):
- `references/testing-matrix.md`
- `references/quick-ops.md`
- `references/examples-good.md`
- `references/examples-bad.md`
- `references/examples-neutral.md`
- `references/coding-standards.md`
- `references/module-architecture.md`
- `references/ci-delivery-patterns.md`
- `references/security-and-governance.md`
- `references/do-dont-patterns.md`
- `references/mcp-integration.md`
- `references/migration-playbooks.md` (load for any address, rename, import, or upgrade change)
- `references/trusted-modules.md` (load when provider is `aws`, `azurerm`, `google`, `oci`, or `ibm`)

## 4) Propose fix path with explicit risk controls

For each fix, include:
- why this addresses the diagnosed failure mode
- what could still go wrong
- guardrails (tests, approvals, rollback)

If the user is already committed to an approach, still surface the risk — do not suppress it to seem agreeable.

## 5) Generate implementation artifacts

### 5a) Pre-generation constraint snapshot — required

Before emitting any HCL, commit in one short block:
- runtime floor (e.g., `terraform >= 1.7`) and the features it unlocks or blocks (see `references/coding-standards.md` feature guard table)
- iteration model per multi-instance resource: `count` (singleton toggle) vs `for_each` (stable business keys)
- identity keys you will use (the actual key names, not placeholders)
- version pins (providers, modules) and lockfile stance
- secret paths: where secret values come from at runtime (never from `default`, `.tfvars`, or literals)
- state boundary: which state file/backend this code lives in, and why

If any of these cannot be committed, stop and ask. Do not generate code against unknown constraints.

### 5b) Generation preference order

For each resource, choose the highest option available:
1. A pinned trusted-registry module covering this resource (see `references/trusted-modules.md`) — use it unless the user explicitly requested raw HCL.
2. A module already present in the target repo — reuse before defining new.
3. Hand-rolled `resource` block — only when 1 and 2 are not viable.

When emitting hand-rolled HCL, do not invent attribute names. If an attribute is not certain from reference material or the user's repo, either:
- use a data source / variable with a documented shape, or
- surface it as an explicit TODO with a link to the authoritative provider doc — never fabricate defaults.

### 5c) Generation rules — every emitted artifact must

- include a `terraform { required_version, required_providers }` block in every new root, with bounded version constraints
- use `for_each` with business-meaningful keys for any collection that can change membership; use `count` only for true `0 | 1` toggles
- include a `moved` block for any rename, restructure, or iteration-model change — before any apply
- prefer declarative `import` blocks (TF 1.5+) over CLI `terraform import` in automation paths
- mark outputs that carry secrets as `sensitive = true`; do not rely on `sensitive` alone for state safety — prefer `write_only` (1.11+) where the runtime allows
- never emit plaintext secret literals, committed `.tfvars` secrets, or plaintext `default` values for credential variables
- in CI: separate plan and apply, persist and consume one reviewed plan artifact, run policy/cost checks on every path to apply
- for stacks that span multiple files, emit a skeleton first (module/resource list, var list, output list) and confirm shape before filling bodies, unless the task is a single small change

### 5d) Evidence discipline

For any non-trivial attribute, function, or version claim, cite the source once inline (provider docs URL, registry module page, or in-repo file path). If citing is not possible, flag the claim as "unverified — confirm before apply".

## 5.5) Self-critique pass — required before presenting output

Run the emitted code against `references/do-dont-patterns.md` and answer:
1. Identity: are any identities index-based, derived from computed values, or unstable across reorderings?
2. Secrets: any plaintext defaults, committed tfvars, provisioner echoes, or sensitive outputs missing `sensitive = true`?
3. Blast radius: any cross-environment coupling, unbounded IAM, or production state mixed with lower environments?
4. Versions: runtime and providers bounded; lockfile stance stated?

If any answer is "yes / missing", fix it before presenting. Do not ship code that fails this pass with a note promising to fix later.

## 6) Validate before finalize

Provide a command ladder tailored to runtime and risk tier. Each step must pass before the next runs:

```
fmt -check  →  validate  →  init  →  plan -out=plan.bin
           →  show -json plan.bin | policy/cost scan
           →  human review of plan
           →  apply plan.bin   (gated by environment + approval)
```

Never recommend direct production apply without a reviewed plan artifact and explicit approval. If the user requests it, push back and offer the safe path.

## 7) Stop conditions — refuse or push back when

- user asks for apply against production without a reviewed plan artifact
- user asks to commit secrets to VCS (any form) or to disable `sensitive`
- user asks to widen IAM to `*:*` or open security groups to `0.0.0.0/0` on a non-dev resource without explicit justification
- runtime floor cannot be determined for a change that depends on it (e.g., `moved`, `import`, `write_only`)
- state backend for production is local or unencrypted

Surface the risk, propose a safer alternative, then wait for the user to decide.

## 8) Output contract — use these exact headers

Return your final response with these `##` sections, in order, so outputs stay auditable and diff-able:

- `## Assumptions` — runtime floor, provider versions, backend, criticality, and any filled-in unknowns from step 1.
- `## Failure modes addressed` — the modes selected in step 2 and why.
- `## Changes` — the HCL / CI / policy artifacts, grouped by file.
- `## Tradeoffs` — what was not done and why; alternatives considered.
- `## Validation plan` — the exact command ladder from step 6, runtime-correct.
- `## Rollback` — how to undo this change, including any `moved`/`removed` reversal or state surgery; required for any destructive-impact change, otherwise write "no destructive impact".
