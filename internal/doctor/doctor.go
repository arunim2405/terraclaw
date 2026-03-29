package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// Client is the subset of the steampipe client needed by doctor checks.
type Client interface {
	ListSchemas() ([]string, error)
	Close() error
}

// CheckResult contains the outcome of a single doctor check.
type CheckResult struct {
	Name    string
	OK      bool
	Details string
	Fix     string
}

// Report is a full doctor run.
type Report struct {
	Checks []CheckResult
}

// HasFailures reports whether any check failed.
func (r Report) HasFailures() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return true
		}
	}
	return false
}

// Deps holds replaceable dependencies for testability.
type Deps struct {
	LookPath         func(file string) (string, error)
	ConnectSteampipe func(connStr string) (Client, error)
	MkdirAll         func(path string, perm os.FileMode) error
	CreateTemp       func(dir, pattern string) (*os.File, error)
	Remove           func(name string) error
}

// DefaultDeps returns production dependencies.
func DefaultDeps() Deps {
	return Deps{
		LookPath: exec.LookPath,
		ConnectSteampipe: func(connStr string) (Client, error) {
			return steampipe.New(connStr)
		},
		MkdirAll:   os.MkdirAll,
		CreateTemp: os.CreateTemp,
		Remove:     os.Remove,
	}
}

// Run executes dependency checks for the current configuration.
func Run(cfg *config.Config, deps Deps) Report {
	report := Report{}

	// Check that opencode is installed.
	report.Checks = append(report.Checks, checkOpencode(deps))

	// Check steampipe binary.
	steampipeBin := checkBinary(deps, "steampipe")
	report.Checks = append(report.Checks, steampipeBin)

	// Check terraform binary.
	terraformBin := cfg.TerraformBin
	if terraformBin == "" {
		terraformBin = "terraform"
	}
	terraform := checkBinary(deps, terraformBin)
	terraform.Name = fmt.Sprintf("terraform binary (%s)", terraformBin)
	report.Checks = append(report.Checks, terraform)

	// Check output directory.
	report.Checks = append(report.Checks, checkOutputDir(cfg, deps))

	// Check steampipe connection.
	report.Checks = append(report.Checks, checkSteampipeConnection(cfg, deps))

	return report
}

func checkOpencode(deps Deps) CheckResult {
	path, err := deps.LookPath("opencode")
	if err != nil {
		return CheckResult{
			Name:    "opencode (coding agent)",
			OK:      false,
			Details: err.Error(),
			Fix:     "Install OpenCode: brew install anomalyco/tap/opencode (or npm i -g opencode-ai)",
		}
	}
	return CheckResult{
		Name:    "opencode (coding agent)",
		OK:      true,
		Details: path,
	}
}

func checkBinary(deps Deps, bin string) CheckResult {
	path, err := deps.LookPath(bin)
	if err != nil {
		return CheckResult{
			Name:    fmt.Sprintf("binary: %s", bin),
			OK:      false,
			Details: err.Error(),
			Fix:     fmt.Sprintf("Install %s and ensure it is available on PATH.", bin),
		}
	}
	return CheckResult{
		Name:    fmt.Sprintf("binary: %s", bin),
		OK:      true,
		Details: path,
	}
}

func checkOutputDir(cfg *config.Config, deps Deps) CheckResult {
	if err := deps.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return CheckResult{
			Name:    "output directory",
			OK:      false,
			Details: err.Error(),
			Fix:     fmt.Sprintf("Use a writable --output-dir (current: %s).", cfg.OutputDir),
		}
	}

	tmp, err := deps.CreateTemp(cfg.OutputDir, ".terraclaw-doctor-*")
	if err != nil {
		return CheckResult{
			Name:    "output directory",
			OK:      false,
			Details: err.Error(),
			Fix:     fmt.Sprintf("Ensure write permissions for %s.", cfg.OutputDir),
		}
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = deps.Remove(name)

	return CheckResult{
		Name:    "output directory",
		OK:      true,
		Details: filepath.Clean(cfg.OutputDir),
	}
}

func checkSteampipeConnection(cfg *config.Config, deps Deps) CheckResult {
	client, err := deps.ConnectSteampipe(cfg.SteampipeConnStr())
	if err != nil {
		return CheckResult{
			Name:    "steampipe connection",
			OK:      false,
			Details: err.Error(),
			Fix:     "Start Steampipe service: steampipe service start",
		}
	}
	defer client.Close()

	schemas, err := client.ListSchemas()
	if err != nil {
		return CheckResult{
			Name:    "steampipe schemas",
			OK:      false,
			Details: err.Error(),
			Fix:     "Ensure your Steampipe user can query information_schema.",
		}
	}
	if len(schemas) == 0 {
		return CheckResult{
			Name:    "steampipe schemas",
			OK:      false,
			Details: "no plugin schemas found",
			Fix:     "Install at least one plugin, for example: steampipe plugin install aws",
		}
	}
	return CheckResult{
		Name:    "steampipe schemas",
		OK:      true,
		Details: fmt.Sprintf("%d schema(s) available", len(schemas)),
	}
}
