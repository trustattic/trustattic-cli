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

// ParseMode turns a --output flag value into a Mode, rejecting anything
// that isn't a mode the CLI actually implements. Without this, a typo (or a
// plausible-but-unsupported value like "yaml") silently fell through to the
// styled/auto renderer and the user got no diagnostic at all.
func ParseMode(value string) (Mode, error) {
	switch value {
	case "":
		return ModeAuto, nil
	case "json":
		return ModeJSON, nil
	default:
		return ModeAuto, fmt.Errorf("unknown --output value %q: must be %q or omitted", value, "json")
	}
}

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
			if kv, ok := data.(map[string]any); ok {
				return renderKV(w, kv)
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
	anyObjects := false
	for _, r := range rows {
		obj, ok := r.(map[string]any)
		if !ok {
			continue
		}
		anyObjects = true
		for k := range obj {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	if !anyObjects {
		for _, r := range rows {
			if _, err := fmt.Fprintln(w, fmt.Sprint(r)); err != nil {
				return err
			}
		}
		return nil
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
