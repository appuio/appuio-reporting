package check

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"go.uber.org/multierr"

	"github.com/appuio/appuio-cloud-reporting/pkg/db"
)

var dimensionsWithTimeranges = []db.Model{
	db.Discount{},
	db.Product{},
	db.Query{},
	db.Tenant{},
}

const inconsistentFactsQuery = `
select distinct '{{table}}' as "table", {{table}}.id as DimensionID, date_times.timestamp::text as FactTime, {{table}}.during::text as DimensionRange from facts
  inner join {{table}} on facts.{{foreign_key}} = {{table}}.id
  inner join date_times on facts.date_time_id = date_times.id
  where false = {{table}}.during @> date_times.timestamp
`

// InconsistentField represents an inconsistent field.
type InconsistentField struct {
	Table string

	DimensionID string

	FactTime       string
	DimensionRange string
}

// Inconsistent checks for facts with inconsistent time ranges.
// Those are facts that reference a dimension with a time range that does not include the fact's timestamp.
func Inconsistent(ctx context.Context, tx sqlx.QueryerContext) ([]InconsistentField, error) {
	var inconsistent []InconsistentField
	var errors []error
	for _, m := range dimensionsWithTimeranges {
		var ic []InconsistentField
		q := strings.NewReplacer("{{table}}", m.TableName(), "{{foreign_key}}", m.ForeignKeyName()).Replace(inconsistentFactsQuery)
		err := sqlx.SelectContext(ctx, tx, &ic, fmt.Sprintf(`WITH inconsistent AS (%s) SELECT * FROM inconsistent ORDER BY "table",FactTime`, q))
		errors = append(errors, err)
		inconsistent = append(inconsistent, ic...)
	}

	return inconsistent, multierr.Combine(errors...)
}
