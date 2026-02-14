package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	providerFlag string
	modelFlag    string
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "Forge - Local agentic AI platform",
	Long: `Forge is a local agentic AI platform for learning and building with AI agents.

It connects to Ollama, Claude, or Gemini and provides tools for code execution,
research, and multi-agent collaboration.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&providerFlag, "provider", "", "LLM provider (ollama, claude, gemini)")
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "Model to use (overrides config)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
