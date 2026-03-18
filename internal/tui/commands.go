package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/graph"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	tf "github.com/arunim2405/terraclaw/internal/terraform"
)

// appConfig is set once at startup so commands can access it.
var appConfig *config.Config

// steampipeClient is the shared Steampipe client used by commands.
var steampipeClient *steampipe.Client

// opencodeServer is the shared OpenCode server used by commands.
var opencodeServer *opencode.Server

// SetConfig stores the application config for use by TUI commands.
func SetConfig(cfg *config.Config) { appConfig = cfg }

// SetSteampipeClient stores the Steampipe client for use by TUI commands.
func SetSteampipeClient(c *steampipe.Client) { steampipeClient = c }

// SetOpencodeServer stores the OpenCode server for use by TUI commands.
func SetOpencodeServer(s *opencode.Server) { opencodeServer = s }

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// spinnerTickMsg triggers a spinner frame update.
type spinnerTickMsg struct{}

// scanProgressMsg reports scan progress.
type scanProgressMsg struct {
	message string
}

// graphBuiltMsg carries the completed resource graph.
type graphBuiltMsg struct {
	graph *graph.Graph
	err   error
}

// asyncResultMsg carries the result of an async code generation or import.
type asyncResultMsg asyncResult

// generatingStartedMsg is sent when the OpenCode session is set up
// and the prompt has been sent asynchronously.
type generatingStartedMsg struct {
	sessionID string
	resultCh  <-chan opencode.PromptResult
}

// agentStatusMsg carries a status update from polling OpenCode messages.
type agentStatusMsg struct {
	status string
}

// generationDoneMsg is sent when the async prompt completes and files are scanned.
type generationDoneMsg struct {
	files []llm.GeneratedFile
	err   error
}

// promptDoneMsg is sent when an async prompt completes (either stage).
type promptDoneMsg struct {
	response string
	err      error
}

// stage2StartedMsg is sent when Stage 2 prompt has been dispatched asynchronously.
type stage2StartedMsg struct {
	resultCh <-chan opencode.PromptResult
}

