// cmd/gen/main.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"

	"github.com/trustattic/trustattic-cli/internal/generator"
)

func main() {
	specPath := flag.String("spec", "spec/api.yaml", "path to the OpenAPI spec")
	outDir := flag.String("out", "internal/commands/generated", "output directory")
	overridesPath := flag.String("overrides", "internal/generator/overrides.yaml", "path to the name-override table")
	flag.Parse()

	if err := run(*specPath, *outDir, *overridesPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(specPath, outDir, overridesPath string) error {
	ops, err := generator.LoadOperations(specPath)
	if err != nil {
		return fmt.Errorf("load operations: %w", err)
	}
	specs := generator.BuildCommandSpecs(ops)

	overrides, err := generator.LoadOverrides(overridesPath)
	if err != nil {
		return fmt.Errorf("load overrides: %w", err)
	}
	specs = generator.ApplyOverrides(specs, overrides)

	cmds := generator.BuildGeneratedCommands(specs)
	groups := generator.GroupByTag(cmds)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for tag, tagCmds := range groups {
		// BuildCommandSpecs (Task 11) empties Spec.Tag for what was the
		// OpenAPI "common" tag; give that group's file a real name instead
		// of writing to "<outDir>/.gen.go".
		fileTag := tag
		if fileTag == "" {
			fileTag = "common"
		}
		path := filepath.Join(outDir, fileTag+".gen.go")
		if err := writeFormatted(path, func(w io.Writer) error {
			return generator.EmitTagFile(w, tagCmds)
		}); err != nil {
			return fmt.Errorf("emit %s: %w", path, err)
		}
	}

	registerPath := filepath.Join(outDir, "register.gen.go")
	if err := writeFormatted(registerPath, func(w io.Writer) error {
		return generator.EmitRegisterFile(w, cmds)
	}); err != nil {
		return fmt.Errorf("emit %s: %w", registerPath, err)
	}

	return nil
}

func writeFormatted(path string, render func(w io.Writer) error) error {
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		return err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt: %w\n--- unformatted source ---\n%s", err, buf.String())
	}
	return os.WriteFile(path, formatted, 0o644)
}
