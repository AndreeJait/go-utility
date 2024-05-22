package refactor

import "testing"

func TestDoRefactor(t *testing.T) {
	type args struct {
		beforePath string
		afterPath  string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success",
			args: args{
				beforePath: "Andree2",
				afterPath:  "Andree2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DoRefactor(tt.args.beforePath, tt.args.afterPath)
		})
	}
}
