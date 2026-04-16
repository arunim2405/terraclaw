package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/modules"
)

var addModuleCmd = &cobra.Command{
	Use:   "add-module <source>",
	Short: "Register a Terraform module from a git repo or local path",
	Long: `Fetch, parse, and register a Terraform module for use during code generation.

When you add a module, terraclaw:
  1. Clones the git repo (or reads the local directory)
  2. Parses all .tf files to extract resource types, variables, and outputs
  3. Detects the cloud provider (AWS, Azure) from resource type prefixes
  4. Reads README.md for a description
  5. Stores the metadata in ~/.cache/terraclaw/modules.db

During code generation, registered modules are matched against your selected
resources by fit score. Matched modules take priority over public registry
modules (e.g. terraform-aws-modules).

Source formats (standard Terraform module source notation):
  Git URL:    git::https://github.com/org/repo.git//modules/vpc?ref=v2.0
  HTTPS URL:  https://github.com/org/repo.git//modules/vpc?ref=v2.0
  SSH URL:    git@github.com:org/repo.git//modules/vpc?ref=v2.0
  Local path: ./my-modules/vpc

The '//' separates the repo URL from the subdirectory within the repo.
The '?ref=' specifies a git tag or branch to clone.

Examples:
  terraclaw add-module "git::https://github.com/acme/infra.git//modules/vpc?ref=v2.0"
  terraclaw add-module "git@github.com:acme/infra.git//modules/rds?ref=main"
  terraclaw add-module ./local-modules/custom-lambda
  terraclaw add-module --name "my-vpc" "https://github.com/acme/infra.git//modules/vpc"`,
	Args: cobra.ExactArgs(1),
	RunE: runAddModule,
}

var listModulesCmd = &cobra.Command{
	Use:   "list-modules",
	Short: "List all registered user modules",
	RunE:  runListModules,
}

var removeModuleCmd = &cobra.Command{
	Use:   "remove-module <name>",
	Short: "Remove a registered module by name",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemoveModule,
}

var inspectModuleCmd = &cobra.Command{
	Use:   "inspect-module <name>",
	Short: "Show full metadata for a registered module",
	Args:  cobra.ExactArgs(1),
	RunE:  runInspectModule,
}

func init() {
	addModuleCmd.Flags().String("name", "", "Override the auto-derived module name")
	rootCmd.AddCommand(addModuleCmd)
	rootCmd.AddCommand(listModulesCmd)
	rootCmd.AddCommand(removeModuleCmd)
	rootCmd.AddCommand(inspectModuleCmd)
}

func runAddModule(cmd *cobra.Command, args []string) error {
	source := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Printf("Scanning module from %s...\n", source)
	meta, err := modules.ScanModule(source)
	if err != nil {
		return fmt.Errorf("scan module: %w", err)
	}

	// Allow name override.
	if nameFlag, _ := cmd.Flags().GetString("name"); nameFlag != "" {
		meta.Name = nameFlag
	}

	store, err := modules.Open(cfg.ModulesDBPath())
	if err != nil {
		return fmt.Errorf("open module store: %w", err)
	}
	defer store.Close()

	if err := store.SaveModule(meta); err != nil {
		return fmt.Errorf("save module: %w", err)
	}

	fmt.Printf("\nModule registered: %s\n", meta.Name)
	fmt.Printf("  Source:    %s\n", meta.Source)
	fmt.Printf("  Provider:  %s\n", meta.ProviderType)
	fmt.Printf("  Resources: %d type(s) — %s\n", len(meta.ResourceTypes), strings.Join(meta.ResourceTypes, ", "))
	fmt.Printf("  Variables: %d (%d required)\n", len(meta.Variables), len(meta.RequiredInputs()))
	fmt.Printf("  Outputs:   %d\n", len(meta.Outputs))
	if meta.Description != "" {
		fmt.Printf("  Desc:      %s\n", meta.Description)
	}

	return nil
}

func runListModules(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store, err := modules.Open(cfg.ModulesDBPath())
	if err != nil {
		return fmt.Errorf("open module store: %w", err)
	}
	defer store.Close()

	mods, err := store.ListModules()
	if err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	if len(mods) == 0 {
		fmt.Println("No modules registered. Use 'terraclaw add-module <source>' to add one.")
		return nil
	}

	fmt.Printf("%-20s %-10s %-12s %s\n", "NAME", "PROVIDER", "RESOURCES", "SOURCE")
	fmt.Printf("%-20s %-10s %-12s %s\n", "----", "--------", "---------", "------")
	for _, m := range mods {
		source := m.Source
		if len(source) > 60 {
			source = source[:57] + "..."
		}
		fmt.Printf("%-20s %-10s %-12d %s\n", m.Name, m.ProviderType, len(m.ResourceTypes), source)
	}

	return nil
}

func runRemoveModule(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store, err := modules.Open(cfg.ModulesDBPath())
	if err != nil {
		return fmt.Errorf("open module store: %w", err)
	}
	defer store.Close()

	if err := store.DeleteModule(name); err != nil {
		return fmt.Errorf("delete module: %w", err)
	}

	fmt.Printf("Module '%s' removed.\n", name)
	return nil
}

func runInspectModule(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store, err := modules.Open(cfg.ModulesDBPath())
	if err != nil {
		return fmt.Errorf("open module store: %w", err)
	}
	defer store.Close()

	m, err := store.GetModule(name)
	if err != nil {
		return fmt.Errorf("get module %q: %w", name, err)
	}

	fmt.Printf("Name:        %s\n", m.Name)
	fmt.Printf("Source:      %s\n", m.Source)
	fmt.Printf("Provider:    %s\n", m.ProviderType)
	fmt.Printf("Description: %s\n", m.Description)
	fmt.Printf("Added:       %s\n", m.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", m.UpdatedAt.Format("2006-01-02 15:04:05"))

	fmt.Printf("\nResource Types (%d):\n", len(m.ResourceTypes))
	for _, rt := range m.ResourceTypes {
		fmt.Printf("  - %s\n", rt)
	}

	fmt.Printf("\nVariables (%d):\n", len(m.Variables))
	for _, v := range m.Variables {
		req := ""
		if v.Required {
			req = " (required)"
		}
		def := ""
		if v.Default != "" {
			def = fmt.Sprintf(" [default: %s]", v.Default)
		}
		desc := ""
		if v.Description != "" {
			desc = fmt.Sprintf(" — %s", v.Description)
		}
		fmt.Printf("  - %s (%s)%s%s%s\n", v.Name, v.Type, req, def, desc)
	}

	fmt.Printf("\nOutputs (%d):\n", len(m.Outputs))
	for _, o := range m.Outputs {
		desc := ""
		if o.Description != "" {
			desc = fmt.Sprintf(" — %s", o.Description)
		}
		fmt.Printf("  - %s%s\n", o.Name, desc)
	}

	if len(m.DataSources) > 0 {
		fmt.Printf("\nData Sources (%d):\n", len(m.DataSources))
		for _, ds := range m.DataSources {
			fmt.Printf("  - %s\n", ds)
		}
	}

	return nil
}
