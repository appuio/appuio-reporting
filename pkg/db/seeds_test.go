package db_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/appuio/appuio-cloud-reporting/pkg/db/dbtest"
)

type SeedsTestSuite struct {
	dbtest.Suite
}

func (s *SeedsTestSuite) TestSeedDefaultQueries() {
	t := s.T()
	d := s.DB()

	_, err := d.Exec("DELETE FROM queries")
	require.NoError(t, err)

	expQueryNum := 7

	count := "SELECT COUNT(*) FROM queries"
	requireQueryEqual(t, d, 0, count)

	require.NoError(t, db.SeedQueries(d.DB, []db.Query{
		{
			Name:        "appuio_cloud_memory",
			Description: "Memory usage (maximum of requested and used memory) aggregated by namespace",
			Unit:        "MiB",
		},
	}))
	t.Log(t, count)
	requireQueryEqual(t, d, 1, count)

	// Some appuio_cloud_memory exists so don't create sub queries
	err = db.Seed(d.DB)
	require.NoError(t, err)
	requireQueryEqual(t, d, expQueryNum-2, count)

	// Drop queries and check we create sub queries
	_, err = d.DB.Exec("DELETE FROM queries;")
	require.NoError(t, err)
	err = db.Seed(d.DB)
	require.NoError(t, err)
	requireQueryEqual(t, d, expQueryNum, count)
	err = db.Seed(d.DB)
	require.NoError(t, err)
	requireQueryEqual(t, d, expQueryNum, count)
}

func requireQueryEqual[T any](t *testing.T, q sqlx.Queryer, expected T, query string, args ...interface{}) {
	t.Helper()
	var res T
	require.NoError(t, sqlx.Get(q, &res, query, args...))
	require.Equal(t, expected, res)
}

func requireQueryTrue(t *testing.T, q sqlx.Queryer, query string, args ...interface{}) {
	t.Helper()
	requireQueryEqual(t, q, true, query, args...)
}

func TestSeeds(t *testing.T) {
	suite.Run(t, new(SeedsTestSuite))
}
