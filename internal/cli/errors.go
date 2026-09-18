package cli

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

// APIError wraps a non-2xx API response so it can be rendered consistently.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, string(e.Body))
}

// RenderError writes a styled error box for err to w.
func RenderError(w io.Writer, err error) {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("9")).
		Padding(0, 1)
	fmt.Fprintln(w, box.Render("Error: "+err.Error()))
}
