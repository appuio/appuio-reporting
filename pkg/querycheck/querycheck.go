package querycheck

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/go-jsonnet"

	"github.com/appuio/appuio-reporting/pkg/testsuite"
)

func RunTestQueries(filepath string, extcodes *map[string]string) error {
	tmp, err := renderJsonnet(filepath, extcodes)
	if err != nil {
		return err
	}
	return runPromtool(tmp)
}

func runPromtool(tmp string) error {
	cmd := exec.Command(testsuite.PromtoolBin, "test", "rules", tmp)
	var stderr, stdout strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	// Not using t.Log to keep formatting sane
	fmt.Println("STDOUT")
	fmt.Println(stdout.String())
	fmt.Println("STDERR")
	fmt.Println(stderr.String())
	return err
}

func renderJsonnet(tFile string, extcodes *map[string]string) (string, error) {
	vm := jsonnet.MakeVM()

	for key := range(*extcodes) {
		vm.ExtCode(key, (*extcodes)[key])
	}

	ev, err := vm.EvaluateFile(tFile)
	if err != nil {
		return "", err
	}

	tmp := path.Join("/tmp", "test.json")
	err = os.WriteFile(tmp, []byte(ev), 0644)
	return tmp, err
}
