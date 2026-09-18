// internal/generator/naming.go
package generator

import (
	"strings"
	"unicode"
)

// CommandSpec is the fully-resolved shape of one generated command: which
// top-level tag it hangs off, an optional nested group, its leaf verb, and
// how its path parameters split into a trailing positional argument plus
// leading flags.
type CommandSpec struct {
	Tag        string
	Group      []string // kebab-case words, nested subcommand path; empty for a direct leaf
	Verb       string   // kebab-case leaf command name
	Positional *Param   // the last path parameter, if any
	Flags      []Param  // every other path parameter, in path order
	Operation  Operation
}

var httpMethodWords = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true,
}

// splitPascal splits a PascalCase identifier into its words, e.g.
// "ScheduleGetHistory" -> ["Schedule", "Get", "History"].
func splitPascal(s string) []string {
	var words []string
	var cur []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			words = append(words, string(cur))
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return words
}

// paramTokens splits a snake_case path parameter name into title-cased
// tokens, e.g. "project_slug" -> ["Project", "Slug"].
func paramTokens(name string) []string {
	parts := strings.Split(name, "_")
	tokens := make([]string, len(parts))
	for i, p := range parts {
		if p == "" {
			continue
		}
		tokens[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return tokens
}

// remainderWords computes the operationId words left over after stripping
// the leading tag word, any HTTP-method words, and any word that is just a
// path parameter name leaking into the operationId (see ProjectSlugGet in
// the design spec).
//
// The "leaking param name" case is specifically a param whose name is
// "<tag>_<suffix>" (e.g. project_slug on the "project" tag): its first token
// duplicates the tag word (already stripped above) and its remaining
// token(s) are what's left over as a spurious word in the operationId. Only
// those suffix tokens are stripped - not every token of every param the
// operation happens to use - so an unrelated param whose name coincidentally
// shares a word with a real action word (e.g. restore_id on a "restore"
// action) doesn't get wrongly treated as a naming artifact.
func remainderWords(op Operation) []string {
	paramTok := map[string]bool{}
	for _, p := range op.Params {
		toks := paramTokens(p.Name)
		if len(toks) > 1 && strings.EqualFold(toks[0], op.Tag) {
			for _, t := range toks[1:] {
				paramTok[strings.ToLower(t)] = true
			}
		}
	}

	words := splitPascal(op.OperationID)
	var out []string
	for i, w := range words {
		if i == 0 && strings.EqualFold(w, op.Tag) {
			continue
		}
		if httpMethodWords[w] {
			continue
		}
		if paramTok[strings.ToLower(w)] {
			continue
		}
		out = append(out, w)
	}
	return out
}

func kebab(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = strings.ToLower(w)
	}
	return out
}

// endsInPathParam reports whether op's path's final segment is the last
// entry of op.Params (a {param} segment), as opposed to a static segment.
func endsInPathParam(op Operation) bool {
	if len(op.Params) == 0 {
		return false
	}
	last := op.Params[len(op.Params)-1]
	return strings.HasSuffix(op.Path, "{"+last.Name+"}")
}

// mechanicalVerb derives a verb purely from the HTTP method and whether the
// path ends in a path parameter, with no knowledge of what the resource is.
func mechanicalVerb(op Operation) string {
	switch op.Method {
	case "GET":
		if endsInPathParam(op) {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		if endsInPathParam(op) {
			return "update"
		}
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(op.Method)
	}
}

// BuildCommandSpecs converts every operation into a CommandSpec, in the same
// order as ops.
func BuildCommandSpecs(ops []Operation) []CommandSpec {
	type groupKey struct {
		tag       string
		remainder string
	}
	groupOf := make([]groupKey, len(ops))
	sizes := map[groupKey]int{}

	for i, op := range ops {
		k := groupKey{tag: op.Tag, remainder: strings.Join(remainderWords(op), "\x00")}
		groupOf[i] = k
		sizes[k]++
	}

	specs := make([]CommandSpec, len(ops))
	for i, op := range ops {
		remainder := remainderWords(op)
		size := sizes[groupOf[i]]

		// The "common" tag isn't a real resource - its operations
		// (healthcheck, permissions, external-types) are promoted to
		// ungrouped top-level commands, so they carry no tag at all.
		tag := op.Tag
		if strings.EqualFold(tag, "common") {
			tag = ""
		}
		spec := CommandSpec{Tag: tag, Operation: op}
		switch {
		case size == 1 && len(remainder) > 0:
			spec.Verb = strings.Join(kebab(remainder), "-")
		case size == 1:
			spec.Verb = mechanicalVerb(op)
		default:
			spec.Group = kebab(remainder)
			spec.Verb = mechanicalVerb(op)
		}

		// A path parameter is positional only when the operation's URL
		// literally ends in it (classic get/update/delete-by-id shape).
		// Every other path parameter - including one on a URL whose final
		// segment is a static action word like "restore" or "check" - is a
		// flag. This is a strict URL-shape check, not a judgment call about
		// whether the operation "feels like" it targets that parameter.
		if n := len(op.Params); n > 0 {
			if endsInPathParam(op) {
				p := op.Params[n-1]
				spec.Positional = &p
				spec.Flags = append(spec.Flags, op.Params[:n-1]...)
			} else {
				spec.Flags = append(spec.Flags, op.Params...)
			}
		}

		specs[i] = spec
	}
	return specs
}
