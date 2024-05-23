package password

import (
	"fmt"
	"testing"
)

func TestHashAndSalt(t *testing.T) {
	type args struct {
		pwd []byte
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "success to hash password",
			args: args{
				pwd: []byte("andre110102"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HashAndSalt(tt.args.pwd)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashAndSalt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got)
		})
	}
}

func TestComparePasswords(t *testing.T) {
	type args struct {
		hashedPwd string
		plainPwd  []byte
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "success to verify",
			args: args{
				hashedPwd: "$2a$10$YEKbVIkCGKDGnCd5aIZceeycpstta7h067Oa1wX5SW4ztgzg20JvK",
				plainPwd:  []byte("andre110102"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComparePasswords(tt.args.hashedPwd, tt.args.plainPwd); got != tt.want {
				t.Errorf("ComparePasswords() = %v, want %v", got, tt.want)
			}
		})
	}
}
