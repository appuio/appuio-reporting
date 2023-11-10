package report_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/appuio/appuio-cloud-reporting/pkg/odoo"
	"github.com/appuio/appuio-cloud-reporting/pkg/report"
	"github.com/appuio/appuio-cloud-reporting/pkg/testsuite"
)

type ReportSuite struct {
	testsuite.Suite
}

func TestReport(t *testing.T) {
	suite.Run(t, new(ReportSuite))
}

const defaultQueryReturnValue = 42
const defaultSubQueryReturnValue = 13
const promTestquery = `
	label_replace(
		label_replace(
			label_replace(
				label_replace(
					vector(%d),
					"namespace", "my-namespace", "", ""
				),
				"product", "my-product", "", ""
			),
			"tenant", "my-tenant", "", ""
		),
	"sales_order_id", "SO00000", "", ""
	)
`
const promInvalidTestquery = `
	label_replace(
		label_replace(
			label_replace(
				vector(%d),
				"namespace", "my-namespace", "", ""
			),
			"product", "my-product", "", ""
		),
		"tenant", "my-tenant", "", ""
	)
`

func (s *ReportSuite) TestReport_ReturnsErrorIfTimestampContainsUnitsSmallerOneHour() {
	t := s.T()
	o := &MockOdooClient{}
	prom := s.PrometheusAPIClient()

	baseTime := time.Date(2020, time.January, 23, 17, 0, 0, 0, time.UTC)
	for _, d := range []time.Duration{time.Minute, time.Second, time.Nanosecond} {
		require.Error(t, report.Run(context.Background(), o, prom, getReportArgs(), baseTime.Add(d)))
	}
}

func (s *ReportSuite) TestReport_RunRange() {
	t := s.T()
	o := &MockOdooClient{}
	prom := s.PrometheusAPIClient()

	const hoursToCalculate = 3

	base := time.Date(2020, time.January, 23, 17, 0, 0, 0, time.UTC)

	expectedProgress := []report.Progress{
		{base.Add(0 * time.Hour), 1},
		{base.Add(1 * time.Hour), 2},
		{base.Add(2 * time.Hour), 3},
	}

	progress := make([]report.Progress, 0)

	c, err := report.RunRange(context.Background(), o, prom, getReportArgs(), base, base.Add(hoursToCalculate*time.Hour),
		report.WithProgressReporter(func(p report.Progress) { progress = append(progress, p) }),
	)
	require.NoError(t, err)
	require.Equal(t, hoursToCalculate, c)
	require.Equal(t, expectedProgress, progress)

	require.Equal(t, hoursToCalculate, o.totalReceived)
}

func (s *ReportSuite) TestReport_Run() {
	t := s.T()
	o := &MockOdooClient{}
	prom := s.PrometheusAPIClient()
	args := getReportArgs()

	args.InstancePattern = "%(tenant)s"
	args.ItemGroupDescriptionPattern = "%(namespace)s"
	args.ItemDescriptionPattern = "%(product)s"

	from := time.Date(2020, time.January, 23, 17, 0, 0, 0, time.UTC)

	err := report.Run(context.Background(), o, prom, args, from)
	require.NoError(t, err)

	require.Equal(t, "my-namespace", o.lastReceivedData.ItemGroupDescription)
	require.Equal(t, "my-tenant", o.lastReceivedData.InstanceID)
	require.Equal(t, "my-product", o.lastReceivedData.ItemDescription)
	require.Equal(t, 1.0, o.lastReceivedData.ConsumedUnits)
	require.Equal(t, "SO00000", o.lastReceivedData.SalesOrderID)
}

func (s *ReportSuite) TestReport_RequireErrorWhenInvalidTemplateVariable() {
	t := s.T()
	o := &MockOdooClient{}
	prom := s.PrometheusAPIClient()
	from := time.Date(2020, time.January, 23, 17, 0, 0, 0, time.UTC)

	args := getReportArgs()
	args.InstancePattern = "%(doesnotexist)s"

	err := report.Run(context.Background(), o, prom, args, from)
	require.Error(t, err)

	args = getReportArgs()
	args.ItemGroupDescriptionPattern = "%(doesnotexist)s"

	err = report.Run(context.Background(), o, prom, args, from)
	require.Error(t, err)

	args = getReportArgs()
	args.ItemDescriptionPattern = "%(doesnotexist)s"

	err = report.Run(context.Background(), o, prom, args, from)
	require.Error(t, err)
}

func (s *ReportSuite) TestReport_RequireErrorWhenNoSalesOrder() {
	t := s.T()
	o := &MockOdooClient{}
	prom := s.PrometheusAPIClient()
	args := getReportArgs()
	args.Query = fmt.Sprintf(promInvalidTestquery, 1)

	from := time.Date(2020, time.January, 23, 17, 0, 0, 0, time.UTC)

	err := report.Run(context.Background(), o, prom, args, from)
	require.Error(t, err)
}

func getReportArgs() report.ReportArgs {
	return report.ReportArgs{
		ProductID:                   "myProductId",
		UnitID:                      "unit_kg",
		Query:                       fmt.Sprintf(promTestquery, 1),
		InstancePattern:             "myinstance",
		ItemGroupDescriptionPattern: "myitemgroup",
		ItemDescriptionPattern:      "myitemdescription",
		TimerangeSize:               time.Hour,
	}
}

type MockOdooClient struct {
	totalReceived    int
	lastReceivedData odoo.OdooMeteredBillingRecord
}

func (c *MockOdooClient) SendData(ctx context.Context, data odoo.OdooMeteredBillingRecord) error {
	c.lastReceivedData = data
	c.totalReceived = c.totalReceived + 1
	return nil
}
