package postgres

import (
	"context"
	"fmt"
	"github.com/AndreeJait/go-utility/sqlw/postgres/dbi"
	"github.com/AndreeJait/go-utility/sqlw/postgres/dbitx"
	"github.com/AndreeJait/go-utility/utils/converter"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (d sqlw) BulkInsert(ctx context.Context, tableName string, fieldName []string, values [][]interface{}) error {
	_, err := d.db.CopyFrom(ctx, pgx.Identifier{tableName}, fieldName, pgx.CopyFromRows(values))
	return err
}

func prepareQueryNamed(queryString QueryString, param QueryParam) (string, []interface{}) {
	var result = string(queryString)
	var args = make([]interface{}, 0)
	count := 1
	for key, value := range param {
		result = strings.ReplaceAll(result, ":"+key, fmt.Sprintf("$%d", count))
		args = append(args, value)
		count += 1
	}

	return result, args
}

func (d sqlw) Get(ctx context.Context, dest interface{}, queryString QueryString, args ...interface{}) error {
	rows, err := d.db.Query(ctx, string(queryString), args...)
	if err != nil {
		return err
	}
	return d.converter.ConvertRows(rows, "db", dest)
}

func (d sqlw) GetNamed(ctx context.Context, dest interface{}, queryString QueryString, param QueryParam) error {
	query, args := prepareQueryNamed(queryString, param)
	rows, err := d.db.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	return d.converter.ConvertRows(rows, "db", dest)
}

func (d sqlw) Select(ctx context.Context, dest interface{}, queryString QueryString, args ...interface{}) error {
	rows, err := d.db.Query(ctx, string(queryString), args...)
	if err != nil {
		return err
	}
	return d.converter.ConvertRows(rows, "db", dest)
}

func (d sqlw) SelectNamed(ctx context.Context, dest interface{}, queryString QueryString, param QueryParam) error {
	query, args := prepareQueryNamed(queryString, param)
	rows, err := d.db.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	return d.converter.ConvertRows(rows, "db", dest)
}

func (d sqlw) Exec(ctx context.Context, queryString QueryString, args ...interface{}) error {
	_, err := d.db.Exec(ctx, string(queryString), args...)
	if err != nil {
		return err
	}
	return nil
}

func (d sqlw) ExecNamed(ctx context.Context, queryString QueryString, param QueryParam) error {
	query, args := prepareQueryNamed(queryString, param)
	_, err := d.db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d sqlw) Begin(ctx context.Context) (dbitx.DBITx, error) {
	return d.db.Begin(ctx)
}

func New(db dbi.DBI) SqlW {
	return &sqlw{
		db:        db,
		converter: converter.New(),
	}
}

// DoInTransaction do process in transaction
func DoInTransaction(ctx context.Context, db SqlW, callbackFunc CallbackFunc) (result interface{}, err error) {
	var tx dbitx.DBITx
	tx, err = db.Begin(ctx)
	if err != nil {
		return result, err
	}

	defer func() {
		if r := recover(); r != nil || err != nil {
			errInternal := tx.Rollback(ctx)
			if errInternal != nil {
				err = errInternal
			}
		} else {
			errInternal := tx.Commit(ctx)
			if errInternal != nil {
				err = errInternal
			}
		}
	}()

	result, err = callbackFunc(ctx, New(tx))
	return
}
