package postgres

import (
	"context"
	"errors"
	"github.com/AndreeJait/go-utility/sqlw/postgres/dbi"
	"github.com/AndreeJait/go-utility/sqlw/postgres/dbitx"
	"github.com/AndreeJait/go-utility/sqlw/postgres/rows"
	"github.com/AndreeJait/go-utility/utils/converter"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"reflect"
	"testing"
)

func TestDoInTransaction(t *testing.T) {
	mockSqlW := NewMockSqlW(t)
	mockDBItx := dbitx.NewMockDBITx(t)

	type args struct {
		ctx          context.Context
		db           SqlW
		callbackFunc CallbackFunc
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		mock       func()
		wantResult interface{}
	}{
		{
			name: "test do in transaction error begin",
			mock: func() {
				mockSqlW.
					On("Begin", context.Background()).
					Return(nil, errors.New("error here")).Once()
			},
			args: args{
				db:  mockSqlW,
				ctx: context.Background(),
				callbackFunc: func(ctx context.Context, db SqlW) (interface{}, error) {
					return nil, errors.New("something happen")
				},
			},
			wantErr: true,
		},
		{
			name: "test do in transaction error rollback",
			mock: func() {
				mockSqlW.
					On("Begin", context.Background()).
					Return(mockDBItx, nil).Once()

				mockDBItx.On("Rollback", context.Background()).
					Return(errors.New("something error - rollback")).Once()
			},
			args: args{
				db:  mockSqlW,
				ctx: context.Background(),
				callbackFunc: func(ctx context.Context, db SqlW) (interface{}, error) {
					return nil, errors.New("something happen")
				},
			},
			wantErr: true,
		},
		{
			name: "test do in transaction error rollback - success",
			mock: func() {
				mockSqlW.
					On("Begin", context.Background()).
					Return(mockDBItx, nil).Once()

				mockDBItx.On("Rollback", context.Background()).
					Return(nil).Once()
			},
			args: args{
				db:  mockSqlW,
				ctx: context.Background(),
				callbackFunc: func(ctx context.Context, db SqlW) (interface{}, error) {
					return nil, errors.New("something happen")
				},
			},
			wantErr: true,
		},
		{
			name: "test do in transaction error commit",
			mock: func() {
				mockSqlW.
					On("Begin", context.Background()).
					Return(mockDBItx, nil).Once()

				mockDBItx.On("Commit",
					context.Background()).
					Return(errors.New("something happen")).Once()
			},
			args: args{
				db:  mockSqlW,
				ctx: context.Background(),
				callbackFunc: func(ctx context.Context, db SqlW) (interface{}, error) {
					return nil, nil
				},
			},
			wantErr: true,
		},
		{
			name: "test do in transaction commit - success",
			mock: func() {
				mockSqlW.
					On("Begin", context.Background()).
					Return(mockDBItx, nil).Once()

				mockDBItx.On("Commit",
					context.Background()).
					Return(errors.New("something happen")).Once()
			},
			args: args{
				db:  mockSqlW,
				ctx: context.Background(),
				callbackFunc: func(ctx context.Context, db SqlW) (interface{}, error) {
					return nil, nil
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()

			result, err := DoInTransaction(tt.args.ctx, tt.args.db, tt.args.callbackFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("DoInTransaction() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(result, tt.wantResult) {
				t.Errorf("DoInTransaction() want = %+v, got= %+v", tt.wantResult, result)
			}
		})
	}
}

func Test_sqlw_Begin(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	mockDBITx := dbitx.NewMockDBITx(t)

	type args struct {
		ctx context.Context
	}

	tests := []struct {
		mock    func()
		name    string
		args    args
		want    dbitx.DBITx
		wantErr bool
	}{
		{
			name: "failed to begin tx",
			args: args{
				ctx: context.Background(),
			},
			mock: func() {
				mockDBI.On("Begin", context.Background()).
					Return(nil,
						errors.New("something happen")).Once()
			},
			wantErr: true,
		},
		{
			name: "success to begin tx",
			args: args{
				ctx: context.Background(),
			},
			mock: func() {
				mockDBI.On("Begin", context.Background()).
					Return(mockDBITx, nil).Once()
			},
			want: mockDBITx,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db: mockDBI,
			}
			got, err := d.Begin(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Begin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Begin() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sqlw_BulkInsert(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	type args struct {
		ctx       context.Context
		tableName string
		fieldName []string
		values    [][]interface{}
	}
	tests := []struct {
		name    string
		args    args
		mock    func()
		wantErr bool
	}{
		{
			name: "error when do bulk insert",
			mock: func() {
				mockDBI.On("CopyFrom",
					context.Background(), pgx.Identifier{"table"}, []string{"name"}, pgx.CopyFromRows([][]interface{}{
						{"Andree"},
					})).Return(int64(0), errors.New("something happen")).Once()
			},
			args: args{
				ctx:       context.Background(),
				tableName: "table",
				fieldName: []string{"name"},
				values: [][]interface{}{
					{"Andree"},
				},
			},
			wantErr: true,
		},
		{
			name: "success when do bulk insert",
			mock: func() {
				mockDBI.On("CopyFrom",
					context.Background(), pgx.Identifier{"table"}, []string{"name"}, pgx.CopyFromRows([][]interface{}{
						{"Andree"},
					})).Return(int64(0), nil).Once()
			},
			args: args{
				ctx:       context.Background(),
				tableName: "table",
				fieldName: []string{"name"},
				values: [][]interface{}{
					{"Andree"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db: mockDBI,
			}
			if err := d.BulkInsert(tt.args.ctx, tt.args.tableName, tt.args.fieldName, tt.args.values); (err != nil) != tt.wantErr {
				t.Errorf("BulkInsert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sqlw_Exec(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	type args struct {
		ctx         context.Context
		queryString QueryString
		args        []interface{}
	}
	tests := []struct {
		mock    func()
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "error",
			args: args{
				ctx:         context.Background(),
				queryString: "insert INTO table (name) VALUES ($1)",
				args:        []interface{}{"Andree"},
			},
			mock: func() {
				mockDBI.On("Exec",
					context.Background(),
					"insert INTO table (name) VALUES ($1)",
					"Andree").
					Return(pgconn.CommandTag{}, errors.New("error here")).
					Once()
			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx:         context.Background(),
				queryString: "insert INTO table (name) VALUES ($1)",
				args:        []interface{}{"Andree"},
			},
			mock: func() {
				mockDBI.On("Exec",
					context.Background(),
					"insert INTO table (name) VALUES ($1)",
					"Andree").
					Return(pgconn.CommandTag{}, nil).
					Once()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db: mockDBI,
			}
			if err := d.Exec(tt.args.ctx, tt.args.queryString, tt.args.args...); (err != nil) != tt.wantErr {
				t.Errorf("Exec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sqlw_ExecNamed(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	type args struct {
		ctx         context.Context
		queryString QueryString
		param       QueryParam
	}
	tests := []struct {
		name    string
		args    args
		mock    func()
		wantErr bool
	}{
		{
			name: "error",
			mock: func() {
				mockDBI.On("Exec", context.Background(),
					"UPDATE table SET name=$1", "Andree").Return(pgconn.CommandTag{}, errors.New("something error")).Once()
			},
			args: args{
				ctx:         context.Background(),
				queryString: "UPDATE table SET name=:name",
				param: QueryParam{
					"name": "Andree",
				},
			},
			wantErr: true,
		},
		{
			name: "success",
			mock: func() {
				mockDBI.On("Exec", context.Background(),
					"UPDATE table SET name=$1", "Andree").Return(pgconn.CommandTag{}, nil).Once()
			},
			args: args{
				ctx:         context.Background(),
				queryString: "UPDATE table SET name=:name",
				param: QueryParam{
					"name": "Andree",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db: mockDBI,
			}
			if err := d.ExecNamed(tt.args.ctx, tt.args.queryString, tt.args.param); (err != nil) != tt.wantErr {
				t.Errorf("ExecNamed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sqlw_Get(t *testing.T) {
	mockDbi := dbi.NewMockDBI(t)
	mockRowsI := rows.NewMockRowsI(t)
	mockConverter := converter.NewMockConverter(t)
	type args struct {
		ctx         context.Context
		dest        interface{}
		queryString QueryString
		args        []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		mock    func()
	}{
		{
			name: "error",
			args: args{
				ctx:         context.Background(),
				dest:        nil,
				queryString: "SELECT * FROM table where name=$1",
				args:        []interface{}{"Andree"},
			},
			mock: func() {
				mockDbi.On("Query", context.Background(),
					"SELECT * FROM table where name=$1", "Andree").
					Return(mockRowsI, errors.New("something happen")).Once()
			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx:         context.Background(),
				dest:        nil,
				queryString: "SELECT * FROM table where name=$1",
				args:        []interface{}{"Andree"},
			},
			mock: func() {
				mockDbi.On("Query", context.Background(),
					"SELECT * FROM table where name=$1", "Andree").
					Return(mockRowsI, nil).Once()
				mockConverter.On("ConvertRows", mockRowsI, "db", nil).
					Return(nil).Once()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db:        mockDbi,
				converter: mockConverter,
			}
			if err := d.Get(tt.args.ctx, tt.args.dest, tt.args.queryString, tt.args.args...); (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sqlw_GetNamed(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	mockRowsI := rows.NewMockRowsI(t)
	mockConverter := converter.NewMockConverter(t)
	type args struct {
		ctx         context.Context
		dest        interface{}
		queryString QueryString
		param       QueryParam
	}
	tests := []struct {
		name    string
		mock    func()
		args    args
		wantErr bool
	}{
		{
			name: "error",
			args: args{
				ctx:         context.Background(),
				dest:        nil,
				queryString: "SELECT * FROM table where name=:name",
				param: QueryParam{
					"name": "Andre",
				},
			},
			mock: func() {
				mockDBI.On("Query",
					context.Background(),
					"SELECT * FROM table where name=$1",
					"Andre",
				).Return(mockRowsI, errors.New("something happen")).Once()

			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx:         context.Background(),
				dest:        nil,
				queryString: "SELECT * FROM table where name=:name",
				param: QueryParam{
					"name": "Andre",
				},
			},
			mock: func() {
				mockDBI.On("Query",
					context.Background(),
					"SELECT * FROM table where name=$1",
					"Andre",
				).Return(mockRowsI, nil).Once()

				mockConverter.On("ConvertRows", mockRowsI, "db", nil).
					Return(nil).Once()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db:        mockDBI,
				converter: mockConverter,
			}
			if err := d.GetNamed(tt.args.ctx, tt.args.dest, tt.args.queryString, tt.args.param); (err != nil) != tt.wantErr {
				t.Errorf("GetNamed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sqlw_Select(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	mockRowsI := rows.NewMockRowsI(t)
	mockConveter := converter.NewMockConverter(t)
	type args struct {
		ctx         context.Context
		dest        interface{}
		queryString QueryString
		args        []interface{}
	}
	tests := []struct {
		name    string
		args    args
		mock    func()
		wantErr bool
	}{
		{
			name: "error",
			args: args{
				ctx:         context.Background(),
				queryString: "SELECT * FROM table where name=$1",
				args:        []interface{}{"Andree"},
			},
			mock: func() {
				mockDBI.On("Query", context.Background(),
					"SELECT * FROM table where name=$1", "Andree").
					Return(mockRowsI, errors.New("error")).Once()
			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx:         context.Background(),
				queryString: "SELECT * FROM table where name=$1",
				args:        []interface{}{"Andree"},
			},
			mock: func() {
				mockDBI.On("Query", context.Background(),
					"SELECT * FROM table where name=$1", "Andree").
					Return(mockRowsI, nil).Once()
				mockConveter.On("ConvertRows", mockRowsI, "db", nil).
					Return(nil).Once()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db:        mockDBI,
				converter: mockConveter,
			}
			if err := d.Select(tt.args.ctx, tt.args.dest, tt.args.queryString, tt.args.args...); (err != nil) != tt.wantErr {
				t.Errorf("Select() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sqlw_SelectNamed(t *testing.T) {
	mockDBI := dbi.NewMockDBI(t)
	mockRowsI := rows.NewMockRowsI(t)
	mockConverter := converter.NewMockConverter(t)

	type args struct {
		ctx         context.Context
		dest        interface{}
		queryString QueryString
		param       QueryParam
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		mock    func()
	}{
		{
			name: "error",
			args: args{
				ctx: context.Background(),
				param: QueryParam{
					"name": "Andree",
				},
				queryString: "SELECT * FROM table WHERE name=:name",
			},
			mock: func() {
				mockDBI.On("Query", context.Background(),
					"SELECT * FROM table WHERE name=$1", "Andree").
					Return(mockRowsI, errors.New("something error")).Once()
			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				ctx: context.Background(),
				param: QueryParam{
					"name": "Andree",
				},
				queryString: "SELECT * FROM table WHERE name=:name",
			},
			mock: func() {
				mockDBI.On("Query", context.Background(),
					"SELECT * FROM table WHERE name=$1", "Andree").
					Return(mockRowsI, nil).Once()

				mockConverter.On("ConvertRows", mockRowsI, "db", nil).
					Return(nil).Once()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			d := sqlw{
				db:        mockDBI,
				converter: mockConverter,
			}
			if err := d.SelectNamed(tt.args.ctx, tt.args.dest, tt.args.queryString, tt.args.param); (err != nil) != tt.wantErr {
				t.Errorf("SelectNamed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