// stageTransitionMsg signals the TUI to update the stage display.
type stageTransitionMsg struct {
	stage int // 1 or 2
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// tickCmd returns a command that sends a spinnerTickMsg after a short delay.
func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// scanResourcesCmd scans cloud resources and builds the dependency graph.
func scanResourcesCmd(schema string, scanMode string) tea.Cmd {
	return func() tea.Msg {
		if steampipeClient == nil {
			return graphBuiltMsg{err: fmt.Errorf("steampipe client not initialized")}
		}

		// Determine which tables to scan.
		var tables []string
		switch scanMode {
		case "all":
			debuglog.Log("[graph] scanning ALL tables for schema=%s", schema)
			var err error
			tables, err = steampipeClient.ListTables(schema)
			if err != nil {
				return graphBuiltMsg{err: fmt.Errorf("list tables: %w", err)}
			}
		default: // "key"
			// Use configured tables if set, else default AWS tables.
			if appConfig != nil && appConfig.ScanTables != "" && appConfig.ScanTables != "*" {
				tables = splitAndTrim(appConfig.ScanTables, ",")
			} else {
				tables = graph.DefaultAWSTables
			}
			debuglog.Log("[graph] scanning %d key tables for schema=%s", len(tables), schema)
		}

		g := graph.New()
		err := g.Build(steampipeClient, schema, tables, func(scanned, total int, table string) {
			debuglog.Log("[graph] progress: %d/%d — %s", scanned, total, table)
		})
		if err != nil {
			return graphBuiltMsg{err: err}
		}

		// Detect relationships between resources.
		g.DetectRelationships()
		debuglog.Log("[graph] build complete: %d nodes, %d edges", g.Stats.ResourceCount, g.Stats.EdgeCount)

		return graphBuiltMsg{graph: g}
	}
}

// generateCodeCmd sets up the OpenCode session and sends the prompt asynchronously.
// It returns a generatingStartedMsg so the TUI can begin polling for progress.
func generateCodeCmd(resources []ResourceItem) tea.Cmd {
	return func() tea.Msg {
		debuglog.Log("[opencode] generateCode called: resources=%d", len(resources))
		if opencodeServer == nil {
			debuglog.Log("[opencode] ERROR: server not initialized")
			return generationDoneMsg{err: fmt.Errorf("opencode server not initialized")}
		}
		if appConfig == nil {
			return generationDoneMsg{err: fmt.Errorf("config not initialized")}
		}

		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		outputDir := appConfig.OutputDir
		debuglog.Log("[opencode] setting up session with %d resource(s), outputDir=%s", len(raw), outputDir)

		// 1. Create a session.
		sessionID, err := opencodeServer.CreateSession("terraclaw-terraform-generation")
		if err != nil {
			return generationDoneMsg{err: fmt.Errorf("create session: %w", err)}
		}
		debuglog.Log("[opencode] session created: %s", sessionID)

		// 2. Inject the Stage 1 system prompt.
		if err := opencodeServer.InjectSystemPrompt(sessionID, llm.BuildStage1SystemPrompt()); err != nil {
			return generationDoneMsg{err: fmt.Errorf("inject system prompt: %w", err)}
		}

		// 3. Build the Stage 1 user prompt and send it asynchronously.
		userPrompt := llm.BuildStage1UserPrompt(raw)
		debuglog.Log("[opencode] sending stage 1 prompt async (%d bytes)", len(userPrompt))

		resultCh := opencodeServer.PromptAsync(sessionID, userPrompt)

		// Return immediately so the TUI can start polling.
		return generatingStartedMsg{
			sessionID: sessionID,
			resultCh:  resultCh,
		}
	}
}

// pollAgentStatusCmd polls OpenCode for session messages and checks if the current prompt is done.
func pollAgentStatusCmd(sessionID string, resultCh <-chan opencode.PromptResult) tea.Cmd {
	return func() tea.Msg {
		// First check if the prompt has completed.
		select {
		case result := <-resultCh:
			debuglog.Log("[opencode] prompt completed (err=%v)", result.Err)
			return promptDoneMsg{response: result.Response, err: result.Err}
		default:
			// Not done yet — poll for status.
		}

		// Poll session messages to get agent status.
		messages, err := opencodeServer.ListMessages(sessionID)
		if err != nil {
			debuglog.Log("[opencode] poll error: %v", err)
			// Non-fatal — keep polling.
			time.Sleep(2 * time.Second)
			return agentStatusMsg{status: "Generating..."}
		}

		status := extractAgentStatus(messages)
		time.Sleep(2 * time.Second)
		return agentStatusMsg{status: status}
	}
}

// transitionToStage2Cmd extracts the blueprint from Stage 1, persists it,
// and sends the Stage 2 prompt asynchronously.
func transitionToStage2Cmd(sessionID string, stage1Response string) tea.Cmd {
	return func() tea.Msg {
		blueprint, err := llm.ExtractBlueprint(stage1Response)
		if err != nil {
			return generationDoneMsg{err: fmt.Errorf("extract blueprint: %w", err)}
		}
		if err := llm.PersistBlueprint(blueprint, appConfig.OutputDir); err != nil {
			return generationDoneMsg{err: fmt.Errorf("persist blueprint: %w", err)}
		}
		debuglog.Log("[opencode] blueprint persisted to %s/blueprint.yaml", appConfig.OutputDir)

		blueprintFromDisk, err := llm.ReadBlueprint(appConfig.OutputDir)
		if err != nil {
			return generationDoneMsg{err: fmt.Errorf("read blueprint: %w", err)}
		}

		// Send Stage 2 prompt async.
		resultCh := opencodeServer.PromptAsync(sessionID, llm.BuildStage2Prompt(blueprintFromDisk, appConfig.OutputDir))
		debuglog.Log("[opencode] stage 2 prompt sent async for session %s", sessionID)

		return stage2StartedMsg{resultCh: resultCh}
	}
}

// scanGeneratedFilesCmd scans the output directory for generated files.
func scanGeneratedFilesCmd() tea.Cmd {
	return func() tea.Msg {
		files, err := llm.RecursiveListGeneratedFiles(appConfig.OutputDir)
		if err != nil {
			return generationDoneMsg{err: fmt.Errorf("list generated files: %w", err)}
		}
		if len(files) == 0 {
			return generationDoneMsg{err: fmt.Errorf("no files were generated")}
		}
		debuglog.Log("[opencode] found %d generated file(s)", len(files))
		return generationDoneMsg{files: files}
	}
}

// extractAgentStatus summarizes the latest agent activity from session messages.
func extractAgentStatus(messages []opencode.SessionMessage) string {
	if len(messages) == 0 {
		return "Waiting for OpenCode to start..."
	}

	// Walk messages in reverse to find the latest assistant message.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Info.Role != "assistant" {
			continue
		}

		var statusParts []string
		var lastToolName string
		var lastText string

		for _, part := range msg.Parts {
			switch part.Type {
			case "tool-invocation", "tool-use":
				if part.ToolName != "" {
					lastToolName = part.ToolName
					state := part.StateString()
					if state == "" {
						state = "running"
					}
					statusParts = append(statusParts, fmt.Sprintf("🔧 %s (%s)", part.ToolName, state))
				}
			case "text":
				if part.Text != "" {
					// Grab last ~80 chars of text as a preview.
					text := strings.TrimSpace(part.Text)
					if len(text) > 100 {
						text = text[len(text)-100:]
					}
					lastText = text
				}
			}
		}

		if len(statusParts) > 0 {
			// Show the most recent tool use.
			latest := statusParts[len(statusParts)-1]
			return fmt.Sprintf("Agent: %s", latest)
		}
		if lastToolName != "" {
			return fmt.Sprintf("Agent: Using %s", lastToolName)
		}
		if lastText != "" {
			return fmt.Sprintf("Agent: %s", lastText)
		}
	}

	return "Agent is thinking..."
}

