package row

//go:generate mockery --name=RowI --filename=mock_RowI.go --inpackage
type RowI interface {
	Scan(dest ...any) error
}
