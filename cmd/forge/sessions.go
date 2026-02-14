package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/storage"
	"github.com/michaelbrown/forge/internal/storage/sqlite"
)

var (
	statusFilter string
	limitFlag    int
	exportFormat string
	exportOutput string
	forceFlag    bool
)

var sessionsCmd = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"session", "s"},
	Short:   "Manage chat sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved sessions",
	RunE:  runSessionsList,
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show session details and messages",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsShow,
}

var sessionsResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Resume a previous session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resumeID = args[0]
		return runChat(cmd, args)
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsDelete,
}

var sessionsExportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a session as markdown or JSON",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsExport,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd, sessionsShowCmd, sessionsResumeCmd, sessionsDeleteCmd, sessionsExportCmd)

	sessionsListCmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (active, completed, failed, running)")
	sessionsListCmd.Flags().IntVar(&limitFlag, "limit", 20, "Max sessions to show")

	sessionsExportCmd.Flags().StringVar(&exportFormat, "format", "md", "Export format: md or json")
	sessionsExportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")

	sessionsDeleteCmd.Flags().BoolVar(&forceFlag, "force", false, "Skip confirmation")
}

func openStore() (storage.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return sqlite.Open(cfg.Storage.DBPath)
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	opts := storage.SessionListOptions{
		Status: storage.SessionStatus(statusFilter),
		Limit:  limitFlag,
	}

	sessions, err := store.ListSessions(context.Background(), opts)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	// Header
	fmt.Printf("%-10s %-12s %-40s %-15s %s\n", "ID", "STATUS", "TITLE", "MODEL", "UPDATED")
	fmt.Println(strings.Repeat("─", 95))

	for _, s := range sessions {
		title := s.Title
		if len(title) > 38 {
			title = title[:38] + ".."
		}
		if title == "" {
			title = "(untitled)"
		}

		model := s.Model
		if len(model) > 13 {
			model = model[:13] + ".."
		}

		age := timeAgo(s.UpdatedAt)

		fmt.Printf("%-10s %-12s %-40s %-15s %s\n",
			s.ID[:8], s.Status, title, model, age)
	}

	return nil
}

func runSessionsShow(cmd *cobra.Command, args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	sess, err := store.GetSession(ctx, args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Session:  %s\n", sess.ID)
	fmt.Printf("Title:    %s\n", sess.Title)
	fmt.Printf("Status:   %s\n", sess.Status)
	fmt.Printf("Provider: %s\n", sess.Provider)
	fmt.Printf("Model:    %s\n", sess.Model)
	if sess.Profile != "" {
		fmt.Printf("Profile:  %s\n", sess.Profile)
	}
	fmt.Printf("Created:  %s\n", sess.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:  %s\n", sess.UpdatedAt.Format(time.RFC3339))

	messages, err := store.LoadMessages(ctx, sess.ID)
	if err != nil {
		return err
	}

	fmt.Printf("\nMessages: %d\n", len(messages))
	fmt.Println(strings.Repeat("─", 60))

	for _, m := range messages {
		switch m.Role {
		case "system":
			continue
		case "user":
			fmt.Printf("\n\033[36myou>\033[0m %s\n", truncate(m.Content, 200))
		case "assistant":
			if m.Content != "" {
				fmt.Printf("\n\033[32mforge>\033[0m %s\n", truncate(m.Content, 200))
			}
			for _, tc := range m.ToolCalls {
				fmt.Printf("  \033[33m⚡ %s\033[0m\n", tc.Name)
			}
		case "tool":
			fmt.Printf("  \033[90m│ %s\033[0m\n", truncate(m.Content, 100))
		}
	}

	return nil
}

func runSessionsDelete(cmd *cobra.Command, args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	sess, err := store.GetSession(ctx, args[0])
	if err != nil {
		return err
	}

	if !forceFlag {
		title := sess.Title
		if title == "" {
			title = "(untitled)"
		}
		fmt.Printf("Delete session %s - %q? [y/N] ", sess.ID[:8], title)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := store.DeleteSession(ctx, sess.ID); err != nil {
		return err
	}
	fmt.Printf("Deleted session %s\n", sess.ID[:8])
	return nil
}

func runSessionsExport(cmd *cobra.Command, args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	sess, err := store.GetSession(ctx, args[0])
	if err != nil {
		return err
	}

	messages, err := store.LoadMessages(ctx, sess.ID)
	if err != nil {
		return err
	}

	var output string
	switch exportFormat {
	case "json":
		data, err := storage.ExportJSON(sess, messages)
		if err != nil {
			return err
		}
		output = string(data)
	default:
		output = storage.ExportMarkdown(sess, messages)
	}

	if exportOutput != "" {
		return os.WriteFile(exportOutput, []byte(output), 0o644)
	}

	fmt.Print(output)
	return nil
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
