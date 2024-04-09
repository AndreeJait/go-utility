package postgres

import (
	"context"
	"github.com/AndreeJait/go-utility/sqlw/postgres/dbi"
	"github.com/AndreeJait/go-utility/sqlw/postgres/dbitx"
	"github.com/AndreeJait/go-utility/utils/converter"
)

type QueryString string

type QueryParam map[string]interface{}

type CallbackFunc func(ctx context.Context, db SqlW) (interface{}, error)

//go:generate mockery --name=SqlW --filename=mock_sqlw.go --inpackage
type SqlW interface {
	Get(ctx context.Context, dest interface{}, queryString QueryString, args ...interface{}) error
	GetNamed(ctx context.Context, dest interface{}, queryString QueryString, param QueryParam) error

	Select(ctx context.Context, dest interface{}, queryString QueryString, args ...interface{}) error
	SelectNamed(ctx context.Context, dest interface{}, queryString QueryString, param QueryParam) error

	Exec(ctx context.Context, queryString QueryString, args ...interface{}) error
	ExecNamed(ctx context.Context, queryString QueryString, param QueryParam) error

	Begin(ctx context.Context) (dbitx.DBITx, error)

	BulkInsert(ctx context.Context, tableName string, fieldName []string, values [][]interface{}) error
}

type sqlw struct {
	db        dbi.DBI
	converter converter.Converter
}
