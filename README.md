# APPUiO Cloud Reporting

[![Build](https://img.shields.io/github/workflow/status/appuio/appuio-cloud-reporting/Test)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/appuio/appuio-cloud-reporting)
[![Version](https://img.shields.io/github/v/release/appuio/appuio-cloud-reporting)][releases]
[![Maintainability](https://img.shields.io/codeclimate/maintainability/appuio/appuio-cloud-reporting)][codeclimate]
[![Coverage](https://img.shields.io/codeclimate/coverage/appuio/appuio-cloud-reporting)][codeclimate]
[![GitHub downloads](https://img.shields.io/github/downloads/appuio/appuio-cloud-reporting/total)][releases]

[build]: https://github.com/appuio/appuio-cloud-reporting/actions?query=workflow%3ATest
[releases]: https://github.com/appuio/appuio-cloud-reporting/releases
[codeclimate]: https://codeclimate.com/github/appuio/appuio-cloud-reporting

## Use APPUiO Global instance

```sh
# Follow the login instructions to get a token
oc login --server=https://api.cloudscale-lpg-2.appuio.cloud:6443

# Forward database and thanos to local host
kubectl -n appuio-reporting port-forward svc/reporting-db 5432 &
kubectl --as=cluster-admin -n appuio-thanos port-forward svc/thanos-query 9090 &

# Check for pending migrations
DB_USER=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.user}' | base64 --decode)
DB_PASSWORD=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.password}' | base64 --decode)
export ACR_DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost/reporting?sslmode=disable"
go run . migrate --show-pending

# Run a query
go run . report --query-name ping --begin "2022-01-17T09:00:00Z"

# Connect to the database's interactive terminal
DB_USER=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.user}' | base64 --decode)
export PGPASSWORD=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.password}' | base64 --decode)
psql -U "${DB_USER}" -w -h localhost reporting
```

## Local Installation

```sh
SUPERUSER_PW=$(pwgen 40 1)

kubectl create ns appuio-reporting
kubectl -n appuio-reporting create secret generic reporting-db-superuser --from-literal=user=reporting-db-superuser "--from-literal=password=${SUPERUSER_PW}"
kubectl -n appuio-reporting apply -k manifests/base
```

### Grafana

There is a Grafana deployment prepared under `manifests/grafana`.
To be able to use the deployment, customize the parameters in `grafana-helm-values.yaml` and run `make` to generate the manifest.

Add the required Grafana Helm chart using `helm repo add grafana https://grafana.github.io/helm-charts`.

The deployment requires a secret `grafana-creds` containing the admin username and password:

```sh
oc -n appuio-reporting create secret generic grafana-creds --from-literal=admin-password=$(pwgen 40 1) --from-literal=admin-user=admin
```

## Usage

### Run Report

```sh
kubectl -n appuio-reporting port-forward svc/reporting-db 5432 &
kubectl --as=cluster-admin -n appuio-thanos port-forward svc/thanos-query 9090 &

DB_USER=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.user}' | base64 --decode)
DB_PASSWORD=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.password}' | base64 --decode)
export ACR_DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost/reporting?sslmode=disable"

go run . report --query-name ping --begin "2022-01-17T09:00:00Z"
```

### Migrate to Most Recent Schema

```sh
kubectl -n appuio-reporting port-forward svc/reporting-db 5432 &

DB_USER=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.user}' | base64 --decode)
DB_PASSWORD=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.password}' | base64 --decode)
export ACR_DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost/reporting?sslmode=disable"

go run . migrate --show-pending

go run . migrate
```

### Connect to the Database

```sh
kubectl -n appuio-reporting port-forward svc/reporting-db 5432 &

DB_USER=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.user}' | base64 --decode)
export PGPASSWORD=$(kubectl -n appuio-reporting get secret/reporting-db-superuser -o jsonpath='{.data.password}' | base64 --decode)

psql -U "${DB_USER}" -w -h localhost reporting
```

## Local Development

Local development assumes a locally installed PostgreSQL database.
This can be achieved by running `make docker-compose-up`.
See `docker-compose.yml` for the configuration.

```sh
# Needs to be repeated after a Docker restart
make docker-compose-up  

# Next command asks for a password, it is "reporting"
createdb --username=reporting -h localhost -p 5432 appuio-cloud-reporting-test

export ACR_DB_URL="postgres://reporting:reporting@localhost/appuio-cloud-reporting-test?sslmode=disable"

go run . migrate
go run . migrate --seed
go test ./...

# To connect to the DB:
psql -U reporting -W -h localhost appuio-cloud-reporting-test
```

### IDE Integration

To enable IDE Test/Debug support, `ACR_DB_URL` should be added to the test environment.

#### VSCode

```sh
mkdir -p .vscode
touch .vscode/settings.json
jq -s '(.[0] // {}) | ."go.testEnvVars"."ACR_DB_URL" = $ENV."ACR_DB_URL"' .vscode/settings.json > .vscode/settings.json.i
mv .vscode/settings.json.i .vscode/settings.json
```
