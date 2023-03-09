// Package invoice allows generating invoices from a filled report database.
package invoice

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
	"github.com/jmoiron/sqlx"
)

// Invoice represents an invoice for a tenant.
type Invoice struct {
	Tenant Tenant

	PeriodStart time.Time
	PeriodEnd   time.Time

	Categories []Category
	// Total represents the total accumulated cost of the invoice.
	Total float64
}

// Category represents a category of the invoice i.e. a namespace.
type Category struct {
	Source string
	Target string
	Items  []Item
	// Total represents the total accumulated cost per category.
	Total float64
}

// Item represents a line in the invoice.
type Item struct {
	// Description describes the line item.
	Description string
	// QueryName is the name of the query that generated this line item
	QueryName string
	// Product describes the product this item is based on.
	ProductRef
	// Quantity represents the amount of the resource used.
	Quantity float64
	// QuantityMin represents the minimum amount of the resource used.
	QuantityMin float64
	// QuantityAvg represents the average amount of the resource used.
	QuantityAvg float64
	// QuantityMax represents the maximum amount of the resource used.
	QuantityMax float64
	// Unit represents the unit of the item. e.g. MiB
	Unit string
	// PricePerUnit represents the price per unit in Rappen
	PricePerUnit float64
	// Discount represents a discount in percent. 0.3 discount equals price per unit * 0.7
	Discount float64
	// Total represents the total accumulated cost.
	// (hour1 * quantity * price per unit * discount) + (hour2 * quantity * price
	// per unit * discount)
	Total float64
	// SubItems are entries created by the subqueries of the main invoice item.
	// The keys are the QueryNames of the sub items.
	SubItems map[string]SubItem
}

// SubItem reflects additional information created by a subquery of the main invoice item
type SubItem struct {
	// Description describes the line item.
	Description string
	// QueryName is the name of the query that generated this line item
	QueryName string
	// Quantity represents the amount of the resource used.
	Quantity float64
	// QuantityMin represents the minimum amount of the resource used.
	QuantityMin float64
	// QuantityAvg represents the average amount of the resource used.
	QuantityAvg float64
	// QuantityMax represents the maximum amount of the resource used.
	QuantityMax float64
	// Unit represents the unit of the item. e.g. MiB
	Unit string
}

// Tenant represents a tenant in the invoice.
type Tenant struct {
	Source string
	Target string
}

// ProductRef represents a product reference in the invoice.
type ProductRef struct {
	Source string `db:"product_ref_source"`
	Target string `db:"product_ref_target"`
}

