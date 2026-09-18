package generator

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// Param is a path parameter used by an operation.
type Param struct {
	Name   string
	Format string // OpenAPI schema format, e.g. "uuid"; empty when none
}

// BodyProp is one top-level property of an operation's JSON request body.
type BodyProp struct {
	Name     string
	Required bool
	Type     string // "string", "boolean", "integer", "number", ...
	Format   string // OpenAPI schema format, e.g. "uuid", "email", "date-time"
}

// Operation is a flattened, generator-friendly view of one OpenAPI operation.
type Operation struct {
	Tag         string
	OperationID string
	Method      string // upper-case, e.g. "GET"
	Path        string
	Params      []Param // path parameters, in path order
	HasParams   bool    // true if the operation declares any parameter at all (path, header, or query) - oapi-codegen only generates a <OperationID>Params argument when this is true
	HasBody     bool
	BodyProps   []BodyProp
}

// LoadOperations parses the OpenAPI document at path and returns every
// operation with a non-empty operationId, in a stable, deterministic order
// (by path, then method).
func LoadOperations(path string) ([]Operation, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}

	var ops []Operation
	for p, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				continue
			}
			o := Operation{
				Method: method,
				Path:   p,
			}
			if len(op.Tags) > 0 {
				o.Tag = op.Tags[0]
			}
			o.OperationID = op.OperationID

			o.HasParams = len(op.Parameters) > 0

			for _, ref := range op.Parameters {
				if ref.Value != nil && ref.Value.In == "path" {
					p := Param{Name: ref.Value.Name}
					if ref.Value.Schema != nil && ref.Value.Schema.Value != nil {
						p.Format = ref.Value.Schema.Value.Format
					}
					o.Params = append(o.Params, p)
				}
			}

			if op.RequestBody != nil && op.RequestBody.Value != nil {
				mt := op.RequestBody.Value.Content.Get("application/json")
				if mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
					o.HasBody = true
					required := map[string]bool{}
					for _, name := range mt.Schema.Value.Required {
						required[name] = true
					}
					for name, propRef := range mt.Schema.Value.Properties {
						bp := BodyProp{Name: name, Required: required[name]}
						if propRef.Value != nil {
							if propRef.Value.Type != nil && len(*propRef.Value.Type) > 0 {
								bp.Type = (*propRef.Value.Type)[0]
							}
							bp.Format = propRef.Value.Format
						}
						o.BodyProps = append(o.BodyProps, bp)
					}
					sort.Slice(o.BodyProps, func(i, j int) bool {
						return o.BodyProps[i].Name < o.BodyProps[j].Name
					})
				}
			}

			ops = append(ops, o)
		}
	}

	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	return ops, nil
}
