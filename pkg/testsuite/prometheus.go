package testsuite

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var (
	_, b, _, _ = runtime.Caller(0)

	// Root folder of this project
	Root = filepath.Join(filepath.Dir(b), "..", "..")
	// PromDir is the filepath to the prometheus directory
	PromDir = filepath.Join(Root, ".cache", "prometheus")
	// PromBin is the filepath to the Prometheus binary
	PromBin = filepath.Join(PromDir, "prometheus")
	// PromtoolBin is the filepath to the promtool binary
	PromtoolBin = filepath.Join(PromDir, "promtool")
)

// StartPrometheus starts a new prometheus instance on the given port.
// Cancel the context to stop.
// The returned cleanup function block until prometheus is stopped. Cleanup has to be called.
func StartPrometheus(ctx context.Context, port int) (cleanup func() error, err error) {
	tmpDir, err := os.MkdirTemp("", "prom-data-*")
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, PromBin,
		fmt.Sprintf("--web.listen-address=127.0.0.1:%d", port),
		fmt.Sprintf("--storage.tsdb.path=%s", tmpDir),
	)
	cmd.Dir = filepath.Dir(PromBin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return func() error {
		defer os.RemoveAll(tmpDir)
		return cmd.Wait()
	}, cmd.Start()
}
