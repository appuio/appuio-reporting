package report

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/odoo"
	"github.com/google/go-jsonnet"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"go.uber.org/multierr"
)

type PromQuerier interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...apiv1.Option) (model.Value, apiv1.Warnings, error)
}

type OdooClient interface {
	SendData(ctx context.Context, data []odoo.OdooMeteredBillingRecord) error
}

type ReportArgs struct {
	Query                       string
	InstancePattern             string
	ItemDescriptionPattern      string
	ItemGroupDescriptionPattern string
	UnitID                      string
	ProductID                   string
	TimerangeSize               time.Duration
	OverrideSalesOrderID        string
}

const SalesOrderIDLabel = "sales_order_id"

// RunRange executes prometheus queries like Run() until the `until` timestamp is reached or an error occurred.
// Returns the number of reports run and a possible error.
func RunRange(ctx context.Context, odoo OdooClient, prom PromQuerier, args ReportArgs, from time.Time, until time.Time, options ...Option) (int, error) {
	opts := buildOptions(options)

	n := 0
	for currentTime := from; until.After(currentTime); currentTime = currentTime.Add(args.TimerangeSize) {
		n++
		if opts.progressReporter != nil {
			opts.progressReporter(Progress{currentTime, n})
		}
		if err := Run(ctx, odoo, prom, args, currentTime, options...); err != nil {
			return n, fmt.Errorf("error running report at %s: %w", currentTime.Format(time.RFC3339), err)
		}
	}

	return n, nil
}

// Run executes a prometheus query loaded from queries with using the `queryName` and the timestamp.
// The results of the query are saved in the facts table.
func Run(ctx context.Context, odoo OdooClient, prom PromQuerier, args ReportArgs, from time.Time, options ...Option) error {
	opts := buildOptions(options)

	from = from.In(time.UTC)
	if !from.Truncate(time.Hour).Equal(from) {
		return fmt.Errorf("timestamp should only contain full hours based on UTC, got: %s", from.Format(time.RFC3339Nano))
	}

	if err := runQuery(ctx, odoo, prom, args, from, opts); err != nil {
		return fmt.Errorf("failed to run query '%s' at '%s': %w", args.Query, from.Format(time.RFC3339), err)
	}

	return nil
}

func runQuery(ctx context.Context, odooClient OdooClient, prom PromQuerier, args ReportArgs, from time.Time, opts options) error {
	promQCtx := ctx
	if opts.prometheusQueryTimeout != 0 {
		ctx, cancel := context.WithTimeout(promQCtx, opts.prometheusQueryTimeout)
		defer cancel()
		promQCtx = ctx
	}

	// The data in the database is from T to T+1h. Prometheus queries backwards from T to T-1h.
	res, _, err := prom.Query(promQCtx, args.Query, from.Add(args.TimerangeSize))
	if err != nil {
		return fmt.Errorf("failed to query prometheus: %w", err)
	}

	samples, ok := res.(model.Vector)
	if !ok {
		return fmt.Errorf("expected prometheus query to return a model.Vector, got %T", res)
	}

	var errs error
	var records []odoo.OdooMeteredBillingRecord
	for _, sample := range samples {
		record, err := processSample(ctx, odooClient, args, from, sample)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to process sample: %w", err))
		} else {
			records = append(records, *record)
		}
	}

	return multierr.Append(errs, odooClient.SendData(ctx, records))
}

func processSample(ctx context.Context, odooClient OdooClient, args ReportArgs, from time.Time, s *model.Sample) (*odoo.OdooMeteredBillingRecord, error) {
	variables := extractTemplateVars(args)
	values := make(map[string]string)

	for i := 0; i < len(variables); i++ {
		value, err := getMetricLabel(s.Metric, variables[i])
		if err != nil {
			return nil, fmt.Errorf("Unable to obtain label %s from sample: %w", variables[i], err)
		}
		values[variables[i]] = string(value)
	}
	salesOrderID := ""
	if args.OverrideSalesOrderID != "" {
		salesOrderID = args.OverrideSalesOrderID
	} else {
		sid, err := getMetricLabel(s.Metric, SalesOrderIDLabel)
		if err != nil {
			return nil, err
		}
		salesOrderID = string(sid)
	}

	jsonStr, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	vm := jsonnet.MakeVM()

	instance, err := vm.EvaluateAnonymousSnippet("snip.json", fmt.Sprintf("\"%s\" %% %s", args.InstancePattern, jsonStr))
	if err != nil {
		return nil, err
	}
	instance = strings.Trim(instance, "\"\n")

	group, err := vm.EvaluateAnonymousSnippet("snip.json", fmt.Sprintf("\"%s\" %% %s", args.ItemGroupDescriptionPattern, jsonStr))
	if err != nil {
		return nil, err
	}
	group = strings.Trim(group, "\"\n")

	description, err := vm.EvaluateAnonymousSnippet("snip.json", fmt.Sprintf("\"%s\" %% %s", args.ItemDescriptionPattern, jsonStr))
	if err != nil {
		return nil, err
	}
	description = strings.Trim(description, "\"\n")

	timerange := odoo.Timerange{
		From: from,
		To: from.Add(args.TimerangeSize),
	} 

	record := odoo.OdooMeteredBillingRecord{
		ProductID:            args.ProductID,
		InstanceID:           instance,
		ItemDescription:      description,
		ItemGroupDescription: group,
		SalesOrderID:         salesOrderID,
		UnitID:               args.UnitID,
		ConsumedUnits:        float64(s.Value),
		Timerange:            timerange,
	}

	return &record, nil
}

func extractTemplateVars(args ReportArgs) []string {
	// given all the patterns, return list of template variables
	regex := regexp.MustCompile(`%\((\w+)\)s`)
	searchString := args.InstancePattern + ":" + args.ItemGroupDescriptionPattern + ":" + args.ItemDescriptionPattern
	matches := regex.FindAllStringSubmatch(searchString, -1)

	vars := make([]string, len(matches))

	for i := 0; i < len(matches); i++ {
		vars[i] = matches[i][1]
	}

	return vars
}

func getMetricLabel(m model.Metric, name string) (model.LabelValue, error) {
	value, ok := m[model.LabelName(name)]
	if !ok {
		return "", fmt.Errorf("expected sample to contain label '%s'", name)
	}
	return value, nil
}
