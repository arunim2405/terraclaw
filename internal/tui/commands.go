package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunim2405/terraclaw/config"
	"github.com/arunim2405/terraclaw/internal/cache"
	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/graph"
	"github.com/arunim2405/terraclaw/internal/llm"
	"github.com/arunim2405/terraclaw/internal/modules"
	"github.com/arunim2405/terraclaw/internal/opencode"
	"github.com/arunim2405/terraclaw/internal/provider"
	"github.com/arunim2405/terraclaw/internal/steampipe"
	tf "github.com/arunim2405/terraclaw/internal/terraform"
)

// appConfig is set once at startup so commands can access it.
var appConfig *config.Config

// steampipeClient is the shared Steampipe client used by commands.
var steampipeClient *steampipe.Client

// opencodeServer is the shared OpenCode server used by commands.
var opencodeServer *opencode.Server

// cacheStore is the shared cache store for persisting scan results.
var cacheStore *cache.Store

// moduleStore is the shared module store for user-registered Terraform modules.
var moduleStore *modules.Store

// SetConfig stores the application config for use by TUI commands.
func SetConfig(cfg *config.Config) { appConfig = cfg }

// SetSteampipeClient stores the Steampipe client for use by TUI commands.
func SetSteampipeClient(c *steampipe.Client) { steampipeClient = c }

// SetOpencodeServer stores the OpenCode server for use by TUI commands.
func SetOpencodeServer(s *opencode.Server) { opencodeServer = s }

// SetCacheStore stores the cache store for use by TUI commands.
func SetCacheStore(s *cache.Store) { cacheStore = s }

// SetModuleStore stores the module store for use by TUI commands.
func SetModuleStore(s *modules.Store) { moduleStore = s }

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
	status     string
	logEntries []string // new activity log lines to append
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
	stage int // 1, 2, or 3
}

// stage3StartedMsg is sent when a Stage 3 import prompt has been dispatched asynchronously.
type stage3StartedMsg struct {
	resultCh  <-chan opencode.PromptResult
	iteration int
}

// importFinishedMsg is sent when Stage 3 import+refinement is complete.
type importFinishedMsg struct {
	files   []llm.GeneratedFile
	results string
	err     error
}

// cacheCheckMsg carries the result of checking for a cached scan.
type cacheCheckMsg struct {
	scanInfo *cache.ScanInfo // nil if no valid cache found
}

