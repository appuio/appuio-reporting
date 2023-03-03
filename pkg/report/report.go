package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/appuio/appuio-cloud-reporting/pkg/sourcekey"
	"github.com/jackc/pgx/v4"
	"github.com/jmoiron/sqlx"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PromQuerier interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...apiv1.Option) (model.Value, apiv1.Warnings, error)
}

// RunRange executes prometheus queries like Run() until the `until` timestamp is reached or an error occurred.
// Returns the number of reports run and a possible error.
func RunRange(ctx context.Context, database *sqlx.DB, prom PromQuerier, queryName string, from time.Time, until time.Time, options ...Option) (int, error) {
	opts := buildOptions(options)

	n := 0
	for currentTime := from; until.After(currentTime); currentTime = currentTime.Add(time.Hour) {
		n++
		if opts.progressReporter != nil {
			opts.progressReporter(Progress{currentTime, n})
		}
		if err := db.RunInTransaction(ctx, database, func(tx *sqlx.Tx) error {
			return Run(ctx, tx, prom, queryName, currentTime, options...)
		}); err != nil {
			return n, fmt.Errorf("error running report at %s: %w", currentTime.Format(time.RFC3339), err)
		}
	}

	return n, nil
}

// Run executes a prometheus query loaded from queries with using the `queryName` and the timestamp.
// The results of the query are saved in the facts table.
func Run(ctx context.Context, tx *sqlx.Tx, prom PromQuerier, queryName string, from time.Time, options ...Option) error {
	opts := buildOptions(options)

	from = from.In(time.UTC)
	if !from.Truncate(time.Hour).Equal(from) {
		return fmt.Errorf("timestamp should only contain full hours based on UTC, got: %s", from.Format(time.RFC3339Nano))
	}

	var query db.Query
	if err := sqlx.GetContext(ctx, tx, &query, "SELECT * FROM queries WHERE name = $1 AND (during @> $2::timestamptz)", queryName, from); err != nil {
		return fmt.Errorf("failed to load query '%s' at '%s': %w", queryName, from.Format(time.RFC3339), err)
	}

	if err := runQuery(ctx, tx, prom, query, from, opts); err != nil {
		return fmt.Errorf("failed to run query '%s' at '%s': %w", queryName, from.Format(time.RFC3339), err)
	}

	var subQueries []db.Query
	if err := sqlx.SelectContext(ctx, tx, &subQueries,
		"SELECT id, name, description, query, unit, during FROM queries WHERE parent_id  = $1 AND (during @> $2::timestamptz)", query.Id, from,
	); err != nil {
		return fmt.Errorf("failed to load subQueries for '%s' at '%s': %w", queryName, from.Format(time.RFC3339), err)
	}
	for _, subQuery := range subQueries {
		if err := runQuery(ctx, tx, prom, subQuery, from, opts); err != nil {
			return fmt.Errorf("failed to run subQuery '%s' at '%s': %w", subQuery.Name, from.Format(time.RFC3339), err)
		}
	}

	return nil
}

func runQuery(ctx context.Context, tx *sqlx.Tx, prom PromQuerier, query db.Query, from time.Time, opts options) error {
	promQCtx := ctx
	if opts.prometheusQueryTimeout != 0 {
		ctx, cancel := context.WithTimeout(promQCtx, opts.prometheusQueryTimeout)
		defer cancel()
		promQCtx = ctx
	}
	// The data in the database is from T to T+1h. Prometheus queries backwards from T to T-1h.
	res, _, err := prom.Query(promQCtx, query.Query, from.Add(time.Hour))
	if err != nil {
		return fmt.Errorf("failed to query prometheus: %w", err)
	}

	samples, ok := res.(model.Vector)
	if !ok {
		return fmt.Errorf("expected prometheus query to return a model.Vector, got %T", res)
	}

	for _, sample := range samples {
		if err := processSample(ctx, tx, from, query, sample); err != nil {
			return fmt.Errorf("failed to process sample: %w", err)
		}
	}

	return nil
}

func processSample(ctx context.Context, tx *sqlx.Tx, ts time.Time, query db.Query, s *model.Sample) error {
	category, err := getMetricLabel(s.Metric, "category")
	if err != nil {
		return err
	}
	productLabel, err := getMetricLabel(s.Metric, "product")
	if err != nil {
		return err
	}

	skey, err := sourcekey.Parse(string(productLabel))
	if err != nil {
		return fmt.Errorf("failed to parse source key from product label: %w", err)
	}

	var upsertedTenant db.Tenant
	err = upsertTenant(ctx, tx, &upsertedTenant, db.Tenant{
		Source: skey.Tenant,
	}, ts)
	if err != nil {
		return fmt.Errorf("failed to upsert tenant '%s': %w", skey.Tenant, err)
	}

	var upsertedCategory db.Category
	if err := upsertCategory(ctx, tx, &upsertedCategory, db.Category{Source: string(category)}); err != nil {
		return err
	}

	sourceLookup := skey.LookupKeys()

	var product db.Product
	if err := getBySourceKeyAndTime(ctx, tx, &product, pgx.Identifier{"products"}, sourceLookup, ts); err != nil {
		return fmt.Errorf("failed to load product for '%s': %w", productLabel, err)
	}

	var discount db.Discount
	if err := getBySourceKeyAndTime(ctx, tx, &discount, pgx.Identifier{"discounts"}, sourceLookup, ts); err != nil {
		return fmt.Errorf("failed to load discount for '%s': %w", productLabel, err)
	}

	var upsertedDateTime db.DateTime
	err = upsertDateTime(ctx, tx, &upsertedDateTime, db.BuildDateTime(ts))
	if err != nil {
		return fmt.Errorf("failed to upsert date_time '%s': %w", ts.Format(time.RFC3339), err)
	}

	var upsertedFact db.Fact
	err = upsertFact(ctx, tx, &upsertedFact, db.Fact{
		DateTimeId: upsertedDateTime.Id,
		TenantId:   upsertedTenant.Id,
		CategoryId: upsertedCategory.Id,
		QueryId:    query.Id,
		ProductId:  product.Id,
		DiscountId: discount.Id,
		Quantity:   float64(s.Value),
	})
	if err != nil {
		return fmt.Errorf("failed to upsert fact '%s': %w", ts.Format(time.RFC3339), err)
	}

	return nil
}

