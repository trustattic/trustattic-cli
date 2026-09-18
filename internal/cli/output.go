// internal/cli/output.go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Mode selects how a response body is rendered.
type Mode int

const (
	// ModeAuto renders styled tables/key-value views on a TTY, and falls
	// back to raw JSON otherwise.
	ModeAuto Mode = iota
	// ModeJSON always renders raw indented JSON.
	ModeJSON
)

// Render writes body (a raw JSON API response) to w, choosing table,
// key-value, or JSON rendering generically from the response's shape - never
// from which endpoint produced it.
func Render(w io.Writer, mode Mode, isTTY bool, body []byte) error {
	if mode == ModeJSON || !isTTY {
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err != nil {
			return fmt.Errorf("indent json: %w", err)
		}
		buf.WriteByte('\n')
		_, err := w.Write(buf.Bytes())
		return err
	}

	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	if obj, ok := v.(map[string]any); ok {
		if data, ok := obj["data"]; ok {
			if rows, ok := data.([]any); ok {
				return renderTable(w, rows)
			}
		}
		return renderKV(w, obj)
	}
	if rows, ok := v.([]any); ok {
		return renderTable(w, rows)
	}
	_, err := fmt.Fprintln(w, string(body))
	return err
}

func renderTable(w io.Writer, rows []any) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "(no results)")
		return err
	}

	seen := map[string]bool{}
	var cols []string
	for _, r := range rows {
		obj, ok := r.(map[string]any)
		if !ok {
			continue
		}
		for k := range obj {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	sort.Strings(cols)

	headerStyle := lipgloss.NewStyle().Bold(true)
	t := table.New().
		Headers(cols...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
		})

	for _, r := range rows {
		obj, _ := r.(map[string]any)
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = fmt.Sprint(obj[c])
		}
		t.Row(row...)
	}
	_, err := fmt.Fprintln(w, t.Render())
	return err
}

func renderKV(w io.Writer, obj map[string]any) error {
	var keys []string
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	keyStyle := lipgloss.NewStyle().Bold(true)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s: %v\n", keyStyle.Render(k), obj[k]); err != nil {
			return err
		}
	}
	return nil
}