// runImportCmd runs terraform import for the selected resources.
// With the two-stage pipeline, import.sh is generated at the root of outputDir
// (not inside module subdirectories), so ImportScriptExists correctly checks
// filepath.Join(outputDir, "import.sh") regardless of the module directory
// structure underneath. If import.sh is absent, the fallback uses
// GuessResourceAddress for per-resource imports.
func runImportCmd(resources []ResourceItem, _ string) tea.Cmd {
	return func() tea.Msg {
		debuglog.Log("[terraform] runImport called for %d resource(s)", len(resources))
		if appConfig == nil {
			debuglog.Log("[terraform] ERROR: config not initialized")
			return asyncResultMsg{err: fmt.Errorf("config not initialized")}
		}

		// Prefer import.sh if OpenCode generated it at the outputDir root.
		if tf.ImportScriptExists(appConfig.OutputDir) {
			debuglog.Log("[terraform] found import.sh, running script")
			output, err := tf.RunImportScript(appConfig.OutputDir)
			if err != nil {
				debuglog.Log("[terraform] import.sh failed: %v", err)
				return asyncResultMsg{imports: fmt.Sprintf("import.sh output:\n%s\n\nError: %v", output, err)}
			}
			debuglog.Log("[terraform] import.sh complete")
			return asyncResultMsg{imports: fmt.Sprintf("import.sh output:\n%s\n\n✅ All imports completed successfully!", output)}
		}

		// Fallback: run per-resource imports using GuessResourceAddress.
		debuglog.Log("[terraform] no import.sh found, falling back to per-resource imports")
		raw := make([]steampipe.Resource, 0, len(resources))
		for _, ri := range resources {
			if r, ok := ri.Resource.(steampipe.Resource); ok {
				raw = append(raw, r)
			}
		}

		debuglog.Log("[terraform] running import with bin=%s outputDir=%s", appConfig.TerraformBin, appConfig.OutputDir)
		results := tf.RunImports(appConfig.TerraformBin, appConfig.OutputDir, raw)
		summary := tf.SummaryText(results)
		debuglog.Log("[terraform] import complete: %s", summary)
		return asyncResultMsg{imports: summary}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitAndTrim splits a string by sep and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	for {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// updateForMessages extends Model.Update to handle data-load messages.
func (m Model) updateForMessages(msg tea.Msg) (Model, tea.Cmd, bool) {
	_ = msg
	return m, nil, false
}
