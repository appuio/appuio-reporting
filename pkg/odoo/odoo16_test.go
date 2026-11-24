package odoo_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	"github.com/appuio/appuio-reporting/pkg/odoo"
)

type mockRoundTripper struct {
	cannedResponse  *http.Response
	receivedContent string
}

type mockRoundTripperWhichFails struct {
}

func (rt *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	rt.receivedContent = string(body)
	return rt.cannedResponse, nil
}

func (rt *mockRoundTripperWhichFails) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("there was an error")
}

func TestOdooRecordsSent(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.WriteString("success")
	expectedResponse := recorder.Result()

	mrt := &mockRoundTripper{cannedResponse: expectedResponse}
	client := http.Client{Transport: mrt}

	logger := logr.New(logr.Discard().GetSink())
	uut := odoo.NewOdooAPIWithClient("https://foo.bar/odoo16/", &client, logger)

	require.NoError(t, uut.SendData(context.Background(), []odoo.OdooMeteredBillingRecord{getOdooRecord()}))
	require.Equal(t, `{"data":[{"product_id":"my-product","instance_id":"my-instance","item_description":"my-description","item_group_description":"my-group","sales_order_id":"SO00000","unit_id":"my-unit","consumed_units":11.1,"timerange":"2022-02-22T22:22:22Z/2022-02-22T23:22:22Z"}]}`, mrt.receivedContent)

	require.NoError(t, uut.SendData(context.Background(), nil))
	require.Equal(t, `{"data":[]}`, mrt.receivedContent, "client should ensure that the data field is always represented as an array")
}

func TestErrorHandling(t *testing.T) {
	mrt := &mockRoundTripperWhichFails{}
	client := http.Client{Transport: mrt}

	logger := logr.New(logr.Discard().GetSink())
	uut := odoo.NewOdooAPIWithClient("https://foo.bar/odoo16/", &client, logger)

	err := uut.SendData(context.Background(), []odoo.OdooMeteredBillingRecord{getOdooRecord()})

	require.Error(t, err)
}

func TestErrorFromServerRaisesError(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.WriteHeader(500)
	recorder.WriteString(`{
    "arguments": [
        "data"
    ],
    "code": 500,
    "context": {},
    "message": "data",
    "name": "builtins.KeyError",
    "traceback": [
        "Traceback (most recent call last):",
        "  File \"/opt/odoo/bin/odoo/http.py\", line 1589, in _serve_db",
        "    return service_model.retrying(self._serve_ir_http, self.env)",
        "  File \"/opt/odoo/bin/odoo/service/model.py\", line 133, in retrying",
        "    result = func()",
        "  File \"/opt/odoo/bin/odoo/http.py\", line 1616, in _serve_ir_http",
        "    response = self.dispatcher.dispatch(rule.endpoint, args)",
        "  File \"/opt/odoo/braintec/ext/muk_rest/core/http.py\", line 295, in dispatch",
        "    result = self.request.registry['ir.http']._dispatch(endpoint)",
        "  File \"/opt/odoo/bin/addons/website/models/ir_http.py\", line 237, in _dispatch",
        "    response = super()._dispatch(endpoint)",
        "  File \"/opt/odoo/braintec/ext/muk_rest/models/ir_http.py\", line 160, in _dispatch",
        "    response = super()._dispatch(endpoint)",
        "  File \"/opt/odoo/addons/monitoring_prometheus/models/ir_http.py\", line 38, in _dispatch",
        "    res = super()._dispatch(endpoint)",
        "  File \"/opt/odoo/bin/odoo/addons/base/models/ir_http.py\", line 154, in _dispatch",
        "    result = endpoint(**request.params)",
        "  File \"/opt/odoo/bin/odoo/http.py\", line 697, in route_wrapper",
        "    result = endpoint(self, *args, **params_ok)",
        "  File \"/opt/odoo/braintec/ext/muk_rest/core/http.py\", line 122, in wrapper",
        "    result = func(*args, **kwargs)",
        "  File \"/opt/odoo/braintec/vshn/vshn_metered_usage_rest/controllers/metered_usage_rest.py\", line 65, in vshn_send_metered_usage",
        "    'payload': json.dumps(kw['data'], indent=4)",
        "KeyError: 'data'"
    ]
}`)
	expectedResponse := recorder.Result()

	mrt := &mockRoundTripper{cannedResponse: expectedResponse}
	client := http.Client{Transport: mrt}

	logger := logr.New(logr.Discard().GetSink())
	uut := odoo.NewOdooAPIWithClient("https://foo.bar/odoo16/", &client, logger)

	err := uut.SendData(context.Background(), []odoo.OdooMeteredBillingRecord{getOdooRecord()})

	require.Error(t, err)
}

func Test_Timerange_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		r string
		e odoo.Timerange
	}{
		{
			"\"2026-11-24T11:00:00Z/2026-11-24T12:00:00Z\"",
			odoo.Timerange{
				From: time.Date(2026, time.November, 24, 11, 0, 0, 0, time.UTC),
				To:   time.Date(2026, time.November, 24, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			"\"2026-11-24T11:00:00+01:00/2026-11-24T12:00:00+01:00\"",
			odoo.Timerange{
				From: time.Date(2026, time.November, 24, 10, 0, 0, 0, time.UTC),
				To:   time.Date(2026, time.November, 24, 11, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tc := range cases {
		var r odoo.Timerange
		err := json.Unmarshal([]byte(tc.r), &r)
		require.NoError(t, err)
		require.Equal(t, tc.e, r)
	}
}

func Test_Timerange_UnmarshalJSON_Error(t *testing.T) {
	cases := []struct {
		r string
		e error
	}{
		{"0", errors.New("Expected timerange to be a string")},
		{"\"\"", errors.New("Expected timerange to have shape '<from>/<to>'")},
		{"\"from/\"", errors.New("Expected timerange <from> to be RFC3339: parsing time \"from\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"from\" as \"2006\"")},
		{"\"2026-11-24T11:00:00Z/to\"", errors.New("Expected timerange <to> to be RFC3339: parsing time \"to\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"to\" as \"2006\"")},
	}

	for _, tc := range cases {
		var r odoo.Timerange
		err := json.Unmarshal([]byte(tc.r), &r)
		require.Error(t, err)
		require.Equal(t, tc.e, err)
	}
}

func getOdooRecord() odoo.OdooMeteredBillingRecord {
	return odoo.OdooMeteredBillingRecord{
		ProductID:            "my-product",
		UnitID:               "my-unit",
		SalesOrderID:         "SO00000",
		InstanceID:           "my-instance",
		ItemDescription:      "my-description",
		ItemGroupDescription: "my-group",
		ConsumedUnits:        11.1,
		Timerange: odoo.Timerange{
			From: time.Date(2022, 2, 22, 22, 22, 22, 222, time.UTC),
			To:   time.Date(2022, 2, 22, 23, 22, 22, 222, time.UTC),
		},
	}
}
