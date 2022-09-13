package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/appuio/appuio-cloud-reporting/pkg/report"
	"github.com/appuio/appuio-cloud-reporting/pkg/thanos"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/urfave/cli/v2"
)

type reportCommand struct {
	DatabaseURL      string
	PrometheusURL    string
	QueryName        string
	Begin            *time.Time
	RepeatUntil      *time.Time
	PromQueryTimeout time.Duration

	ThanosAllowPartialResponses bool
}

var reportCommandName = "report"

func newReportCommand() *cli.Command {
	command := &reportCommand{}
	return &cli.Command{
		Name:   reportCommandName,
		Usage:  "Run a report for a query in the given period",
		Before: command.before,
		Action: command.execute,
		Flags: []cli.Flag{
			newDbURLFlag(&command.DatabaseURL),
			newPromURLFlag(&command.PrometheusURL),
			&cli.StringFlag{Name: "query-name", Usage: fmt.Sprintf("Name of the query (sample values: %s)", queryNames(db.DefaultQueries)),
				EnvVars: envVars("QUERY_NAME"), Destination: &command.QueryName, Required: true, DefaultText: defaultTestForRequiredFlags},
			&cli.TimestampFlag{Name: "begin", Usage: fmt.Sprintf("Beginning timestamp of the report period in the form of RFC3339 (%s)", time.RFC3339),
				EnvVars: envVars("BEGIN"), Layout: time.RFC3339, Required: true, DefaultText: defaultTestForRequiredFlags},
			&cli.TimestampFlag{Name: "repeat-until", Usage: fmt.Sprintf("Repeat running the report until reaching this timestamp (%s)", time.RFC3339),
				EnvVars: envVars("REPEAT_UNTIL"), Layout: time.RFC3339, Required: false},
			&cli.DurationFlag{Name: "prom-query-timeout", Usage: "Timeout when querying prometheus (example: 1m)",
				EnvVars: envVars("PROM_QUERY_TIMEOUT"), Destination: &command.PromQueryTimeout, Required: false},
			&cli.BoolFlag{Name: "thanos-allow-partial-responses", Usage: "Allows partial responses from Thanos. Can be helpful when querying a Thanos cluster with lost data.",
				EnvVars: envVars("THANOS_ALLOW_PARTIAL_RESPONSES"), Destination: &command.ThanosAllowPartialResponses, Required: false, DefaultText: "false"},
		},
	}
}

func (cmd *reportCommand) before(context *cli.Context) error {
	cmd.Begin = context.Timestamp("begin")
	cmd.RepeatUntil = context.Timestamp("repeat-until")
	return LogMetadata(context)
}

func (cmd *reportCommand) execute(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	log := AppLogger(ctx).WithName(reportCommandName)

	promClient, err := newPrometheusAPIClient(cmd.PrometheusURL, cmd.ThanosAllowPartialResponses)
	if err != nil {
		return fmt.Errorf("could not create prometheus client: %w", err)
	}

	log.V(1).Info("Opening database connection", "url", cmd.DatabaseURL)
	rdb, err := db.Openx(cmd.DatabaseURL)
	if err != nil {
		return fmt.Errorf("could not open database connection: %w", err)
	}
	defer rdb.Close()

	o := make([]report.Option, 0)
	if cmd.PromQueryTimeout != 0 {
		o = append(o, report.WithPrometheusQueryTimeout(cmd.PromQueryTimeout))
	}

	if cmd.RepeatUntil != nil {
		if err := cmd.runReportRange(ctx, rdb, promClient, o); err != nil {
			return err
		}
	} else {
		if err := cmd.runReport(ctx, rdb, promClient, o); err != nil {
			return err
		}
	}

	log.Info("Done")
	return nil
}

func (cmd *reportCommand) runReportRange(ctx context.Context, db *sqlx.DB, promClient apiv1.API, o []report.Option) error {
	log := AppLogger(ctx)

	started := time.Now()
	reporter := report.WithProgressReporter(func(p report.Progress) {
		fmt.Fprintf(os.Stderr, "Report %d, Current: %s [%s]\n",
			p.Count, p.Timestamp.Format(time.RFC3339), time.Since(started).Round(time.Second),
		)
	})

	log.Info("Running reports...")
	c, err := report.RunRange(ctx, db, promClient, cmd.QueryName, *cmd.Begin, *cmd.RepeatUntil, append(o, reporter)...)
	log.Info(fmt.Sprintf("Ran %d reports", c))
	return err
}

func (cmd *reportCommand) runReport(ctx context.Context, db *sqlx.DB, promClient apiv1.API, o []report.Option) error {
	log := AppLogger(ctx)

	log.V(1).Info("Begin transaction")
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	log.Info("Running report...")
	if err := report.Run(ctx, tx, promClient, cmd.QueryName, *cmd.Begin, o...); err != nil {
		return err
	}

	log.V(1).Info("Commit transaction")
	return tx.Commit()
}

func newPrometheusAPIClient(promURL string, thanosAllowPartialResponses bool) (apiv1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: promURL,
		RoundTripper: &thanos.PartialResponseRoundTripper{
			RoundTripper: api.DefaultRoundTripper,
			Allow:        thanosAllowPartialResponses,
		},
	})
	return apiv1.NewAPI(client), err
}
