package jwt

import (
	"fmt"
	"github.com/AndreeJait/go-utility/timew"
	"github.com/golang-jwt/jwt/v4"
	"testing"
	"time"
)

type user struct {
	ID       string `json:"id"`
	Username string `json:"username"`
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
	type args struct {
		param CreateTokenRequest
	}
	type testCase struct {
		name    string
		args    args
		want    string
		want1   time.Time
		wantErr bool
	}
	tests := []testCase{
		{
			name: "success get token",
			args: args{
				param: CreateTokenRequest{
					SecretToken: "andree",
					Claims: MyClaims[string]{
						Claims: jwt.RegisteredClaims{
							Issuer:    "testing",
							ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Minute)),
						},
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
			got, err := CreateToken(tt.args.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got)
		})
	}
}

func TestParseToken(t *testing.T) {
	type args struct {
		tokenStr string
		secret   string
	}
	type testCase[T interface{}] struct {
		name    string
		args    args
		want    T
		wantErr bool
	}

	var secretToken = "andree"

	tokenGenerate, _ := CreateToken(CreateTokenRequest{
		SecretToken: secretToken,
		Claims: MyClaims[user]{
			Data: user{
				ID:       "123",
				Username: "andree",
			},
			Claims: jwt.RegisteredClaims{
				Issuer:    "testing",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Minute)),
			},
		},
	})
	tests := []testCase[user]{
		{
			name: "success to parse token",
			args: args{
				tokenStr: tokenGenerate,
				secret:   secretToken,
			},
			want: user{
				ID:       "123",
				Username: "andree",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClaims, err := ParseToken[user](tt.args.tokenStr, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != gotClaims {
				t.Errorf("ParseToken() gotClaims = %v, want %v", gotClaims, tt.want)
			}
		})
	}
}
