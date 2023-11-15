package main

import (
	"github.com/urfave/cli/v2"
)

const defaultTextForRequiredFlags = "<required>"
const defaultTextForOptionalFlags = "<optional>"

func newPromURLFlag(destination *string) *cli.StringFlag {
	return &cli.StringFlag{Name: "prom-url", Usage: "Prometheus connection URL in the form of http://host:port",
		EnvVars: envVars("PROM_URL"), Destination: destination, Value: "http://localhost:9090"}
}

func newOdooURLFlag(destination *string) *cli.StringFlag {
	return &cli.StringFlag{Name: "odoo-url", Usage: "URL of the Odoo Metered Billing API",
		EnvVars: envVars("ODOO_URL"), Destination: destination, Value: "http://localhost:8080"}
}
