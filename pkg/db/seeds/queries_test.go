package seeds_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/appuio/appuio-cloud-reporting/pkg/testsuite"
)

func TestQueries(t *testing.T) {
	wd := os.DirFS(".")
	testFiles, err := fs.Glob(wd, "promtest/*.jsonnet")
	require.NoError(t, err)

	for _, tFile := range testFiles {
		t.Run(tFile, func(t *testing.T) {
			tmp := renderJsonnet(t, tFile)
			runPromtool(t, tmp)
		})
	}
}

func runPromtool(t *testing.T, tmp string) {
	t.Helper()

	cmd := exec.Command(testsuite.PromtoolBin, "test", "rules", tmp)
	var stderr, stdout strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	assert.NoError(t, cmd.Run())
	// Not using t.Log to keep formatting sane
	fmt.Println("STDOUT")
	fmt.Println(stdout.String())
	fmt.Println("STDERR")
	fmt.Println(stderr.String())
}

func renderJsonnet(t *testing.T, tFile string) string {
	t.Helper()

	ev, err := jsonnet.MakeVM().EvaluateFile(tFile)
	require.NoError(t, err)
	tmp := path.Join(t.TempDir(), "test.json")
	require.NoError(t, os.WriteFile(tmp, []byte(ev), 0644))
	return tmp
}
