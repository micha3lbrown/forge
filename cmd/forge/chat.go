package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"

	"path/filepath"

	"github.com/michaelbrown/forge/internal/agent"
	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/tools"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with an agent",
	Long: `Start an interactive conversation with a Forge agent.
The agent can use tools to help answer your questions.

Examples:
  forge chat
  forge chat --provider claude
  forge chat --provider ollama --model qwen3:8b`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

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

	fmt.Printf("Type /help for commands, /quit to exit\n\n")

	client := llm.NewClient(provider.BaseURL, provider.APIKey, model)
	a := agent.New(client, registry, maxIter)

	// Apply profile overrides
	if profile != nil {
		a.SetSystemPrompt(profile.SystemPrompt)
		a.FilterTools(profile.Tools)
	}

	// Wire up callbacks for display
	a.OnTextDelta = func(delta string) {
		fmt.Print(delta)
	}
	a.OnToolCall = func(name string, args map[string]any) {
		fmt.Printf("\n  \033[33m⚡ Tool: %s\033[0m\n", agent.FormatToolCall(name, args))
	}
	a.OnToolResult = func(name string, result string) {
		// Show first few lines of result
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

	// Per-request cancellation: Ctrl+C cancels the active LLM request,
	// not the whole app. A second Ctrl+C while idle exits.
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
			if handleCommand(input, a) {
				continue
			}
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

func handleCommand(input string, a *agent.Agent) bool {
	switch strings.ToLower(strings.Fields(input)[0]) {
	case "/quit", "/exit", "/q":
		fmt.Println("Goodbye!")
		os.Exit(0)
	case "/reset":
		a.Reset()
		fmt.Println("Conversation reset.")
		fmt.Println()
	case "/history":
		fmt.Println(a.HistoryJSON())
		fmt.Println()
	case "/help":
		fmt.Println("Commands:")
		fmt.Println("  /help     - Show this help")
		fmt.Println("  /reset    - Clear conversation history")
		fmt.Println("  /history  - Show raw conversation history (JSON)")
		fmt.Println("  /quit     - Exit")
		fmt.Println()
	default:
		fmt.Printf("Unknown command: %s (try /help)\n\n", input)
	}
	return true
}
