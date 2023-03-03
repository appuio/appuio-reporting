package invoice_test

import (
	"database/sql"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/stretchr/testify/require"
)

func (s *InvoiceGoldenSuite) TestInvoiceGolden_Tenants() {
	t := s.T()
	tdb := s.DB()

	_, err := db.CreateTenant(tdb, db.Tenant{
		Source: "tricell",
		Target: sql.NullString{Valid: true, String: "98757"},
		During: timerange(t, "-", "2022-01-20"),
	})
	require.NoError(t, err)

	_, err = db.CreateTenant(tdb, db.Tenant{
		Source: "tricell",
		Target: sql.NullString{Valid: true, String: "98942"},
		During: timerange(t, "2022-01-20", "-"),
	})
	require.NoError(t, err)

	_, err = db.CreateTenant(tdb, db.Tenant{
		Source: "umbrellacorp",
		Target: sql.NullString{Valid: true, String: "96432"},
		During: db.InfiniteRange(),
	})
	require.NoError(t, err)

	_, err = db.CreateTenant(tdb, db.Tenant{
		Source: "megacorp",
		Target: sql.NullString{Valid: true, String: "83492"},
		During: db.InfiniteRange(),
	})
	require.NoError(t, err)

	_, err = db.CreateProduct(tdb, db.Product{
		Source: "my-product",
		Amount: 1,
		During: db.InfiniteRange(),
	})
	require.NoError(t, err)

	_, err = db.CreateDiscount(tdb, db.Discount{
		Source: "my-product",
		During: db.InfiniteRange(),
	})
	require.NoError(t, err)

	query, err := db.CreateQuery(tdb, db.Query{
		Name:        "test",
		Description: "test description",
		Query:       "test",
		Unit:        "tps",
		During:      db.InfiniteRange(),
	})
	require.NoError(t, err)

	s.prom.queries[query.Query] = fakeQueryResults{
		"my-product:my-cluster:tricell:my-namespace":      fakeQuerySample{Value: 42}, // split over two tenant targets
		"my-product:my-cluster:megacorp:my-namespace":     fakeQuerySample{Value: 42}, // same value to verify that the sum of both tricell tenant targets is correct
		"my-product:my-cluster:umbrellacorp:my-namespace": fakeQuerySample{Value: 14},
	}

	runReport(t, tdb, s.prom, query.Query, "2022-01-01", "2022-01-30")
	invoiceEqualsGolden(t, "tenants",
		generateInvoice(t, tdb, 2022, time.January),
		*updateGolden)
}