// cacheLoadedMsg carries a graph loaded from cache.
type cacheLoadedMsg struct {
	graph *graph.Graph
	err   error
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// checkCacheCmd checks if a valid cached scan exists for the given schema and scan mode.
func checkCacheCmd(schema, scanMode string) tea.Cmd {
	return func() tea.Msg {
		if cacheStore == nil || appConfig == nil || appConfig.NoCache {
			return cacheCheckMsg{scanInfo: nil}
		}

		info, err := cacheStore.LatestScan(schema, scanMode)
		if err != nil {
			debuglog.Log("[cache] error checking cache: %v", err)
			return cacheCheckMsg{scanInfo: nil}
		}

		if info == nil {
			debuglog.Log("[cache] no cached scan found for schema=%s mode=%s", schema, scanMode)
			return cacheCheckMsg{scanInfo: nil}
		}

		// Check TTL.
		age := time.Since(info.FinishedAt)
		if age > appConfig.CacheTTL {
			debuglog.Log("[cache] cached scan expired (age=%s, ttl=%s)", age, appConfig.CacheTTL)
			return cacheCheckMsg{scanInfo: nil}
		}

		debuglog.Log("[cache] valid cached scan found: id=%d, age=%s, resources=%d",
			info.ID, age.Round(time.Second), info.Stats.ResourceCount)
		return cacheCheckMsg{scanInfo: info}
	}
}

// loadCacheCmd loads a graph from a cached scan.
func loadCacheCmd(scanID int64) tea.Cmd {
	return func() tea.Msg {
		if cacheStore == nil {
			return cacheLoadedMsg{err: fmt.Errorf("cache store not available")}
		}

		g, err := cacheStore.LoadGraph(scanID)
		if err != nil {
			debuglog.Log("[cache] error loading cached graph: %v", err)
			return cacheLoadedMsg{err: err}
		}

		debuglog.Log("[cache] loaded graph from cache: %d nodes, %d edges",
			g.Stats.ResourceCount, g.Stats.EdgeCount)
		return cacheLoadedMsg{graph: g}
	}
}

// tickCmd returns a command that sends a spinnerTickMsg after a short delay.
func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// scanStartedMsg is sent when the async scan goroutine has started.
type scanStartedMsg struct {
	progressCh <-chan string
	doneCh     <-chan graphBuiltMsg
}

// scanResourcesCmd starts the resource scan asynchronously and returns immediately.
// Progress updates are sent via a channel and polled by pollScanProgressCmd.
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
			if appConfig != nil && appConfig.ScanTables != "" && appConfig.ScanTables != "*" {
				tables = splitAndTrim(appConfig.ScanTables, ",")
			} else {
				cloud := provider.DetectFromSchema(schema)
				tables = graph.DefaultTablesForProvider(cloud)
			}
			debuglog.Log("[graph] scanning %d key tables for schema=%s", len(tables), schema)
		}

		progressCh := make(chan string, len(tables)+5)
		doneCh := make(chan graphBuiltMsg, 1)

		progressCh <- fmt.Sprintf("Scanning %d resource tables in %q...", len(tables), schema)

		go func() {
			defer close(progressCh)
			defer close(doneCh)

			g := graph.New()
			err := g.Build(steampipeClient, schema, tables, func(scanned, total int, table string) {
				msg := fmt.Sprintf("[%d/%d] Scanning %s...", scanned+1, total, table)
				debuglog.Log("[graph] progress: %s", msg)
				select {
				case progressCh <- msg:
				default:
				}
			})
			if err != nil {
				doneCh <- graphBuiltMsg{err: err}
				return
			}

			progressCh <- fmt.Sprintf("Scanned %d tables, found %d resources. Detecting relationships...",
				g.Stats.TablesScanned, g.Stats.ResourceCount)

			g.DetectRelationships()
			debuglog.Log("[graph] build complete: %d nodes, %d edges", g.Stats.ResourceCount, g.Stats.EdgeCount)

			progressCh <- fmt.Sprintf("Found %d resources with %d relationships. Saving...",
				g.Stats.ResourceCount, g.Stats.EdgeCount)

			if cacheStore != nil && !appConfig.NoCache {
				if err := cacheStore.SaveGraph(schema, scanMode, tables, g); err != nil {
					debuglog.Log("[cache] warning: failed to save scan to cache: %v", err)
				} else {
					debuglog.Log("[cache] scan saved to cache for schema=%s mode=%s", schema, scanMode)
				}
			}

			doneCh <- graphBuiltMsg{graph: g}
		}()

		return scanStartedMsg{progressCh: progressCh, doneCh: doneCh}
	}
}

// pollScanProgressCmd reads progress updates from the scan goroutine.
func pollScanProgressCmd(progressCh <-chan string, doneCh <-chan graphBuiltMsg) tea.Cmd {
	return func() tea.Msg {
		// Check if scan is done (non-blocking).
		select {
		case result := <-doneCh:
			return result
		default:
		}

		// Not done — drain to the latest progress message.
		var latest string
		for {
			select {
			case msg, ok := <-progressCh:
				if !ok {
					// Channel closed — scan goroutine finished, wait for result.
					time.Sleep(100 * time.Millisecond)
					return <-doneCh
				}
				latest = msg
			default:
				// No more queued messages.
				if latest != "" {
					return scanProgressMsg{message: latest}
				}
				time.Sleep(200 * time.Millisecond)
				return scanProgressMsg{message: ""}
			}
		}
	}
}

