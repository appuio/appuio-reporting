package report

import (
	"context"
	"encoding/json"
	"fmt"
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
	InstanceJsonnet             string
	ItemDescriptionJsonnet      string
	ItemGroupDescriptionJsonnet string
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
	metricLabels := s.Metric

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

	labelList, err := json.Marshal(metricLabels)
	if err != nil {
		return nil, err
	}

	vm := jsonnet.MakeVM()
	vm.ExtCode("labels", string(labelList))

	instance, err := vm.EvaluateAnonymousSnippet("instance.json", args.InstanceJsonnet)
	if err != nil {
		return nil, err
	}
	instanceStr := ""
	err = json.Unmarshal([]byte(instance), &instanceStr)
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate instance template: %w", err)
	}

	var groupStr string
	if args.ItemGroupDescriptionJsonnet != "" {
		group, err := vm.EvaluateAnonymousSnippet("group.json", args.ItemGroupDescriptionJsonnet)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate group description template: %w", err)
		}
		err = json.Unmarshal([]byte(group), &groupStr)
		if err != nil {
			return nil, err
		}
	}

	var descriptionStr string
	if args.ItemDescriptionJsonnet != "" {
		description, err := vm.EvaluateAnonymousSnippet("description.json", args.ItemDescriptionJsonnet)
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate description template: %w", err)
		}
		err = json.Unmarshal([]byte(description), &descriptionStr)
		if err != nil {
			return nil, err
		}
	}

	timerange := odoo.Timerange{
		From: from,
		To:   from.Add(args.TimerangeSize),
	}

	record := odoo.OdooMeteredBillingRecord{
		ProductID:            args.ProductID,
		InstanceID:           instanceStr,
		ItemDescription:      descriptionStr,
		ItemGroupDescription: groupStr,
		SalesOrderID:         salesOrderID,
		UnitID:               args.UnitID,
		ConsumedUnits:        float64(s.Value),
		Timerange:            timerange,
	}

	return &record, nil
}

func getMetricLabel(m model.Metric, name string) (model.LabelValue, error) {
	value, ok := m[model.LabelName(name)]
	if !ok {
		return "", fmt.Errorf("expected sample to contain label '%s'", name)
	}
	return value, nil
}
