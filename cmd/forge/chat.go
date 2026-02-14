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

	"github.com/michaelbrown/forge/internal/agent"
	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/llm"
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

	providerName := providerFlag
	if providerName == "" {
		providerName = cfg.DefaultProvider
	}

	provider, err := cfg.Provider(providerName)
	if err != nil {
		return err
	}

	model := modelFlag
	if model == "" {
		model = provider.Models["default"]
	}

	fmt.Printf("Forge - Interactive Agent Chat\n")
	fmt.Printf("Provider: %s | Model: %s\n", providerName, model)
	fmt.Printf("Type /help for commands, /quit to exit\n\n")

	client := llm.NewClient(provider.BaseURL, provider.APIKey, model)
	a := agent.New(client, cfg.Agent.MaxIterations)

	// Wire up tool call callbacks for display
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

	// Handle Ctrl+C gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
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

		// Run the agent
		fmt.Printf("\n\033[32mforge>\033[0m ")
		response, err := a.Run(ctx, input)
		if err != nil {
			if ctx.Err() != nil {
				fmt.Println("\n(interrupted)")
				return nil
			}
			fmt.Printf("\n\033[31merror: %s\033[0m\n\n", err)
			continue
		}

		fmt.Printf("%s\n\n", response)
	}
}

func handleCommand(input string, a *agent.Agent) bool {
	switch strings.ToLower(strings.Fields(input)[0]) {
	case "/quit", "/exit", "/q":
		fmt.Println("Goodbye!")
		os.Exit(0)
	case "/reset":
		a.Reset()
		fmt.Println("Conversation reset.\n")
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
