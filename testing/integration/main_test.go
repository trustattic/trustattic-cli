//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/trustattic/trustattic-cli/testing/integration"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "trustattic-cli-integration")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	bin := filepath.Join(tmpDir, "trustattic")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/trustattic")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build trustattic binary: " + err.Error() + "\n" + string(out))
	}
	integration.BinaryPath = bin

	os.Exit(m.Run())
}
