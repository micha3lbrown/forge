package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/michaelbrown/forge/internal/agent"
	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/storage"
	"github.com/michaelbrown/forge/internal/storage/sqlite"
	"github.com/michaelbrown/forge/internal/tools"
)

var resumeID string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with an agent",
	Long: `Start an interactive conversation with a Forge agent.
The agent can use tools to help answer your questions.

Examples:
  forge chat
  forge chat --provider claude
  forge chat --provider ollama --model qwen3:8b
  forge chat --resume <session-id>`,
	RunE: runChat,
}

func init() {
	chatCmd.Flags().StringVar(&resumeID, "resume", "", "Resume a previous session by ID or prefix")
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Open storage
	store, err := sqlite.Open(cfg.Storage.DBPath)
	if err != nil {
		return fmt.Errorf("opening storage: %w", err)
	}
	defer store.Close()

	// Load agent profile if specified
	var profile *agent.Profile
	if profileFlag != "" {
		profilePath := filepath.Join(cfg.Agent.ProfilesDir, profileFlag+".yaml")
		profile, err = agent.LoadProfile(profilePath)
		if err != nil {
			return fmt.Errorf("loading profile: %w", err)
		}
	}

	providerName := providerFlag
	if providerName == "" {
		if profile != nil && profile.Provider != "" {
			providerName = profile.Provider
		} else {
			providerName = cfg.DefaultProvider
		}
	}

	provider, err := cfg.Provider(providerName)
	if err != nil {
		return err
	}

	model := modelFlag
	if model == "" {
		if profile != nil && profile.Model != "" {
			model = profile.Model
		} else if provider.IsOllama() {
			picked, err := pickOllamaModel(provider, provider.Models["default"])
			if err == nil {
				model = picked
			} else {
				model = provider.Models["default"]
			}
		} else {
			model = provider.Models["default"]
		}
	}

	maxIter := cfg.Agent.MaxIterations
	if profile != nil && profile.MaxIter > 0 {
		maxIter = profile.MaxIter
	}

	fmt.Printf("Forge - Interactive Agent Chat\n")
	if profile != nil {
		fmt.Printf("Profile: %s\n", profile.Name)
	}
	fmt.Printf("Provider: %s | Model: %s\n", providerName, model)

	// Create tool registry from config
	registry := tools.NewRegistry()
	defer registry.Close()

	for name, toolCfg := range cfg.Tools {
		if err := registry.Register(name, toolCfg); err != nil {
			fmt.Printf("Warning: failed to start tool server %s: %v\n", name, err)
		}
	}

	if registry.HasTools() {
		fmt.Printf("Tools: MCP servers loaded\n")
	} else {
		fmt.Printf("Tools: builtin shell_exec\n")
	}

	client := llm.NewClient(provider.BaseURL, provider.APIKey, model)
	a := agent.New(client, registry, maxIter)
	a.SetMaxTokens(cfg.Agent.ContextMaxTokens)

	// Create utility LLM if configured
	if utilityModel, ok := provider.Models["utility"]; ok && utilityModel != "" {
		utilityClient := llm.NewClient(provider.BaseURL, provider.APIKey, utilityModel)
		a.SetUtilityLLM(utilityClient)
		fmt.Printf("Utility model: %s\n", utilityModel)
	}

	// Apply profile overrides
	if profile != nil {
		a.SetSystemPrompt(profile.SystemPrompt)
		a.FilterTools(profile.Tools)
	}

	// Create or resume session
	ctx := context.Background()
	var sess *storage.Session

	if resumeID != "" {
		sess, err = store.GetSession(ctx, resumeID)
		if err != nil {
			return fmt.Errorf("loading session: %w", err)
		}
		messages, err := store.LoadMessages(ctx, sess.ID)
		if err != nil {
			return fmt.Errorf("loading messages: %w", err)
		}
		a.SetHistory(messages)
		sess.Status = storage.StatusActive
		store.UpdateSession(ctx, sess)
		fmt.Printf("Session: %s (resumed)\n", sess.ID[:8])
	} else {
		sess = &storage.Session{
			ID:       uuid.New().String(),
			Status:   storage.StatusActive,
			Provider: providerName,
			Model:    model,
			Profile:  profileFlag,
		}
		if err := store.CreateSession(ctx, sess); err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		fmt.Printf("Session: %s\n", sess.ID[:8])
	}

	cs := &chatState{
		agent:        a,
		cfg:          cfg,
		providerName: providerName,
		model:        model,
		sess:         sess,
		store:        store,
	}

	fmt.Printf("Type /help for commands, /quit to exit\n\n")

	// Wire up callbacks for display
	a.OnTextDelta = func(delta string) {
		fmt.Print(delta)
	}
	a.OnToolCall = func(name string, args map[string]any) {
		fmt.Printf("\n  \033[33m⚡ Tool: %s\033[0m\n", agent.FormatToolCall(name, args))
	}
	a.OnToolResult = func(name string, result string) {
		lines := strings.Split(strings.TrimSpace(result), "\n")
		preview := lines
		if len(preview) > 8 {
			preview = preview[:8]
		}
		for _, line := range preview {
			fmt.Printf("  \033[90m│ %s\033[0m\n", line)
		}
		if len(lines) > 8 {
			fmt.Printf("  \033[90m│ ... (%d more lines)\033[0m\n", len(lines)-8)
		}
		fmt.Println()
	}

	// Set up readline for input with history
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[36myou>\033[0m ",
		HistoryFile:     "/tmp/forge_history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}
	defer rl.Close()

	// Mark session completed on exit
	defer func() {
		if sess.Status == storage.StatusActive {
			sess.Status = storage.StatusCompleted
			store.UpdateSession(ctx, sess)
		}
	}()

	// Per-request cancellation
	var reqCancel context.CancelFunc
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range sigCh {
			if reqCancel != nil {
				reqCancel()
			}
		}
	}()

	firstMessage := resumeID == "" // track if we need to generate a title

	for {
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye!")
				return nil
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, cs) {
				continue
			}
		}

		// Auto-generate title from first user message
		if firstMessage {
			sess.Title = generateTitle(input)
			store.UpdateSession(ctx, sess)
			firstMessage = false
		}

		// Create a per-request context so Ctrl+C only cancels this request
		reqCtx, cancel := context.WithCancel(context.Background())
		reqCancel = cancel

		// Run the agent with streaming output
		fmt.Printf("\n\033[32mforge>\033[0m ")
		_, err = a.RunStreaming(reqCtx, input)
		wasInterrupted := reqCtx.Err() != nil
		cancel()
		reqCancel = nil

		// Auto-save after each turn
		if saveErr := store.SaveMessages(ctx, sess.ID, a.History()); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save session: %v\n", saveErr)
		}

		if err != nil {
			if wasInterrupted {
				fmt.Println("\n(interrupted)")
				continue
			}
			fmt.Printf("\n\033[31merror: %s\033[0m\n\n", err)
			continue
		}

		fmt.Printf("\n\n")
	}
}

