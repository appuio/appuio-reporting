package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"
)

// PGTimestampFormat is the string representation of PostgreSQL's `timestamptz`.
const PGTimestampFormat = "2006-01-02 15:04:05-07"

// NamedPreparer is an interface used by GetNamed.
type NamedPreparer interface {
	PrepareNamed(query string) (*sqlx.NamedStmt, error)
}

// NamedPreparerContext is an interface used by GetNamedContext.
type NamedPreparerContext interface {
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
}

// GetNamed is like sqlx.Get but for named statements.
func GetNamed(p NamedPreparer, dest interface{}, query string, arg interface{}) error {
	st, err := p.PrepareNamed(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer st.Close()
	return st.Get(dest, arg)
}

// GetNamedContext is like sqlx.GetContext but for named statements.
func GetNamedContext(ctx context.Context, p NamedPreparerContext, dest interface{}, query string, arg interface{}) error {
	st, err := p.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer st.Close()
	return st.GetContext(ctx, dest, arg)
}

// SelectNamed is like sqlx.Select but for named statements.
func SelectNamed(p sqlx.Ext, dest interface{}, query string, arg interface{}) error {
	nq, narg, err := sqlx.Named(query, arg)
	if err != nil {
		return fmt.Errorf("failed to substitute name parameters: %w", err)
	}
	nq = p.Rebind(nq)
	return sqlx.Select(p, dest, nq, narg...)
}

// SelectNamedContext is like sqlx.SelectContext but for named statements.
func SelectNamedContext(ctx context.Context, p sqlx.ExtContext, dest interface{}, query string, arg interface{}) error {
	nq, narg, err := sqlx.Named(query, arg)
	if err != nil {
		return fmt.Errorf("failed to substitute name parameters: %w", err)
	}
	nq = p.Rebind(nq)
	return sqlx.SelectContext(ctx, p, dest, nq, narg...)
}

// InfiniteRange returns an infinite PostgreSQL timerange [-Inf,Inf).
func InfiniteRange() pgtype.Tstzrange {
	return Timerange(MustTimestamp(pgtype.NegativeInfinity), MustTimestamp(pgtype.Infinity))
}

// RunInTransaction runs the given cb func in a transaction.
// If the func returns an error, the transaction is rolled back, otherwise committed.
func RunInTransaction(ctx context.Context, db *sqlx.DB, cb func(tx *sqlx.Tx) error) error {
	// TODO: Add unit test
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	if err := cb(tx); err != nil {
		return err
	}
	return tx.Commit()
}
