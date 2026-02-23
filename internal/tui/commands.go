package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	tf "github.com/arunim2405/terraclaw/internal/terraform"
)

// appConfig is set once at startup so commands can access it.
var appConfig *config.Config

// steampipeClient is the shared Steampipe client used by commands.
var steampipeClient *steampipe.Client

// SetConfig stores the application config for use by TUI commands.
func SetConfig(cfg *config.Config) { appConfig = cfg }

// SetSteampipeClient stores the Steampipe client for use by TUI commands.
func SetSteampipeClient(c *steampipe.Client) { steampipeClient = c }

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// spinnerTickMsg triggers a spinner frame update.
type spinnerTickMsg struct{}

// tablesLoadedMsg carries the tables fetched from Steampipe.
type tablesLoadedMsg struct {
	tables []string
	err    error
}

// resourcesLoadedMsg carries the resources fetched from Steampipe.
type resourcesLoadedMsg struct {
	resources []ResourceItem
	err       error
}

// asyncResultMsg carries the result of an async code generation or import.
type asyncResultMsg asyncResult

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// tickCmd returns a command that sends a spinnerTickMsg after a short delay.
func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// fetchTablesCmd loads the available tables for the selected schema.
func fetchTablesCmd(schema string) tea.Cmd {
	return func() tea.Msg {
		if steampipeClient == nil {
			return tablesLoadedMsg{err: fmt.Errorf("steampipe client not initialized")}
		}
		tables, err := steampipeClient.ListTables(schema)
		return tablesLoadedMsg{tables: tables, err: err}
	}
}

// fetchResourcesCmd loads resources for a given schema/table.
func fetchResourcesCmd(schema, table string) tea.Cmd {
	return func() tea.Msg {
		if steampipeClient == nil {
			return resourcesLoadedMsg{err: fmt.Errorf("steampipe client not initialized")}
		}
		raw, err := steampipeClient.FetchResources(schema, table)
		if err != nil {
			return resourcesLoadedMsg{err: err}
		}
		items := make([]ResourceItem, len(raw))
		for i, r := range raw {
			label := r.Name
			if label == "" {
				label = r.ID
			}
			items[i] = ResourceItem{
				Resource: r,
				label:    label,
			}
		}
		return resourcesLoadedMsg{resources: items, err: nil}
	}
}

// generateCodeCmd calls the selected LLM to generate Terraform code.
func generateCodeCmd(providerName string, resources []ResourceItem) tea.Cmd {
	return func() tea.Msg {
		if appConfig == nil {
			return asyncResultMsg{err: fmt.Errorf("config not initialized")}
		}

		// Map provider display name back to config provider.
		switch {
		case strings.Contains(providerName, "OpenAI"):
			appConfig.LLMProvider = config.ProviderOpenAI
		case strings.Contains(providerName, "Anthropic"):
			appConfig.LLMProvider = config.ProviderClaude
		case strings.Contains(providerName, "Google"):
			appConfig.LLMProvider = config.ProviderGemini
		}

		provider, err := llm.New(appConfig)
		if err != nil {
			return asyncResultMsg{err: err}
		}

		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		code, err := provider.GenerateTerraform(context.Background(), raw)
		if err != nil {
			return asyncResultMsg{err: err}
		}

		// Also write the code to disk.
		if appConfig != nil {
			_, _ = tf.WriteConfig(appConfig.OutputDir, code)
		}

		return asyncResultMsg{code: code}
	}
}

// runImportCmd runs terraform import for the selected resources.
func runImportCmd(resources []ResourceItem, _ string) tea.Cmd {
	return func() tea.Msg {
		if appConfig == nil {
			return asyncResultMsg{err: fmt.Errorf("config not initialized")}
		}

		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		results := tf.RunImports(appConfig.TerraformBin, appConfig.OutputDir, raw)
		summary := tf.SummaryText(results)
		return asyncResultMsg{imports: summary}
	}
}

// ---------------------------------------------------------------------------
// Model update additions for loaded data
// ---------------------------------------------------------------------------

// UpdateForMessages extends Model.Update to handle data-load messages.
// It returns the updated model, a command, and whether the message was handled.
func (m Model) updateForMessages(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tablesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil, true
		}
		m.tables = msg.tables
		m.step = StepSelectTable
		m.list = buildTableList(msg.tables)
		return m, nil, true

	case resourcesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil, true
		}
		m.resources = msg.resources
		m.selectedResources = nil
		m.step = StepSelectResources

		items := make([]list.Item, len(msg.resources))
		for i, r := range msg.resources {
			items[i] = listItem{
				title: "  " + r.label,
				desc:  r.Resource.String(),
			}
		}
		m.list = newList(items, fmt.Sprintf("Select Resources from %s.%s", m.selectedSchema, m.selectedTable), 0)
		return m, nil, true
	}
	return m, nil, false
}