// generateCodeCmd sets up the OpenCode session and sends the prompt asynchronously.
// It returns a generatingStartedMsg so the TUI can begin polling for progress.
func generateCodeCmd(resources []ResourceItem, schema string, selectedModules ...[]modules.FitResult) tea.Cmd {
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

		// Detect cloud provider from schema.
		cloud := provider.DetectFromSchema(schema)

		// 2. Inject the Stage 1 system prompt with optional user module constraints.
		systemPrompt := llm.BuildStage1SystemPrompt(cloud)
		if len(selectedModules) > 0 && len(selectedModules[0]) > 0 {
			moduleSection := modules.BuildModuleCatalogPrompt(selectedModules[0])
			if moduleSection != "" {
				systemPrompt = systemPrompt + "\n\n" + moduleSection
				debuglog.Log("[opencode] injected %d user module constraint(s) into system prompt", len(selectedModules[0]))
			}
		}
		if err := opencodeServer.InjectSystemPrompt(sessionID, systemPrompt); err != nil {
			return generationDoneMsg{err: fmt.Errorf("inject system prompt: %w", err)}
		}

		// 3. Build the Stage 1 user prompt and send it asynchronously.
		userPrompt := llm.BuildStage1UserPrompt(raw, cloud)
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
func pollAgentStatusCmd(sessionID string, resultCh <-chan opencode.PromptResult, tracker *opencode.MessageTracker) tea.Cmd {
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

		// Log new parts via the tracker and collect activity log entries.
		status := "Agent is thinking..."
		var logEntries []string
		if tracker != nil {
			newParts := tracker.NewParts(messages)
			for _, tp := range newParts {
				if tp.Role != "assistant" {
					continue
				}
				logMessagePart(tp.Part)
				logEntries = append(logEntries, formatPartForActivityLog(tp.Part)...)
			}
			status = extractLatestStatus(messages)
		} else {
			status = extractLatestStatus(messages)
		}

		time.Sleep(2 * time.Second)
		return agentStatusMsg{status: status, logEntries: logEntries}
	}
}

