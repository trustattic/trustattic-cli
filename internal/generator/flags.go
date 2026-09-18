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

// ValueKind describes how a flag's raw string/bool/int value must be
// converted to build the real typed request oapi-codegen generates.
// Path parameters and body properties are always entered as flat scalars on
// the CLI, but the real generated client sometimes expects a distinct Go
// type for a given OpenAPI "format" - a UUID-formatted string becomes
// uuid.UUID, an email-formatted string becomes openapi_types.Email, and a
// date-time-formatted string becomes time.Time. ValuePlain covers every
// other case, where the raw scalar is used (or pointed to) as-is.
type ValueKind int

const (
	ValuePlain ValueKind = iota
	ValueUUID
	ValueEmail
	ValueDateTime
)

// valueKindFor resolves an OpenAPI schema "format" into the ValueKind the
// real generated client needs. Only string-typed values carry a meaningful
// non-plain format in the current spec.
func valueKindFor(format string) ValueKind {
	switch format {
	case "uuid":
		return ValueUUID
	case "email":
		return ValueEmail
	case "date-time":
		return ValueDateTime
	default:
		return ValuePlain
	}
}

// isFlatScalar reports whether an OpenAPI schema type can be represented as
// a single Cobra flag value. Per the design spec's "no nested-object escape
// hatch" decision, object/array-typed body properties (and properties whose
// type can't be determined directly, e.g. a oneOf/anyOf union) have no flat
// CLI representation and are simply not exposed as flags.
func isFlatScalar(openAPIType string) bool {
	switch openAPIType {
	case "object", "array", "":
		return false
	default:
		return true
	}
}

// FlagDef is one Cobra flag or positional argument, template-ready.
type FlagDef struct {
	Name    string // CLI flag name, kebab-case
	GoIdent string // Go variable name
	// Help is the flag's usage string: the spec's description for the
	// underlying path parameter or body property, falling back to the flag
	// name when the spec declares none.
	Help     string
	Kind     BodyFlagKind
	Value    ValueKind // how to convert the raw scalar into the real request type
	Required bool
	Optional bool // true only for the project flag
}

// helpFor returns the usage text for a flag: the spec-supplied description
// when there is one, else the flag name itself (which is at least no worse
// than the placeholder text the generator emitted before descriptions were
// captured).
func helpFor(description, flagName string) string {
	if strings.TrimSpace(description) != "" {
		return description
	}
	return flagName
}

// UnsupportedBodyProp is a request-body property that isFlatScalar filtered
// out of BodyFlags - it has no flat CLI representation (an object/array/
// oneOf shape). GeneratedCommand carries two lists of these, split by
// whether the property is required:
//
//   - required (UnsupportedRequiredBodyProps): silently omitting it would
//     send an incomplete request the server can reject (or worse, silently
//     accept with the field missing/zero-valued), so the generated command
//     fails fast with a clear error instead of running at all.
//   - optional (DroppedOptionalBodyProps): the request is still valid
//     without it, so the command runs - but the field can never be set from
//     the CLI, which is worth saying out loud in --help rather than leaving
//     as an invisible gap.
type UnsupportedBodyProp struct {
	Name string
	Type string // OpenAPI schema type; "" for a oneOf/anyOf union with no direct type
}

// GeneratedCommand is a fully-resolved, template-ready description of one
// generated Cobra command.
type GeneratedCommand struct {
	Spec           CommandSpec
	PositionalFlag *FlagDef
	Flags          []FlagDef // path-param flags, excluding the positional one
	BodyFlags      []FlagDef // request body property flags

	// UnsupportedRequiredBodyProps lists every required body property this
	// command's operation declares that isFlatScalar excluded from
	// BodyFlags. Non-empty here means the command cannot be correctly
	// implemented via flat CLI flags yet - see emit.go, which renders such
	// a command's RunE as an immediate, descriptive error instead of a real
	// (silently incomplete) request.
	UnsupportedRequiredBodyProps []UnsupportedBodyProp

	// DroppedOptionalBodyProps lists every *optional* body property this
	// command's operation declares that isFlatScalar excluded from
	// BodyFlags. These don't block the command - the request is valid
	// without them - but they can never be supplied from the CLI, so
	// emit.go names them in the command's Long help instead of leaving the
	// omission invisible (see the design spec's "no nested-object escape
	// hatch" decision).
	DroppedOptionalBodyProps []UnsupportedBodyProp
}

// BuildGeneratedCommands resolves every CommandSpec's path parameters and
// body properties into template-ready flag definitions.
func BuildGeneratedCommands(specs []CommandSpec) []GeneratedCommand {
	out := make([]GeneratedCommand, len(specs))
	for i, s := range specs {
		gc := GeneratedCommand{Spec: s}

		if s.Positional != nil {
			name := FlagName(s.Positional.Name)
			gc.PositionalFlag = &FlagDef{
				Name:    name,
				GoIdent: GoIdent(name),
				Help:    helpFor(s.Positional.Description, name),
				Kind:    KindString,
				Value:   valueKindFor(s.Positional.Format),
			}
		}

		for _, p := range s.Flags {
			name := FlagName(p.Name)
			gc.Flags = append(gc.Flags, FlagDef{
				Name:     name,
				GoIdent:  GoIdent(name),
				Help:     helpFor(p.Description, name),
				Kind:     KindString,
				Value:    valueKindFor(p.Format),
				Required: !IsProjectFlag(p),
				Optional: IsProjectFlag(p),
			})
		}

		for _, bp := range s.Operation.BodyProps {
			if !isFlatScalar(bp.Type) {
				// No flat CLI representation for an object/array/union body
				// property - it's simply not exposed as a flag. Record it
				// either way so the omission is never silent: a *required*
				// one makes emit.go fail the command fast instead of
				// sending an incomplete request, and an *optional* one is
				// named in the command's Long help so users can see the
				// field exists but isn't settable here.
				prop := UnsupportedBodyProp{Name: bp.Name, Type: bp.Type}
				if bp.Required {
					gc.UnsupportedRequiredBodyProps = append(gc.UnsupportedRequiredBodyProps, prop)
				} else {
					gc.DroppedOptionalBodyProps = append(gc.DroppedOptionalBodyProps, prop)
				}
				continue
			}
			name := strings.ReplaceAll(bp.Name, "_", "-")
			gc.BodyFlags = append(gc.BodyFlags, FlagDef{
				Name:     name,
				GoIdent:  GoIdent(name),
				Help:     helpFor(bp.Description, name),
				Kind:     FlagKindForType(bp.Type),
				Value:    valueKindFor(bp.Format),
				Required: bp.Required,
			})
		}

		out[i] = gc
	}
	return out
}
