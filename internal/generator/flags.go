// internal/generator/flags.go
package generator

import "strings"

// FlagName derives a CLI flag name from a snake_case path parameter or body
// property name: underscores become hyphens, and a trailing "_slug" is
// dropped (project_slug -> "project", not "project-slug") since the
// resource name alone is unambiguous. "_id" is kept as "-id" since bare
// resource names would be ambiguous there (e.g. "account" could mean many
// things; "account-id" cannot).
func FlagName(param string) string {
	name := strings.ReplaceAll(param, "_", "-")
	return strings.TrimSuffix(name, "-slug")
}

// GoIdent turns a kebab-case flag name into a camelCase Go identifier,
// special-casing a trailing "id" segment to "ID" (Go style), e.g.
// "account-id" -> "accountID".
func GoIdent(name string) string {
	parts := strings.Split(name, "-")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if p == "id" {
			b.WriteString("ID")
			continue
		}
		if i == 0 {
			b.WriteString(strings.ToLower(p[:1]) + p[1:])
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// BodyFlagKind is the Cobra flag kind used for a request body property.
type BodyFlagKind string

const (
	KindString BodyFlagKind = "string"
	KindBool   BodyFlagKind = "bool"
	KindInt    BodyFlagKind = "int"
)

// FlagKindForType maps an OpenAPI schema type to a Cobra flag kind. Every
// request body in the current spec is flat (see the design spec's "no
// nested-object escape hatch" decision), so object/array/unknown types fall
// back to KindString - the flag simply carries the raw scalar as text.
func FlagKindForType(openAPIType string) BodyFlagKind {
	switch openAPIType {
	case "boolean":
		return KindBool
	case "integer":
		return KindInt
	default:
		return KindString
	}
}

// IsProjectFlag reports whether p is the project_slug path parameter - the
// one path parameter that gets an optional/current-project fallback instead
// of being required. See the design spec's "Current project" section.
func IsProjectFlag(p Param) bool {
	return p.Name == "project_slug"
}

// FlagDef is one Cobra flag or positional argument, template-ready.
type FlagDef struct {
	Name     string // CLI flag name, kebab-case
	GoIdent  string // Go variable name
	Kind     BodyFlagKind
	Required bool
	Optional bool // true only for the project flag
}

// GeneratedCommand is a fully-resolved, template-ready description of one
// generated Cobra command.
type GeneratedCommand struct {
	Spec           CommandSpec
	PositionalFlag *FlagDef
	Flags          []FlagDef // path-param flags, excluding the positional one
	BodyFlags      []FlagDef // request body property flags
}

// BuildGeneratedCommands resolves every CommandSpec's path parameters and
// body properties into template-ready flag definitions.
func BuildGeneratedCommands(specs []CommandSpec) []GeneratedCommand {
	out := make([]GeneratedCommand, len(specs))
	for i, s := range specs {
		gc := GeneratedCommand{Spec: s}

		if s.Positional != nil {
			name := FlagName(s.Positional.Name)
			gc.PositionalFlag = &FlagDef{Name: name, GoIdent: GoIdent(name), Kind: KindString}
		}

		for _, p := range s.Flags {
			name := FlagName(p.Name)
			gc.Flags = append(gc.Flags, FlagDef{
				Name:     name,
				GoIdent:  GoIdent(name),
				Kind:     KindString,
				Required: !IsProjectFlag(p),
				Optional: IsProjectFlag(p),
			})
		}

		for _, bp := range s.Operation.BodyProps {
			name := strings.ReplaceAll(bp.Name, "_", "-")
			gc.BodyFlags = append(gc.BodyFlags, FlagDef{
				Name:     name,
				GoIdent:  GoIdent(name),
				Kind:     FlagKindForType(bp.Type),
				Required: bp.Required,
			})
		}

		out[i] = gc
	}
	return out
}
