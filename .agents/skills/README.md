# Terraform Skills

Agent skills for Terraform infrastructure-as-code development.

## Plugins

### terraform-code-generation

Skills for generating and validating Terraform HCL code.

| Skill | Description |
|-------|-------------|
| terraform-style-guide  | Generate Terraform HCL code following HashiCorp style conventions |
| terraform-test         | Writing and running `.tftest.hcl` test files |
| azure-verified-modules | Azure Verified Modules (AVM) requirements and certification |
| terraform-search-import | Discover existing resources with Terraform Search and bulk import |

### terraform-module-generation

Skills for creating and refactoring Terraform modules.

| Skill | Description |
|-------|-------------|
| refactor-module  | Transform monolithic configs into reusable modules |
| terraform-stacks | Multi-region/environment orchestration with Terraform Stacks |

### terraform-provider-development

Skills for developing Terraform providers.

| Skill | Description |
|-------|-------------|
| new-terraform-provider | Scaffold a new Terraform provider |
| run-acceptance-tests   | Run and debug provider acceptance tests |
| provider-actions       | Implement provider actions (lifecycle operations) |
| provider-resources     | Implement resources and data sources |

### terrashark

Failure-mode-first guardrails that prevent the highest-impact Terraform / OpenTofu hallucinations (identity churn, secret exposure, blast-radius mistakes, CI drift, compliance gaps). Upstream: [LukasNiessen/terrashark](https://github.com/LukasNiessen/terrashark) — MIT licensed, bundled verbatim at `.agents/skills/terrashark/`.

| Reference | Why it matters for terraclaw |
|-----------|------------------------------|
| `SKILL.md`              | 7-step workflow: diagnose before generating |
| `coding-standards.md`   | Iteration/identity, version discipline, feature-guard table |
| `do-dont-patterns.md`   | Fast checklist for safe HCL output |
| `identity-churn.md`     | `count` vs `for_each`, `moved` blocks, stable keys |
| `secret-exposure.md`    | Preventing secret leakage through state/logs/artifacts |
| `module-architecture.md`| Module role model and composition rules |
| `migration-playbooks.md`| `moved`/`import`/refactor playbooks |
| `examples-good.md` / `examples-bad.md` | Pattern banks |

**How terraclaw uses it:** The references are also embedded into the binary via `internal/terrashark` and injected into both Stage 1 (blueprint design) and Stage 2 (HCL emission) prompts as a `<terrashark_guardrails>` block. This is independent of OpenCode-side skill loading — both paths are active, giving belt + suspenders coverage.

## Installation

### Claude Code Plugin

```bash
claude plugin marketplace add hashicorp/agent-skills

claude plugin install terraform-code-generation@hashicorp
claude plugin install terraform-module-generation@hashicorp
claude plugin install terraform-provider-development@hashicorp
```

### Individual Skills

```bash
# Code generation
npx skills add hashicorp/agent-skills/terraform/code-generation/skills/terraform-style-guide
npx skills add hashicorp/agent-skills/terraform/code-generation/skills/terraform-test
npx skills add hashicorp/agent-skills/terraform/code-generation/skills/azure-verified-modules
npx skills add hashicorp/agent-skills/terraform/code-generation/skills/terraform-search-import

# Module generation
npx skills add hashicorp/agent-skills/terraform/module-generation/skills/refactor-module
npx skills add hashicorp/agent-skills/terraform/module-generation/skills/terraform-stacks

# Provider development
npx skills add hashicorp/agent-skills/terraform/provider-development/skills/new-terraform-provider
npx skills add hashicorp/agent-skills/terraform/provider-development/skills/run-acceptance-tests
npx skills add hashicorp/agent-skills/terraform/provider-development/skills/provider-actions
npx skills add hashicorp/agent-skills/terraform/provider-development/skills/provider-resources
```

## MCP Server

Relevant Terraform plugins include the `terraform-mcp-server` which provides access to Terraform Cloud/Enterprise APIs. Set the following environment variables:

```bash
export TFE_TOKEN="your-terraform-cloud-token"
export TFE_ADDRESS="https://app.terraform.io"  # or your TFE instance
```

## Structure

```
terraform/
├── code-generation/
│   ├── .claude-plugin/plugin.json
│   └── skills/
│       ├── terraform-style-guide/
│       ├── terraform-test/
│       ├── azure-verified-modules/
│       └── terraform-search-import/
├── module-generation/
│   ├── .claude-plugin/plugin.json
│   └── skills/
│       ├── terraform-stacks/
│       └── refactor-module/
└── provider-development/
    ├── .claude-plugin/plugin.json
    └── skills/
        ├── new-terraform-provider/
        ├── provider-actions/
        ├── provider-resources/
        └── run-acceptance-tests/
```

## References

- [Terraform Documentation](https://developer.hashicorp.com/terraform)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform MCP Server](https://github.com/hashicorp/terraform-mcp-server)