// transitionToStage2Cmd extracts the blueprint from Stage 1, persists it,
// and sends the Stage 2 prompt asynchronously.
func transitionToStage2Cmd(sessionID string, stage1Response string, cloud provider.Cloud) tea.Cmd {
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
		resultCh := opencodeServer.PromptAsync(sessionID, llm.BuildStage2Prompt(blueprintFromDisk, appConfig.OutputDir, cloud))
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

// importViaOpencodeCmd sends a Stage 3 import+refinement prompt to the
// existing OpenCode session. On iteration 1, it sends the initial import
// prompt; on subsequent iterations it sends a refinement prompt.
func importViaOpencodeCmd(sessionID string, iteration int, cloud provider.Cloud) tea.Cmd {
	return func() tea.Msg {
		if opencodeServer == nil {
			return importFinishedMsg{err: fmt.Errorf("opencode server not initialized")}
		}
		if appConfig == nil {
			return importFinishedMsg{err: fmt.Errorf("config not initialized")}
		}

		var prompt string
		if iteration == 1 {
			prompt = llm.BuildStage3Prompt(appConfig.OutputDir, iteration, llm.MaxRefinementIterations, cloud)
		} else {
			prompt = llm.BuildRefinementPrompt(appConfig.OutputDir, iteration, llm.MaxRefinementIterations, cloud)
		}

		debuglog.Log("[opencode] sending stage 3 prompt (iteration %d, %d bytes)", iteration, len(prompt))
		resultCh := opencodeServer.PromptAsync(sessionID, prompt)

		return stage3StartedMsg{resultCh: resultCh, iteration: iteration}
	}
}

// scanAndFinishImportCmd rescans the output directory for generated files
// (which may have been modified during Stage 3 refinement) and returns
// an importFinishedMsg to transition to StepDone.
func scanAndFinishImportCmd(results string) tea.Cmd {
	return func() tea.Msg {
		files, err := llm.RecursiveListGeneratedFiles(appConfig.OutputDir)
		if err != nil {
			debuglog.Log("[opencode] warning: rescan after import failed: %v", err)
		}
		return importFinishedMsg{files: files, results: results}
	}
}

// logMessagePart logs a single message part to the debug log with full content.
func logMessagePart(part opencode.MessagePart) {
	switch {
	case part.IsThinking():
		text := strings.TrimSpace(part.Text)
		if text != "" {
			debuglog.Log("[agent:thinking] %s", text)
		}
	case part.IsText():
		text := strings.TrimSpace(part.Text)
		if text != "" {
			debuglog.Log("[agent:text] %s", text)
		}
	case part.IsToolUse():
		state := part.StateString()
		if state == "" {
			state = "running"
		}
		debuglog.Log("[agent:tool] %s (%s)", part.ToolName, state)
		// Log tool input so the exact command is visible in the debug log.
		if input := string(part.Input); input != "" && input != "null" && input != "{}" {
			debuglog.Log("[agent:tool-input] %s", input)
		}
	case part.IsToolResult():
		output := part.OutputString()
		// Full output — no truncation.
		debuglog.Log("[agent:tool-result] %s", output)
	default:
		if part.Text != "" {
			debuglog.Log("[agent:%s] %s", part.Type, part.Text)
		}
	}
}

// formatPartForActivityLog converts a MessagePart into one or more human-readable
// lines for the TUI activity log. Every tool invocation and its full output is
// included so that the user can see exactly what commands OpenCode runs.
func formatPartForActivityLog(part opencode.MessagePart) []string {
	switch {
	case part.IsThinking():
		text := strings.TrimSpace(part.Text)
		if text == "" {
			return nil
		}
		// Show a condensed thinking indicator.
		if len(text) > 200 {
			text = text[len(text)-200:]
		}
		return []string{"[thinking] " + singleLine(text)}

	case part.IsText():
		text := strings.TrimSpace(part.Text)
		if text == "" {
			return nil
		}
		var lines []string
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		return lines

	case part.IsToolUse():
		state := part.StateString()
		if state == "" {
			state = "running"
		}
		entry := fmt.Sprintf("[tool] %s (%s)", part.ToolName, state)
		// Include tool input if it looks like a shell command.
		input := string(part.Input)
		if input != "" && input != "null" && input != "{}" {
			// Try to extract the command from common input shapes.
			if cmd := extractCommandFromInput(input); cmd != "" {
				entry += "\n       $ " + cmd
			}
		}
		return []string{entry}

	case part.IsToolResult():
		output := part.OutputString()
		if output == "" {
			return nil
		}
		var lines []string
		lines = append(lines, "[result]")
		for _, line := range strings.Split(output, "\n") {
			lines = append(lines, "  "+line)
		}
		return lines

	default:
		if part.Text != "" {
			return []string{fmt.Sprintf("[%s] %s", part.Type, strings.TrimSpace(part.Text))}
		}
		return nil
	}
}

// extractCommandFromInput tries to pull a shell command string from tool input JSON.
// Common shapes: {"command":"..."} or {"content":"..."} or plain string.
func extractCommandFromInput(raw string) string {
	raw = strings.TrimSpace(raw)
	// Try JSON object with "command" key.
	type cmdInput struct {
		Command string `json:"command"`
		Content string `json:"content"`
	}
	var ci cmdInput
	if err := json.Unmarshal([]byte(raw), &ci); err == nil {
		if ci.Command != "" {
			return ci.Command
		}
		if ci.Content != "" {
			return ci.Content
		}
	}
	return ""
}

// singleLine collapses a multiline string into a single line.
func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Collapse multiple spaces.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// extractLatestStatus returns the most relevant status line from the latest
// assistant message. It prioritizes: thinking > tool use > text.
func extractLatestStatus(messages []opencode.SessionMessage) string {
	if len(messages) == 0 {
		return "Waiting for OpenCode to start..."
	}

	// Walk messages in reverse to find the latest assistant message.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Info.Role != "assistant" {
			continue
		}

		var lastThinking string
		var lastTool string
		var lastText string

		for _, part := range msg.Parts {
			switch {
			case part.IsThinking():
				text := strings.TrimSpace(part.Text)
				if text != "" {
					if len(text) > 150 {
						text = text[len(text)-150:]
					}
					lastThinking = text
				}
			case part.IsToolUse():
				if part.ToolName != "" {
					state := part.StateString()
					if state == "" {
						state = "running"
					}
					lastTool = fmt.Sprintf("%s (%s)", part.ToolName, state)
				}
			case part.IsText():
				text := strings.TrimSpace(part.Text)
				if text != "" {
					if len(text) > 150 {
						text = text[len(text)-150:]
					}
					lastText = text
				}
			}
		}

		// Priority: tool (most actionable) > thinking > text.
		if lastTool != "" {
			if lastThinking != "" {
				return fmt.Sprintf("Agent: %s | %s", lastTool, truncate(lastThinking, 80))
			}
			return fmt.Sprintf("Agent: %s", lastTool)
		}
		if lastThinking != "" {
			return fmt.Sprintf("Thinking: %s", lastThinking)
		}
		if lastText != "" {
			return fmt.Sprintf("Agent: %s", lastText)
		}
	}

	return "Agent is thinking..."
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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
