package storage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/michaelbrown/forge/internal/llm"
)

// ExportMarkdown renders a session and its messages as a markdown document.
func ExportMarkdown(sess *Session, messages []llm.Message) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", sess.Title))
	b.WriteString(fmt.Sprintf("- **Session:** %s\n", sess.ID))
	b.WriteString(fmt.Sprintf("- **Provider:** %s\n", sess.Provider))
	b.WriteString(fmt.Sprintf("- **Model:** %s\n", sess.Model))
	if sess.Profile != "" {
		b.WriteString(fmt.Sprintf("- **Profile:** %s\n", sess.Profile))
	}
	b.WriteString(fmt.Sprintf("- **Created:** %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- **Status:** %s\n", sess.Status))
	b.WriteString("\n---\n\n")

	for _, m := range messages {
		switch m.Role {
		case "system":
			continue
		case "user":
			b.WriteString(fmt.Sprintf("## You\n\n%s\n\n", m.Content))
		case "assistant":
			if m.Content != "" {
				b.WriteString(fmt.Sprintf("## Forge\n\n%s\n\n", m.Content))
			}
			for _, tc := range m.ToolCalls {
				args, _ := json.Marshal(tc.Args)
				b.WriteString(fmt.Sprintf("**Tool Call:** `%s`\n```json\n%s\n```\n\n", tc.Name, string(args)))
			}
		case "tool":
			b.WriteString(fmt.Sprintf("<details>\n<summary>Tool Result</summary>\n\n```\n%s\n```\n</details>\n\n", m.Content))
		}
	}

	return b.String()
}

// ExportJSON renders a session and its messages as formatted JSON.
func ExportJSON(sess *Session, messages []llm.Message) ([]byte, error) {
	export := struct {
		Session  *Session      `json:"session"`
		Messages []llm.Message `json:"messages"`
	}{
		Session:  sess,
		Messages: messages,
	}
	return json.MarshalIndent(export, "", "  ")
}
