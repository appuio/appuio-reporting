package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/appuio/appuio-reporting/pkg/querycheck"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

type queryTestCommand struct {
	testFilePath string
	additionalYamlFiles cli.StringSlice
}

var queryTestCommandName = "test"

func newQueryTestCommand() *cli.Command {
	command := &queryTestCommand{}
	return &cli.Command{
		Name:   queryTestCommandName,
		Usage:  "Run Prometheus tests on a set of query test cases",
		Before: command.before,
		Action: command.execute,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "test-file", Usage: "Path of the jsonnet test file from which to test queries",
				EnvVars: envVars("TEST_FILE"), Destination: &command.testFilePath, Value: "./test.jsonnet"},
			&cli.StringSliceFlag{Name: "add-yaml-file", Usage: "Additional yaml files to include into the test (available to jsonnet as extVar)",
				EnvVars: envVars("ADD_YAML_FILE"), Destination: &command.additionalYamlFiles},
		},
	}
}

func (cmd *queryTestCommand) before(context *cli.Context) error {
	fmt.Println("begin!")
	return nil
}

func (cmd *queryTestCommand) execute(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	log := AppLogger(ctx).WithName(queryTestCommandName)

	extVars, err := cmd.buildExtVars()
	if err != nil {
		log.Error(err, "Query test setup failed")
	}
	err = querycheck.RunTestQueries(cmd.testFilePath, extVars)

	if err != nil {
		log.Error(err, "Query test failed")
	}
	log.Info("Done")
	return err 
}

func (cmd *queryTestCommand) buildExtVars() (*map[string]string, error) {
	extVars := make(map[string]string)
	for file := range(cmd.additionalYamlFiles.Value()) {
		path := cmd.additionalYamlFiles.Value()[file]
		name := filepath.Base(path)
		fileHandle, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fileHandle.Close()
		fileContent, err := io.ReadAll(fileHandle)
		if err != nil {
			return nil, err
		}

		var parsed map[string]interface{}
		err = yaml.Unmarshal(fileContent, &parsed)
		if err != nil {
			return nil, err
		}
		jsonStr, err := json.Marshal(parsed)
		if err != nil {
			return nil, err
		}
		extVars[name] = string(jsonStr)
	}
	return &extVars, nil
}
