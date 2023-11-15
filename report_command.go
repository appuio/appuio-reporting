package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/odoo"
	"github.com/appuio/appuio-cloud-reporting/pkg/report"
	"github.com/appuio/appuio-cloud-reporting/pkg/thanos"
	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/urfave/cli/v2"
)

type reportCommand struct {
	PrometheusURL     string
	OdooURL           string
	OdooOauthTokenURL string
	OdooClientId      string
	OdooClientSecret  string

	ReportArgs report.ReportArgs

	Begin       *time.Time
	RepeatUntil *time.Time

	PromQueryTimeout            time.Duration
	ThanosAllowPartialResponses bool
	OrgId                       string
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
			&cli.StringFlag{Name: "prom-url", Usage: "Prometheus connection URL in the form of http://host:port",
				EnvVars: envVars("PROM_URL"), Destination: &command.PrometheusURL, Value: "http://localhost:9090"},
			&cli.StringFlag{Name: "odoo-url", Usage: "URL of the Odoo Metered Billing API",
				EnvVars: envVars("ODOO_URL"), Destination: &command.OdooURL, Value: "http://localhost:8080"},
			&cli.StringFlag{Name: "odoo-oauth-token-url", Usage: "Oauth Token URL to authenticate with Odoo metered billing API",
				EnvVars: envVars("ODOO_OAUTH_TOKEN_URL"), Destination: &command.OdooOauthTokenURL, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.StringFlag{Name: "odoo-oauth-client-id", Usage: "Client ID of the oauth client to interact with Odoo metered billing API",
				EnvVars: envVars("ODOO_OAUTH_CLIENT_ID"), Destination: &command.OdooClientId, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.StringFlag{Name: "odoo-oauth-client-secret", Usage: "Client secret of the oauth client to interact with Odoo metered billing API",
				EnvVars: envVars("ODOO_OAUTH_CLIENT_SECRET"), Destination: &command.OdooClientSecret, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.StringFlag{Name: "product-id", Usage: fmt.Sprintf("Odoo Product ID for this query"),
				EnvVars: envVars("PRODUCT_ID"), Destination: &command.ReportArgs.ProductID, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.StringFlag{Name: "query", Usage: fmt.Sprintf("Prometheus query to run"),
				EnvVars: envVars("QUERY"), Destination: &command.ReportArgs.Query, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.StringFlag{Name: "instance-jsonnet", Usage: fmt.Sprintf("Jsonnet snippet that generates the Instance ID"),
				EnvVars: envVars("INSTANCE_JSONNET"), Destination: &command.ReportArgs.InstanceJsonnet, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.StringFlag{Name: "item-group-description-jsonnet", Usage: fmt.Sprintf("Jsonnet snippet that generates the item group description on invoice"),
				EnvVars: envVars("ITEM_GROUP_DESCRIPTION_JSONNET"), Destination: &command.ReportArgs.ItemGroupDescriptionJsonnet, Required: false, DefaultText: defaultTextForOptionalFlags},
			&cli.StringFlag{Name: "item-description-jsonnet", Usage: fmt.Sprintf("Jsonnet snippet that generates the item description on invoice"),
				EnvVars: envVars("ITEM_DESCRIPTION_JSONNET"), Destination: &command.ReportArgs.ItemDescriptionJsonnet, Required: false, DefaultText: defaultTextForOptionalFlags},
			&cli.StringFlag{Name: "unit-id", Usage: fmt.Sprintf("ID of the unit to use in Odoo"),
				EnvVars: envVars("UNIT_ID"), Destination: &command.ReportArgs.UnitID, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.TimestampFlag{Name: "begin", Usage: fmt.Sprintf("Beginning timestamp of the report period in the form of RFC3339 (%s)", time.RFC3339),
				EnvVars: envVars("BEGIN"), Layout: time.RFC3339, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.DurationFlag{Name: "timerange", Usage: "Timerange for individual measurement samples",
				EnvVars: envVars("TIMERANGE"), Destination: &command.ReportArgs.TimerangeSize, Required: true, DefaultText: defaultTextForRequiredFlags},
			&cli.TimestampFlag{Name: "repeat-until", Usage: fmt.Sprintf("Repeat running the report until reaching this timestamp (%s)", time.RFC3339),
				EnvVars: envVars("REPEAT_UNTIL"), Layout: time.RFC3339, Required: false},
			&cli.DurationFlag{Name: "prom-query-timeout", Usage: "Timeout when querying prometheus (example: 1m)",
				EnvVars: envVars("PROM_QUERY_TIMEOUT"), Destination: &command.PromQueryTimeout, Required: false},
			&cli.BoolFlag{Name: "thanos-allow-partial-responses", Usage: "Allows partial responses from Thanos. Can be helpful when querying a Thanos cluster with lost data.",
				EnvVars: envVars("THANOS_ALLOW_PARTIAL_RESPONSES"), Destination: &command.ThanosAllowPartialResponses, Required: false, DefaultText: "false"},
			&cli.StringFlag{Name: "org-id", Usage: "Sets the X-Scope-OrgID header to this value on requests to Prometheus", Value: "",
				EnvVars: envVars("ORG_ID"), Destination: &command.OrgId, Required: false, DefaultText: "empty"},
			&cli.StringFlag{Name: "debug-override-sales-order-id", Usage: "Overrides the sales order ID to a static constant for debugging purposes", Value: "",
				EnvVars: envVars("DEBUG_OVERRIDE_SALES_ORDER_ID"), Destination: &command.ReportArgs.OverrideSalesOrderID, Required: false, DefaultText: "empty"},
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

	promClient, err := newPrometheusAPIClient(cmd.PrometheusURL, cmd.ThanosAllowPartialResponses, cmd.OrgId)
	if err != nil {
		return fmt.Errorf("could not create prometheus client: %w", err)
	}

	odooClient := odoo.NewOdooAPIClient(ctx, cmd.OdooURL, cmd.OdooOauthTokenURL, cmd.OdooClientId, cmd.OdooClientSecret, log)

	o := make([]report.Option, 0)
	if cmd.PromQueryTimeout != 0 {
		o = append(o, report.WithPrometheusQueryTimeout(cmd.PromQueryTimeout))
	}

	if cmd.RepeatUntil != nil {
		if err := cmd.runReportRange(ctx, odooClient, promClient, o); err != nil {
			return err
		}
	} else {
		if err := cmd.runReport(ctx, odooClient, promClient, o); err != nil {
			return err
		}
	}

	log.Info("Done")
	return nil
}

func (cmd *reportCommand) runReportRange(ctx context.Context, odooClient *odoo.OdooAPIClient, promClient apiv1.API, o []report.Option) error {
	log := AppLogger(ctx)

	started := time.Now()
	reporter := report.WithProgressReporter(func(p report.Progress) {
		fmt.Fprintf(os.Stderr, "Report %d, Current: %s [%s]\n",
			p.Count, p.Timestamp.Format(time.RFC3339), time.Since(started).Round(time.Second),
		)
	})

	log.Info("Running reports...")
	c, err := report.RunRange(ctx, odooClient, promClient, cmd.ReportArgs, *cmd.Begin, *cmd.RepeatUntil, append(o, reporter)...)
	log.Info(fmt.Sprintf("Ran %d reports", c))
	return err
}

func (cmd *reportCommand) runReport(ctx context.Context, odooClient *odoo.OdooAPIClient, promClient apiv1.API, o []report.Option) error {
	log := AppLogger(ctx)

	log.V(1).Info("Begin transaction")

	log.Info("Running report...")
	if err := report.Run(ctx, odooClient, promClient, cmd.ReportArgs, *cmd.Begin, o...); err != nil {
		return err
	}
	return nil
}

func newPrometheusAPIClient(promURL string, thanosAllowPartialResponses bool, orgId string) (apiv1.API, error) {
	rt := api.DefaultRoundTripper
	rt = &thanos.PartialResponseRoundTripper{
		RoundTripper: rt,
		Allow:        thanosAllowPartialResponses,
	}

	if orgId != "" {
		rt = &thanos.AdditionalHeadersRoundTripper{
			RoundTripper: rt,
			Headers: map[string][]string{
				"X-Scope-OrgID": []string{orgId},
			},
		}
	}

	client, err := api.NewClient(api.Config{
		Address:      promURL,
		RoundTripper: rt,
	})

	return apiv1.NewAPI(client), err
}
