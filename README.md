# APPUiO Reporting

[![Build](https://img.shields.io/github/workflow/status/appuio/appuio-reporting/Test)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/appuio/appuio-reporting)
[![Version](https://img.shields.io/github/v/release/appuio/appuio-reporting)][releases]
[![Maintainability](https://img.shields.io/codeclimate/maintainability/appuio/appuio-reporting)][codeclimate]
[![Coverage](https://img.shields.io/codeclimate/coverage/appuio/appuio-reporting)][codeclimate]
[![GitHub downloads](https://img.shields.io/github/downloads/appuio/appuio-reporting/total)][releases]

[build]: https://github.com/appuio/appuio-reporting/actions?query=workflow%3ATest
[releases]: https://github.com/appuio/appuio-reporting/releases
[codeclimate]: https://codeclimate.com/github/appuio/appuio-reporting

## Usage

### Run Report

```sh
# Follow the login instructions to get a token
oc login --server=https://api.cloudscale-lpg-2.appuio.cloud:6443

# Forward mimir to local host
kubectl --as cluster-admin -nvshn-appuio-mimir service/vshn-appuio-mimir-query-frontend 8080

# Set environment
export AR_PROM_URL="http://localhost:8080/prometheus"
export AR_ORG_ID="appuio-managed-openshift-billing" # mimir organization in which data is stored
export AR_ODOO_URL=https://test.central.vshn.ch/api/v2/product_usage_report_POST
export AR_ODOO_OAUTH_TOKEN_URL="https://test.central.vshn.ch/api/v2/authentication/oauth2/token"
export AR_ODOO_OAUTH_CLIENT_ID="your_client_id" # see https://docs.central.vshn.ch/rest-api.html#_authentication_and_authorization
export AR_ODOO_OAUTH_CLIENT_SECRET="your_client_secret"

# Run a query
go run . report --query 'sum by (label) (metric)' --begin "2023-07-08T13:00:00Z" --product-id "your-odoo-product-id" --instance-jsonnet 'local labels = std.extVar("labels"); "instance-%(label)s" % labels' --unit-id "your_odoo_unit_id" --timerange 1h --item-description-jsonnet '"This is a description."' --item-group-description-jsonnet 'local labels = std.extVar("labels"); "Instance %(label)s" % labels'

```
