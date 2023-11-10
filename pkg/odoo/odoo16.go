package odoo

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
)

type OdooAPIClient struct {
	OdooURL           string
	OauthClientID     string
	OauthClientSecret string
	logger            logr.Logger
}

type OdooMeteredBillingRecord struct {
	ProductID            string  `json:"product_id"`
	InstanceID           string  `json:"instance_id"`
	ItemDescription      string  `json:"item_description,omitempty"`
	ItemGroupDescription string  `json:"item_group_description,omitempty"`
	SalesOrderID         string  `json:"sales_order_id"`
	UnitID               string  `json:"unit_id"`
	ConsumedUnits        float64 `json:"consumed_units"`
	Timerange            string  `json:"timerange"`
}

func NewOdooAPIClient(odooURL string, oauthClientId string, oauthClientSecret string, logger logr.Logger) (*OdooAPIClient, error) {
	return &OdooAPIClient{
		OdooURL:           odooURL,
		OauthClientID:     oauthClientId,
		OauthClientSecret: oauthClientSecret,
		logger:            logger,
	}, nil
}

func (c OdooAPIClient) SendData(ctx context.Context, data OdooMeteredBillingRecord) error {
	str, _ := json.Marshal(data)
	c.logger.Info("<" + data.InstanceID + ">")
	c.logger.Info(string(str))
	return nil
}