func generateTitle(firstMessage string) string {
	t := strings.TrimSpace(firstMessage)
	if len(t) > 80 {
		t = t[:80] + "..."
	}
	return t
}

// chatState holds mutable session state for the chat loop.
type chatState struct {
	agent        *agent.Agent
	cfg          *config.Config
	providerName string
	model        string
	sess         *storage.Session
	store        storage.Store
}

func handleCommand(input string, cs *chatState) bool {
	fields := strings.Fields(input)
	switch strings.ToLower(fields[0]) {
	case "/quit", "/exit", "/q":
		fmt.Println("Goodbye!")
		os.Exit(0)
	case "/reset":
		cs.agent.Reset()
		fmt.Println("Conversation reset.")
		fmt.Println()
	case "/history":
		fmt.Println(cs.agent.HistoryJSON())
		fmt.Println()
	case "/model":
		handleModelCommand(fields[1:], cs)
	case "/help":
		fmt.Println("Commands:")
		fmt.Println("  /help              - Show this help")
		fmt.Println("  /model             - Show current provider and model")
		fmt.Println("  /model <model>     - Switch model (e.g. /model qwen3:8b)")
		fmt.Println("  /model <p>/<model> - Switch provider and model (e.g. /model claude/claude-sonnet-4-5-20250929)")
		fmt.Println("  /reset             - Clear conversation history")
		fmt.Println("  /history           - Show raw conversation history (JSON)")
		fmt.Println("  /quit              - Exit")
		fmt.Println()
	default:
		fmt.Printf("Unknown command: %s (try /help)\n\n", input)
	}
	return true
}

func handleModelCommand(args []string, cs *chatState) {
	// No args: show current model
	if len(args) == 0 {
		fmt.Printf("Provider: %s | Model: %s\n\n", cs.providerName, cs.model)
		return
	}

	target := args[0]
	newProvider := cs.providerName
	newModel := target

	// Check for provider/model syntax
	if idx := strings.Index(target, "/"); idx > 0 {
		newProvider = target[:idx]
		newModel = target[idx+1:]
	}

	// Look up provider config
	providerCfg, err := cs.cfg.Provider(newProvider)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
		return
	}

	// Create new client and swap
	newClient := llm.NewClient(providerCfg.BaseURL, providerCfg.APIKey, newModel)
	cs.agent.SetClient(newClient)
	cs.providerName = newProvider
	cs.model = newModel

	// Update session metadata
	cs.sess.Provider = newProvider
	cs.sess.Model = newModel
	ctx := context.Background()
	cs.store.UpdateSession(ctx, cs.sess)

	fmt.Printf("Switched to %s/%s\n\n", newProvider, newModel)
}

// pickOllamaModel queries Ollama for available models and lets the user choose.
func pickOllamaModel(provider config.ProviderConfig, defaultModel string) (string, error) {
	client := llm.NewClient(provider.BaseURL, provider.APIKey, "")
	models, err := client.ListModels(context.Background())
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	fmt.Println("Available models:")
	defaultIdx := -1
	for i, m := range models {
		sizeGB := float64(m.Size) / (1024 * 1024 * 1024)
		marker := "  "
		if m.Name == defaultModel {
			marker = "* "
			defaultIdx = i
		}
		fmt.Printf("  %s%d) %-30s (%.1f GB)\n", marker, i+1, m.Name, sizeGB)
	}

	defaultHint := ""
	if defaultIdx >= 0 {
		defaultHint = fmt.Sprintf(" [%d]", defaultIdx+1)
	}
	fmt.Printf("\nSelect model%s: ", defaultHint)

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("no input")
	}
	choice := strings.TrimSpace(scanner.Text())

	if choice == "" && defaultIdx >= 0 {
		return models[defaultIdx].Name, nil
	}

	n, err := strconv.Atoi(choice)
	if err != nil || n < 1 || n > len(models) {
		return "", fmt.Errorf("invalid selection: %s", choice)
	}
	return models[n-1].Name, nil
}
