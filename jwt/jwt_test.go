package jwt

import (
	"fmt"
	"github.com/AndreeJait/go-utility/timew"
	"testing"
	"time"
)

type user struct {
	ID       string
	Username string
}

func (u *user) GetUserID() string {
	return u.ID
}
func (u *user) GetUsername() string {
	return u.Username
}
func (u *user) GetPassword() string {
	return ""
}

func TestCreateToken(t *testing.T) {
	type args[T string] struct {
		param CreateTokenRequest[T]
	}
	type testCase[T string] struct {
		name    string
		args    args[T]
		want    string
		want1   time.Time
		wantErr bool
	}
	tests := []testCase[string]{
		{
			name: "success get token",
			args: args[string]{
				param: CreateTokenRequest[string]{
					SecretToken:     "andree",
					ExpiredDuration: 1 * time.Minute,
					TimeW: timew.New(timew.
						LoadLocation("Asia/Jakarta")),
					ServiceName: "testing",
					Identify: &user{
						ID:       "testing-Andre",
						Username: "andree",
					},
				},
			},
			wantErr: false,
			want1: timew.New(timew.
				LoadLocation("Asia/Jakarta")).Now(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := CreateToken(tt.args.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got)
			if got1.Format(time.DateOnly) != tt.want1.Format(time.DateOnly) {
				t.Errorf("CreateToken() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
