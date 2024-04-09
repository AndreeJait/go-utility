package dbitx

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:generate mockery --name=DBITx --filename=mock_DBITx.go --inpackage
type DBITx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Begin(ctx context.Context) (pgx.Tx, error)

	Rollback(ctx context.Context) error
	Commit(ctx context.Context) error

	Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error)

	Conn() *pgx.Conn

	LargeObjects() pgx.LargeObjects

	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}
