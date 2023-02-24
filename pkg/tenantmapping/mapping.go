package tenantmapping

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/common/model"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/appuio/appuio-cloud-reporting/pkg/report"
)

var (
	promQuery = `sum by(organization, billing_entity) (control_api_organization_billing_entity_ref{%s})`
)

type options struct {
	prometheusQueryTimeout time.Duration
	metricSelector         string
}

// Option represents a report option.
type Option interface {
	set(*options)
}

func buildOptions(os []Option) options {
	var build options
	for _, o := range os {
		o.set(&build)
	}
	return build
}

// WithPrometheusQueryTimeout allows setting a timout when querying prometheus.
func WithPrometheusQueryTimeout(tm time.Duration) Option {
	return prometheusQueryTimeout(tm)
}

type prometheusQueryTimeout time.Duration

func (t prometheusQueryTimeout) set(o *options) {
	o.prometheusQueryTimeout = time.Duration(t)
}

// WithMetricSelector allows further specifying which metrics to choose.
// Example: WithMetricSelector(`namespace="testing"`)
func WithMetricSelector(q string) Option {
	return metricSelector(q)
}

type metricSelector string

func (t metricSelector) set(o *options) {
	o.metricSelector = string(t)
}

// MapTenantTarget maps the tenants (source, target) for a given time, read from a control-api prometheus metric.
// Truncates the time to the current hour.
func MapTenantTarget(ctx context.Context, tx *sqlx.Tx, prom report.PromQuerier, at time.Time, options ...Option) error {
	log := logr.FromContextOrDiscard(ctx).WithValues("at", at)
	opts := buildOptions(options)
	at = at.In(time.UTC).Truncate(time.Hour)

	promQCtx := ctx
	if opts.prometheusQueryTimeout != 0 {
		ctx, cancel := context.WithTimeout(promQCtx, opts.prometheusQueryTimeout)
		defer cancel()
		promQCtx = ctx
	}
	res, _, err := prom.Query(promQCtx, fmt.Sprintf(promQuery, opts.metricSelector), at)
	if err != nil {
		return fmt.Errorf("failed to query prometheus: %w", err)
	}

	samples, ok := res.(model.Vector)
	if !ok {
		return fmt.Errorf("expected prometheus query to return a model.Vector, got %T", res)
	}

	log.V(1).Info("processing samples", "count", len(samples))
	for _, sample := range samples {
		if err := processSample(ctx, tx, at, sample); err != nil {
			return fmt.Errorf("failed to process sample: %w", err)
		}
	}

	return nil
}

func processSample(ctx context.Context, tx *sqlx.Tx, ts time.Time, s *model.Sample) error {
	log := logr.FromContextOrDiscard(ctx).WithValues("at", ts)

	organization, err := getMetricLabel(s.Metric, "organization")
	if err != nil {
		return err
	}
	// Entity can be unset
	billingEntityRaw, _ := getMetricLabel(s.Metric, "billing_entity")
	billingEntity := strings.TrimPrefix(string(billingEntityRaw), "be-")
	log = log.WithValues("organization", organization, "billing_entity", billingEntity)

	et := db.Tenant{}
	err = tx.GetContext(ctx, &et, "SELECT * FROM tenants WHERE source = $1 AND during @> $2::timestamptz", organization, ts)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if et.Target.String == billingEntity {
		log.V(1).Info("tenant mapping already up to date")
		return nil
	}

	if et.Id != "" {
		log = log.WithValues("id", et.Id)
		log.Info("found existing tenant mapping, updating")
		if l, ok := et.During.Lower.Get().(time.Time); ok && l.Equal(ts) {
			log.Info("update would result in empty range, deleting instead")
			if _, err := tx.ExecContext(ctx, "DELETE FROM tenants WHERE id = $1", et.Id); err != nil {
				return err
			}
		} else {
			log.Info("setting upper bound of existing tenant mapping")
			et.During.Upper.Set(ts)
			if _, err := tx.NamedExecContext(ctx, "UPDATE tenants SET during = :during WHERE id = :id", et); err != nil {
				return err
			}
		}
	}

	it := db.Tenant{}
	// Set the upper bound to the next lower bound, or infinity if there is none
	err = tx.GetContext(ctx, &it, `
		WITH upper AS (
			select coalesce(min(lower(during)),'infinity'::timestamptz) as upper from tenants where source = $1 AND lower(during) > $2
		)
		INSERT INTO tenants (source,target,during)
		VALUES (
			$3, $4,
			tstzrange(
				$5::timestamptz,
				(select upper from upper),
				'[)')
			)
		RETURNING *`, organization, ts, organization, billingEntity, ts)
	if err != nil {
		return err
	}
	log.Info("created new tenant mapping", "tenant", it.Id,
		"during_lower", it.During.Lower.Get().(fmt.Stringer).String(),
		"during_upper", it.During.Upper.Get().(fmt.Stringer).String())

	return nil
}

func getMetricLabel(m model.Metric, name string) (model.LabelValue, error) {
	value, ok := m[model.LabelName(name)]
	if !ok {
		return "", fmt.Errorf("expected sample to contain label '%s'", name)
	}
	return value, nil
}
