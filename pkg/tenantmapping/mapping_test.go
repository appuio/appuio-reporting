package tenantmapping_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	tenantmapping "github.com/appuio/appuio-cloud-reporting/pkg/tenantmapping"
	"github.com/appuio/appuio-cloud-reporting/pkg/testsuite"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type MappingSuite struct {
	testsuite.Suite
}

func (s *MappingSuite) TestReport_RunReportCreatesTenants() {
	t := s.T()
	prom := fakeQuerier{
		mappings: map[string]string{
			"foo-org": "be-555",
			"bar-org": "be-666",
		},
	}

	tx, err := s.DB().Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	ts := time.Now().Truncate(time.Hour)
	require.NoError(t, tenantmapping.MapTenantTarget(context.Background(), tx, prom, ts, tenantmapping.WithPrometheusQueryTimeout(time.Second), tenantmapping.WithMetricSelector(`namespace="testing"`)))

	var tenantCount int
	require.NoError(t, sqlx.Get(tx, &tenantCount, "SELECT COUNT(*) FROM tenants WHERE during @> $1::timestamptz", ts))
	require.Equal(t, 2, tenantCount)
}

func (s *MappingSuite) TestReport_RunReportRemapsExistingTenants() {
	t := s.T()
	prom := fakeQuerier{
		mappings: map[string]string{
			"foo-org": "be-555",
			"bar-org": "be-666",
		},
	}

	tx, err := s.DB().Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	existing := db.Tenant{}
	require.NoError(t,
		db.GetNamed(tx, &existing,
			"INSERT INTO tenants (source,target) VALUES (:source,:target) RETURNING *", db.Tenant{
				Source: "foo-org",
				Target: sql.NullString{String: "be-other", Valid: true},
			}))

	ts := time.Now().Truncate(time.Hour)
	require.NoError(t, tenantmapping.MapTenantTarget(context.Background(), tx, prom, ts, tenantmapping.WithPrometheusQueryTimeout(time.Second)))

	expectedTenants := []comparableTenant{
		{
			Source: "bar-org",
			Target: "666",
			During: fmt.Sprintf("[\"%s\",infinity)", ts.In(time.UTC).Format(db.PGTimestampFormat)),
		},
		{
			Source: "foo-org",
			Target: "555",
			During: fmt.Sprintf("[\"%s\",infinity)", ts.In(time.UTC).Format(db.PGTimestampFormat)),
		},
		{
			Source: "foo-org",
			Target: "be-other",
			During: fmt.Sprintf("[-infinity,\"%s\")", ts.In(time.UTC).Format(db.PGTimestampFormat)),
		},
	}
	var tenants []comparableTenant
	// make timestamps string comparable
	require.NoError(t, func() error { _, err := tx.Exec("set timezone to 'UTC';"); return err }())
	require.NoError(t, sqlx.Select(tx, &tenants, "SELECT source, target, during::text FROM tenants ORDER BY source, target"))
	require.Equal(t, expectedTenants, tenants)
	// Edge case: Don't fail on zero-length ranges.
	// This should only happen if some data was inserted manually. Not failing here aligns with the behavior of any other ranges.
	prom.mappings["foo-org"] = "be-777"
	expectedTenants[1].Target = "777"
	require.NoError(t, tenantmapping.MapTenantTarget(context.Background(), tx, prom, ts))
	require.NoError(t, sqlx.Select(tx, &tenants, "SELECT source, target, during::text FROM tenants ORDER BY source, target"))
	require.Equal(t, expectedTenants, tenants)
}

func (s *MappingSuite) TestReport_RunReport_NewUpperBoundInfinityOrUntilNextRange() {
	t := s.T()
	prom := fakeQuerier{
		mappings: map[string]string{
			"foo-org": "be-555",
			"bar-org": "be-666",
		},
	}

	tx, err := s.DB().Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	ts := time.Now().Truncate(time.Hour)
	futureTS := ts.Add(5 * time.Hour)
	pastTS := ts.Add(-5 * time.Hour)

	existingFoo := db.Tenant{}
	require.NoError(t,
		db.GetNamed(tx, &existingFoo,
			"INSERT INTO tenants (source,target,during) VALUES (:source,:target,:during) RETURNING *", db.Tenant{
				Source: "foo-org",
				Target: sql.NullString{String: "be-other", Valid: true},
				During: db.Timerange(db.MustTimestamp(futureTS), db.MustTimestamp(pgtype.Infinity)),
			}))

	existingBar := db.Tenant{}
	require.NoError(t,
		db.GetNamed(tx, &existingBar,
			"INSERT INTO tenants (source,target,during) VALUES (:source,:target,:during) RETURNING *", db.Tenant{
				Source: "bar-org",
				Target: sql.NullString{String: "be-other", Valid: true},
				During: db.Timerange(db.MustTimestamp(pgtype.NegativeInfinity), db.MustTimestamp(pastTS)),
			}))

	require.NoError(t, tenantmapping.MapTenantTarget(context.Background(), tx, prom, ts))

	expectedTenants := []comparableTenant{
		{
			Source: "bar-org",
			Target: "666",
			During: fmt.Sprintf("[\"%s\",infinity)", ts.In(time.UTC).Format(db.PGTimestampFormat)),
		},
		{
			Source: "bar-org",
			Target: "be-other",
			During: fmt.Sprintf("[-infinity,\"%s\")", pastTS.In(time.UTC).Format(db.PGTimestampFormat)),
		},
		{
			Source: "foo-org",
			Target: "555",
			During: fmt.Sprintf("[\"%s\",\"%s\")", ts.In(time.UTC).Format(db.PGTimestampFormat), futureTS.In(time.UTC).Format(db.PGTimestampFormat)),
		},
		{
			Source: "foo-org",
			Target: "be-other",
			During: fmt.Sprintf("[\"%s\",infinity)", futureTS.In(time.UTC).Format(db.PGTimestampFormat)),
		},
	}
	var tenants []comparableTenant
	// make timestamps string comparable
	require.NoError(t, func() error { _, err := tx.Exec("set timezone to 'UTC';"); return err }())
	require.NoError(t, sqlx.Select(tx, &tenants, "SELECT source, target, during::text FROM tenants ORDER BY source, target"))
	require.Equal(t, expectedTenants, tenants)
}

func TestReport(t *testing.T) {
	suite.Run(t, new(MappingSuite))
}

type fakeQuerier struct {
	mappings map[string]string
}

func (q fakeQuerier) Query(ctx context.Context, query string, ts time.Time, _ ...apiv1.Option) (model.Value, apiv1.Warnings, error) {
	var res model.Vector
	for k, s := range q.mappings {
		res = append(res, &model.Sample{
			Metric: map[model.LabelName]model.LabelValue{
				"__name__":       "control_api_organization_billing_entity_ref",
				"organization":   model.LabelValue(k),
				"billing_entity": model.LabelValue(s),
			},
			Value: 1,
		})
	}
	return res, nil, nil
}

type comparableTenant struct {
	Source string
	Target string
	During string
}