// getBySourceKeyAndTime gets the first record matching a key in keys while preserving the priority or order of the keys.
// The first key has the highest priority while the last key has the lowest priority.
// If keys are [a,b,c] and records [a,c] exist a is chosen.
func getBySourceKeyAndTime(ctx context.Context, q sqlx.QueryerContext, dest interface{}, table pgx.Identifier, keys []string, ts time.Time) error {
	const query = `WITH keys AS (
		-- add a priority to keep track of which key match we should choose
		-- first key -> prio 1, third key -> prio 3
		SELECT row_number() over () AS prio, unnest as key
		-- unpack the given array of strings into rows
		FROM unnest($1::text[])
	)
	SELECT {{table}}.*
		FROM {{table}}
		INNER JOIN keys ON (keys.key = {{table}}.source)
		WHERE during @> $2::timestamptz
		ORDER BY prio
		LIMIT 1`
	return sqlx.GetContext(ctx, q, dest, strings.ReplaceAll(query, "{{table}}", table.Sanitize()), keys, ts)
}

func upsertFact(ctx context.Context, tx *sqlx.Tx, dst *db.Fact, src db.Fact) error {
	err := db.GetNamedContext(ctx, tx, dst,
		`INSERT INTO facts
				(date_time_id,query_id,tenant_id,category_id,product_id,discount_id,quantity)
			VALUES
				(:date_time_id,:query_id,:tenant_id,:category_id,:product_id,:discount_id,:quantity)
			ON CONFLICT (date_time_id,query_id,tenant_id,category_id,product_id,discount_id)
				DO UPDATE SET quantity = :quantity
			RETURNING *`,
		src)
	if err != nil {
		return fmt.Errorf("failed to upsert fact %+v: %w", src, err)
	}
	return nil
}

func upsertCategory(ctx context.Context, tx *sqlx.Tx, dst *db.Category, src db.Category) error {
	err := db.GetNamedContext(ctx, tx, dst,
		`WITH
				existing AS (
					SELECT * FROM categories WHERE source = :source
				),
				inserted AS (
					INSERT INTO categories (source)
					SELECT :source WHERE NOT EXISTS (SELECT 1 FROM existing)
					RETURNING *
				)
			SELECT * FROM inserted UNION ALL SELECT * FROM existing`,
		src)
	if err != nil {
		return fmt.Errorf("failed to upsert category %+v: %w", src, err)
	}
	return nil
}

func upsertTenant(ctx context.Context, tx *sqlx.Tx, dst *db.Tenant, src db.Tenant, ts time.Time) error {
	err := sqlx.GetContext(ctx, tx, dst,
		`WITH
				existing AS (
					SELECT * FROM tenants WHERE source = $1 AND during @> $2::timestamptz
				),
				inserted AS (
					INSERT INTO tenants (source)
					SELECT $1 WHERE NOT EXISTS (SELECT 1 FROM existing)
					RETURNING *
				)
			SELECT * FROM inserted UNION ALL SELECT * FROM existing`, src.Source, ts)
	if err != nil {
		return fmt.Errorf("failed to upsert tenant %+v: %w", src, err)
	}
	return nil
}

func upsertDateTime(ctx context.Context, tx *sqlx.Tx, dst *db.DateTime, src db.DateTime) error {
	err := db.GetNamedContext(ctx, tx, dst,
		`WITH
		existing AS (
			SELECT * FROM date_times WHERE year = :year AND month = :month AND day = :day AND hour = :hour
		),
		inserted AS (
			INSERT INTO date_times (timestamp, year, month, day, hour)
			SELECT :timestamp, :year, :month, :day, :hour WHERE NOT EXISTS (SELECT 1 FROM existing)
			RETURNING *
		)
		SELECT * FROM inserted UNION ALL SELECT * FROM existing`,
		src)
	if err != nil {
		return fmt.Errorf("failed to upsert date_time %+v: %w", src, err)
	}
	return nil
}

func getMetricLabel(m model.Metric, name string) (model.LabelValue, error) {
	value, ok := m[model.LabelName(name)]
	if !ok {
		return "", fmt.Errorf("expected sample to contain label '%s'", name)
	}
	return value, nil
}
