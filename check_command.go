package main

import (
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/appuio/appuio-cloud-reporting/pkg/check"
	"github.com/appuio/appuio-cloud-reporting/pkg/db"
)

type checkMissingCommand struct {
	DatabaseURL string
}

var checkMissingCommandName = "check_missing"

func newCheckMissingCommand() *cli.Command {
	command := &checkMissingCommand{}
	return &cli.Command{
		Name:   checkMissingCommandName,
		Usage:  "Check for missing data in the database",
		Before: command.before,
		Action: command.execute,
		Flags: []cli.Flag{
			newDbURLFlag(&command.DatabaseURL),
		},
	}
}

func (cmd *checkMissingCommand) before(context *cli.Context) error {
	return LogMetadata(context)
}

func (cmd *checkMissingCommand) execute(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	log := AppLogger(ctx).WithName(migrateCommandName)

	log.V(1).Info("Opening database connection", "url", cmd.DatabaseURL)
	rdb, err := db.Openx(cmd.DatabaseURL)
	if err != nil {
		return fmt.Errorf("could not open database connection: %w", err)
	}

	log.V(1).Info("Begin transaction")
	tx, err := rdb.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	missing, err := check.Missing(ctx, tx)
	if err != nil {
		return err
	}
	inconsistent, err := check.Inconsistent(ctx, tx)
	if err != nil {
		return err
	}

	if len(missing) != 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		defer w.Flush()
		fmt.Fprint(w, "Table\tMissing Field\tID\tSource\n")
		for _, m := range missing {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Table, m.MissingField, m.ID, m.Source)
		}
	}

	if len(inconsistent) != 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		defer w.Flush()
		fmt.Fprint(w, "Table\tDimension ID\tFact Time\tDimension Time Range\n")
		for _, i := range inconsistent {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Table, i.DimensionID, i.FactTime, i.DimensionRange)
		}
	}

	if len(missing) == 0 && len(inconsistent) == 0 {
		return cli.Exit("No missing or inconsistent data found.", 0)
	}

	return cli.Exit(fmt.Sprintf("%d missing, %d inconsistent entries found.", len(missing), len(inconsistent)), 1)
}
