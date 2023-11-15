package testsuite_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/appuio/appuio-reporting/pkg/testsuite"
)

type TestSuite struct {
	testsuite.Suite
}

func (s *TestSuite) TestPrometheus() {
	t := s.T()
	_, err := s.PrometheusAPIClient().Runtimeinfo(context.Background())
	require.NoError(t, err)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