// Generate generates invoices for the given month.
// No data is written to the database. The transaction can be read-only.
func Generate(ctx context.Context, tx *sqlx.Tx, year int, month time.Month) ([]Invoice, error) {
	tenants, err := tenantsForPeriod(ctx, tx, year, month)
	if err != nil {
		return nil, err
	}

	invoices := make([]Invoice, 0, len(tenants))
	for _, tenant := range tenants {
		invoice, err := invoiceForTenant(ctx, tx, tenant, year, month)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, nil
}

func invoiceForTenant(ctx context.Context, tx *sqlx.Tx, tenant db.Tenant, year int, month time.Month) (Invoice, error) {
	var categories []db.Category
	err := sqlx.SelectContext(ctx, tx, &categories,
		`SELECT DISTINCT categories.*
			FROM categories
			INNER JOIN facts ON (facts.category_id = categories.id)
			INNER JOIN date_times ON (facts.date_time_id = date_times.id)
			WHERE date_times.year = $1 AND date_times.month = $2
			AND facts.tenant_id = $3
			ORDER BY categories.source
			`,
		year, int(month), tenant.Id)

	if err != nil {
		return Invoice{}, fmt.Errorf("failed to load categories for %q at %d %s: %w", tenant.Source, year, month.String(), err)
	}

	invCategories := make([]Category, 0, len(categories))
	for _, category := range categories {
		items, err := itemsForCategory(ctx, tx, tenant, category, year, month)
		if err != nil {
			return Invoice{}, err
		}
		invCategories = append(invCategories, Category{
			Source: category.Source,
			Target: category.Target.String,
			Items:  items,
			Total:  sumCategoryTotal(items),
		})
	}

	return Invoice{
		Tenant:      Tenant{Source: tenant.Source, Target: tenant.Target.String},
		PeriodStart: time.Date(year, month, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1),
		Categories:  invCategories,
		Total:       sumInvoiceTotal(invCategories),
	}, nil
}

// rawItem is a line item with additional internal fields for querying.
// This way we do not needlessly expose (and test) internal IDs.
type rawItem struct {
	Item
	// QueryID is the id of the query that generated this item
	QueryID string `db:"query_id"`
	// ParentQueryID is the id of the parent-query of the query that generated this item
	ParentQueryID sql.NullString `db:"parent_query_id"`
	// DiscountID is the id of the corresponding discount
	DiscountID string `db:"discount_id"`
	// ProductID is the id of the corresponding product entry
	ProductID string `db:"product_ref_id"`
}

func itemsForCategory(ctx context.Context, tx *sqlx.Tx, tenant db.Tenant, category db.Category, year int, month time.Month) ([]Item, error) {
	var items []rawItem
	err := sqlx.SelectContext(ctx, tx, &items,
		`SELECT  queries.id as query_id, queries.parent_id as parent_query_id, discounts.id as discount_id,
				queries.description, queries.name as queryName,
				SUM(facts.quantity) as quantity, MIN(facts.quantity) as quantitymin, AVG(facts.quantity) as quantityavg, MAX(facts.quantity) as quantitymax,
				queries.unit, products.amount AS pricePerUnit, discounts.discount,
				products.id as product_ref_id, products.source as product_ref_source, COALESCE(products.target,''::text) as product_ref_target,
				SUM( facts.quantity * products.amount * ( 1::double precision - discounts.discount ) ) AS total
			FROM facts
				INNER JOIN tenants    ON (facts.tenant_id = tenants.id)
				INNER JOIN queries    ON (facts.query_id = queries.id)
				INNER JOIN discounts  ON (facts.discount_id = discounts.id)
				INNER JOIN products   ON (facts.product_id = products.id)
				INNER JOIN date_times ON (facts.date_time_id = date_times.id)
			WHERE date_times.year = $1 AND date_times.month = $2
				AND facts.tenant_id = $3
				AND facts.category_id = $4
			GROUP BY queries.id, products.id, discounts.id
		`,
		year, int(month), tenant.Id, category.Id)

	if err != nil {
		return nil, fmt.Errorf("failed to load item for %q/%q at %d %s: %w", tenant.Source, category.Source, year, month.String(), err)
	}

	return buildItemHierarchy(items), nil
}

// buildItemHierarchy takes a flat list of raw items containing items and sub-items and returns a list of items containing their corresponding sub-items.
// It will drop any sub-item without a matching main item.
func buildItemHierarchy(items []rawItem) []Item {
	mainItems := map[string]Item{}
	for _, item := range items {
		if !item.ParentQueryID.Valid {
			// These three IDs uniquely identify the line item
			itemID := fmt.Sprintf("%s:%s:%s", item.QueryID, item.ProductID, item.DiscountID)
			item.Item.SubItems = map[string]SubItem{}
			mainItems[itemID] = item.Item
		}
	}
	for _, item := range items {
		if item.ParentQueryID.Valid {
			pqid := fmt.Sprintf("%s:%s:%s", item.ParentQueryID.String, item.ProductID, item.DiscountID)
			parent, ok := mainItems[pqid]
			if ok {
				parent.SubItems[item.QueryName] = SubItem{
					Description: item.Description,
					QueryName:   item.QueryName,
					Quantity:    item.Quantity,
					QuantityMin: item.QuantityMin,
					QuantityAvg: item.QuantityAvg,
					QuantityMax: item.QuantityAvg,
					Unit:        item.Unit,
				}
				mainItems[pqid] = parent
			}
		}
	}
	var res []Item
	for _, it := range mainItems {
		res = append(res, it)
	}
	return res
}

func tenantsForPeriod(ctx context.Context, tx *sqlx.Tx, year int, month time.Month) ([]db.Tenant, error) {
	var tenants []db.Tenant

	err := sqlx.SelectContext(ctx, tx, &tenants,
		`SELECT DISTINCT tenants.*
			FROM tenants
				INNER JOIN facts ON (facts.tenant_id = tenants.id)
				INNER JOIN date_times ON (facts.date_time_id = date_times.id)
			WHERE date_times.year = $1 AND date_times.month = $2
			ORDER BY tenants.source, tenants.during
		`,
		year, int(month))

	if err != nil {
		return nil, fmt.Errorf("failed to load tenants for %d %s: %w", year, month.String(), err)
	}
	return tenants, nil
}

func sumCategoryTotal(itms []Item) (sum float64) {
	for _, itm := range itms {
		sum += itm.Total
	}
	return
}

func sumInvoiceTotal(cat []Category) (sum float64) {
	for _, itm := range cat {
		sum += itm.Total
	}
	return
}
