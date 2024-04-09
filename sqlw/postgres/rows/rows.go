package rows

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:generate mockery --name=RowsI --filename=mock_RowsI.go --inpackage
type RowsI interface {
	Close()
	Err() error
	CommandTag() pgconn.CommandTag
	FieldDescriptions() []pgconn.FieldDescription
	Next() bool
	Scan(dest ...any) error
	Values() ([]any, error)
	RawValues() [][]byte
	Conn() *pgx.Conn
}
