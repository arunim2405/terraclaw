package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tfconfig "github.com/hashicorp/terraform-config-inspect/tfconfig"
)

// ScanModule fetches a module from the given source, parses its HCL files,
// and returns populated metadata. The source can be:
//   - A git URL: "git::https://github.com/org/repo.git//modules/vpc?ref=v1.0"
//   - A local path: "./modules/vpc" or "/absolute/path/to/module"
func ScanModule(source string) (*ModuleMetadata, error) {
	dir, cleanup, err := resolveSource(source)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	meta, err := parseModule(dir)
	if err != nil {
		return nil, fmt.Errorf("parse module: %w", err)
	}

	meta.Source = source
	meta.Name = deriveName(source, dir)
	meta.Description = readDescription(dir)
	meta.ProviderType = detectProvider(meta.ResourceTypes)

	return meta, nil
}

// resolveSource returns the local directory to parse and an optional cleanup func.
func resolveSource(source string) (dir string, cleanup func(), err error) {
	if isGitSource(source) {
		repoURL, subdir, ref := parseGitSource(source)
		tmpDir, cloneErr := cloneToTemp(repoURL, ref)
		if cloneErr != nil {
			return "", nil, cloneErr
		}
		cleanupFn := func() { os.RemoveAll(tmpDir) }

		moduleDir := tmpDir
		if subdir != "" {
			moduleDir = filepath.Join(tmpDir, subdir)
		}

		if _, statErr := os.Stat(moduleDir); statErr != nil {
			cleanupFn()
			return "", nil, fmt.Errorf("subdirectory %q not found in cloned repo", subdir)
		}

		return moduleDir, cleanupFn, nil
	}

	// Local path.
	absDir, err := filepath.Abs(source)
	if err != nil {
		return "", nil, fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(absDir); err != nil {
		return "", nil, fmt.Errorf("module directory not found: %w", err)
	}

	return absDir, nil, nil
}

// isGitSource returns true if the source looks like a git URL.
func isGitSource(source string) bool {
	return strings.HasPrefix(source, "git::") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "git@")
}

// parseGitSource splits a Terraform-style git source into repo URL, subdirectory,
// and ref. Example: "git::https://github.com/org/repo.git//modules/vpc?ref=v1.0"
// returns ("https://github.com/org/repo.git", "modules/vpc", "v1.0").
func parseGitSource(source string) (repoURL, subdir, ref string) {
	// Strip "git::" prefix.
	s := strings.TrimPrefix(source, "git::")

	// Extract ?ref=xxx.
	if idx := strings.Index(s, "?ref="); idx != -1 {
		ref = s[idx+5:]
		s = s[:idx]
	}

	// Split on "//" for subdirectory.
	if idx := strings.Index(s, "//"); idx != -1 {
		repoURL = s[:idx]
		subdir = s[idx+2:]
	} else {
		repoURL = s
	}

	return repoURL, subdir, ref
}

// cloneToTemp clones a git repository to a temporary directory.
func cloneToTemp(repoURL, ref string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "terraclaw-module-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	args := []string{"clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repoURL, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone %s: %w", repoURL, err)
	}

	return tmpDir, nil
}

// parseModule uses terraform-config-inspect to extract metadata from .tf files.
func parseModule(dir string) (*ModuleMetadata, error) {
	mod, diags := tfconfig.LoadModule(dir)
	if diags.HasErrors() {
		return nil, fmt.Errorf("terraform-config-inspect: %s", diags.Error())
	}

	meta := &ModuleMetadata{}

	// Extract resource types.
	typeSet := make(map[string]bool)
	for _, r := range mod.ManagedResources {
		typeSet[r.Type] = true
	}
	for t := range typeSet {
		meta.ResourceTypes = append(meta.ResourceTypes, t)
	}
	sortStrings(meta.ResourceTypes)

	// Extract data sources.
	dsSet := make(map[string]bool)
	for _, d := range mod.DataResources {
		dsSet[d.Type] = true
	}
	for d := range dsSet {
		meta.DataSources = append(meta.DataSources, d)
	}
	sortStrings(meta.DataSources)

	// Extract variables.
	for _, v := range mod.Variables {
		vm := VariableMeta{
			Name:        v.Name,
			Type:        v.Type,
			Description: v.Description,
			Required:    v.Required,
		}
		if v.Default != nil {
			vm.Default = fmt.Sprintf("%v", v.Default)
		}
		meta.Variables = append(meta.Variables, vm)
	}

	// Extract outputs.
	for _, o := range mod.Outputs {
		meta.Outputs = append(meta.Outputs, OutputMeta{
			Name:        o.Name,
			Description: o.Description,
		})
	}

	return meta, nil
}

// deriveName extracts a human-friendly module name from the source or directory.
func deriveName(source, dir string) string {
	// Try subdirectory from git source first.
	if isGitSource(source) {
		_, subdir, _ := parseGitSource(source)
		if subdir != "" {
			return filepath.Base(subdir)
		}
	}

	// Fall back to directory basename.
	base := filepath.Base(dir)
	base = strings.TrimSuffix(base, ".git")
	return base
}

// readDescription reads the first non-empty paragraph from README.md if present.
func readDescription(dir string) string {
	for _, name := range []string{"README.md", "readme.md", "README"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(line) > 200 {
				line = line[:200]
			}
			return line
		}
	}
	return ""
}

// detectProvider infers the cloud provider from resource type prefixes.
func detectProvider(resourceTypes []string) string {
	counts := make(map[string]int)
	for _, rt := range resourceTypes {
		switch {
		case strings.HasPrefix(rt, "aws_"):
			counts["aws"]++
		case strings.HasPrefix(rt, "azurerm_") || strings.HasPrefix(rt, "azure_"):
			counts["azure"]++
		case strings.HasPrefix(rt, "google_"):
			counts["google"]++
		}
	}

	best := ""
	bestCount := 0
	for p, count := range counts {
		if count > bestCount {
			best = p
			bestCount = count
		}
	}
	return best
}

// sortStrings sorts a slice of strings in place (simple insertion sort).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
