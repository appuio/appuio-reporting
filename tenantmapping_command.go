package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/jmoiron/sqlx"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/urfave/cli/v2"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/appuio/appuio-cloud-reporting/pkg/tenantmapping"
)

type tmapCommand struct {
	DatabaseURL      string
	PrometheusURL    string
	Begin            *time.Time
	RepeatUntil      *time.Time
	PromQueryTimeout time.Duration
	DryRun           bool

	AdditionalMetricSelector string

	ThanosAllowPartialResponses bool
	OrgId                       string
}

var tenantmappingCommandName = "tenantmapping"

func newTmapCommand() *cli.Command {
	command := &tmapCommand{}
	return &cli.Command{
		Name:   tenantmappingCommandName,
		Usage:  "Update the tenant mapping (source, target) in the database for a given time",
		Before: command.before,
		Action: command.execute,
		Flags: []cli.Flag{
			newDbURLFlag(&command.DatabaseURL),
			newPromURLFlag(&command.PrometheusURL),
			&cli.TimestampFlag{Name: "begin", Usage: fmt.Sprintf("Beginning timestamp of the report period in the form of RFC3339 (%s)", time.RFC3339),
				EnvVars: envVars("BEGIN"), Layout: time.RFC3339, Required: true, DefaultText: defaultTestForRequiredFlags},
			&cli.TimestampFlag{Name: "repeat-until", Usage: fmt.Sprintf("Repeat running the report until reaching this timestamp (%s)", time.RFC3339),
				EnvVars: envVars("REPEAT_UNTIL"), Layout: time.RFC3339, Required: false},
			&cli.DurationFlag{Name: "prom-query-timeout", Usage: "Timeout when querying prometheus (example: 1m)",
				EnvVars: envVars("PROM_QUERY_TIMEOUT"), Destination: &command.PromQueryTimeout, Required: false},
			&cli.BoolFlag{Name: "thanos-allow-partial-responses", Usage: "Allows partial responses from Thanos. Can be helpful when querying a Thanos cluster with lost data.",
				EnvVars: envVars("THANOS_ALLOW_PARTIAL_RESPONSES"), Destination: &command.ThanosAllowPartialResponses, Required: false, DefaultText: "false"},
			&cli.BoolFlag{Name: "dry-run", Usage: "Does not commit results if set.",
				EnvVars: envVars("DRY_RUN"), Destination: &command.DryRun, Required: false, DefaultText: "false"},
			&cli.StringFlag{Name: "additional-metric-selector", Usage: "Allows further specifying which metrics to choose. Example: --additional-metric-selector='namespace=\"testing\"'",
				EnvVars: envVars("ADDITIONAL_METRIC_SELECTOR"), Destination: &command.AdditionalMetricSelector, Required: false, DefaultText: "false"},
			&cli.StringFlag{Name: "org-id", Usage: "Sets the X-Scope-OrgID header to this value on requests to Prometheus", Value: "",
				EnvVars: envVars("ORG_ID"), Destination: &command.OrgId, Required: false, DefaultText: "empty"},
		},
	}
}

func (cmd *tmapCommand) before(context *cli.Context) error {
	cmd.Begin = context.Timestamp("begin")
	cmd.RepeatUntil = context.Timestamp("repeat-until")
	return LogMetadata(context)
}

func (cmd *tmapCommand) execute(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	log := AppLogger(ctx).WithName(tenantmappingCommandName)
	// We really need to fix the inane dance around the AppLogger which needs custom plumbing and can't be used from packages because of import cycles.
	ctx = logr.NewContext(ctx, log)

	promClient, err := newPrometheusAPIClient(cmd.PrometheusURL, cmd.ThanosAllowPartialResponses, cmd.OrgId)
	if err != nil {
		return fmt.Errorf("could not create prometheus client: %w", err)
	}

	log.V(1).Info("Opening database connection", "url", cmd.DatabaseURL)
	rdb, err := db.Openx(cmd.DatabaseURL)
	if err != nil {
		return fmt.Errorf("could not open database connection: %w", err)
	}
	defer rdb.Close()

	o := make([]tenantmapping.Option, 0)
	if cmd.PromQueryTimeout != 0 {
		o = append(o, tenantmapping.WithPrometheusQueryTimeout(cmd.PromQueryTimeout))
	}
	if cmd.AdditionalMetricSelector != "" {
		o = append(o, tenantmapping.WithMetricSelector(cmd.AdditionalMetricSelector))
	}

	if cmd.RepeatUntil == nil {
		return runTenantMapping(ctx, rdb, promClient, *cmd.Begin, cmd.DryRun, o...)
	}

	for currentTime := *cmd.Begin; cmd.RepeatUntil.After(currentTime); currentTime = currentTime.Add(time.Hour) {
		if err := runTenantMapping(ctx, rdb, promClient, currentTime, cmd.DryRun, o...); err != nil {
			return fmt.Errorf("error running report at %s: %w", currentTime.Format(time.RFC3339), err)
		}
	}

	return nil
}

func runTenantMapping(ctx context.Context, rdb *sqlx.DB, promClient apiv1.API, begin time.Time, dryRun bool, o ...tenantmapping.Option) error {
	log := AppLogger(ctx).WithName(tenantmappingCommandName)

	log.V(1).Info("Begin transaction")
	tx, err := rdb.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	log.Info("Running mapper...")
	err = tenantmapping.MapTenantTarget(ctx, tx, promClient, begin, o...)
	if err != nil {
		return err
	}

	if dryRun {
		log.Info("Dry run, not committing transaction")
		return nil
	}

	log.V(1).Info("Commit transaction")
	return tx.Commit()
}
