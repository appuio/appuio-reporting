package check_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/appuio/appuio-cloud-reporting/pkg/check"
	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/appuio/appuio-cloud-reporting/pkg/db/dbtest"
)

type InconsistentTestSuite struct {
	dbtest.Suite
}

func (s *InconsistentTestSuite) TestInconsistentFields() {
	t := s.T()
	tx := s.Begin()
	defer tx.Rollback()
	require.NoError(t, func() error { _, err := tx.Exec("set timezone to 'UTC';"); return err }())

	m, err := check.Inconsistent(context.Background(), tx)
	require.NoError(t, err)
	require.Len(t, m, 0)

	expectedInconsistent := s.requireInconsistentTestEntries(t, tx)

	m, err = check.Inconsistent(context.Background(), tx)
	require.NoError(t, err)
	require.Equal(t, expectedInconsistent, m)
}

func (s *InconsistentTestSuite) requireInconsistentTestEntries(t *testing.T, tdb *sqlx.Tx) []check.InconsistentField {
	var category db.Category
	require.NoError(t,
		db.GetNamed(tdb, &category,
			"INSERT INTO categories (source,target) VALUES (:source,:target) RETURNING *", db.Category{
				Source: "af-south-1:uroboros-research",
			}))

	at := time.Date(2023, time.January, 2, 3, 0, 0, 0, time.UTC)
	var dateTime db.DateTime
	require.NoError(t,
		db.GetNamed(tdb, &dateTime,
			"INSERT INTO date_times (timestamp, year, month, day, hour) VALUES (:timestamp, :year, :month, :day, :hour) RETURNING *",
			db.BuildDateTime(at),
		))

	discountOutsideRange, err := db.CreateDiscount(tdb, db.Discount{
		Source: "test_memory:us-rac-2",
		During: rangeOutsideDateTimes(),
	})
	require.NoError(t, err)

	var tenantOutsideRange db.Tenant
	require.NoError(t,
		db.GetNamed(tdb, &tenantOutsideRange,
			"INSERT INTO tenants (source,during) VALUES (:source,:during) RETURNING *", db.Tenant{
				Source: "tricell",
				During: rangeOutsideDateTimes(),
			}))

	var tenantInsideRange db.Tenant
	require.NoError(t,
		db.GetNamed(tdb, &tenantInsideRange,
			"INSERT INTO tenants (source,during) VALUES (:source,:during) RETURNING *", db.Tenant{
				Source: "tricell",
				During: db.Timerange(db.MustTimestamp(at), db.MustTimestamp(at.Add(time.Hour))),
			}))

	var productOutsideRange db.Product
	require.NoError(t,
		db.GetNamed(tdb, &productOutsideRange,
			"INSERT INTO products (source,target,amount,unit,during) VALUES (:source,:target,:amount,:unit,:during) RETURNING *", db.Product{
				Source: "test_memory:us-rac-2",
				During: rangeOutsideDateTimes(),
			}))

	var queryOutsideRange db.Query
	require.NoError(t,
		db.GetNamed(tdb, &queryOutsideRange,
			"INSERT INTO queries (name,query,unit,during) VALUES (:name,:query,:unit,:during) RETURNING *", db.Query{
				Name:   "test_memory",
				Query:  "test_memory",
				Unit:   "GiB",
				During: rangeOutsideDateTimes(),
			}))

	testFact := db.Fact{
		DateTimeId: dateTime.Id,
		QueryId:    queryOutsideRange.Id,
		TenantId:   tenantOutsideRange.Id,
		CategoryId: category.Id,
		ProductId:  productOutsideRange.Id,
		DiscountId: discountOutsideRange.Id,
		Quantity:   1,
	}
	createFact(t, tdb, testFact)
	testFact.TenantId = tenantInsideRange.Id
	createFact(t, tdb, testFact)

	formattedAt := at.Format(db.PGTimestampFormat)
	formattedRange := fmt.Sprintf("[\"%s\",\"%s\")",
		rangeOutsideDateTimes().Lower.Time.Format(db.PGTimestampFormat),
		rangeOutsideDateTimes().Upper.Time.Format(db.PGTimestampFormat),
	)
	return []check.InconsistentField{
		{Table: "discounts", DimensionID: discountOutsideRange.Id, FactTime: formattedAt, DimensionRange: formattedRange},
		{Table: "products", DimensionID: productOutsideRange.Id, FactTime: formattedAt, DimensionRange: formattedRange},
		{Table: "queries", DimensionID: queryOutsideRange.Id, FactTime: formattedAt, DimensionRange: formattedRange},
		{Table: "tenants", DimensionID: tenantOutsideRange.Id, FactTime: formattedAt, DimensionRange: formattedRange},
	}
}

func rangeOutsideDateTimes() pgtype.Tstzrange {
	return db.Timerange(
		db.MustTimestamp(time.Date(2023, time.January, 2, 10, 0, 0, 0, time.UTC)),
		db.MustTimestamp(time.Date(2023, time.January, 2, 11, 0, 0, 0, time.UTC)),
	)
}

func createFact(t *testing.T, tx *sqlx.Tx, fact db.Fact) (rf db.Fact) {
	require.NoError(t,
		db.GetNamed(tx, &rf,
			"INSERT INTO facts (date_time_id,query_id,tenant_id,category_id,product_id,discount_id,quantity) VALUES (:date_time_id,:query_id,:tenant_id,:category_id,:product_id,:discount_id,:quantity) RETURNING *", fact))
	return
}

func TestInconsistentTestSuite(t *testing.T) {
	suite.Run(t, new(InconsistentTestSuite))
}
