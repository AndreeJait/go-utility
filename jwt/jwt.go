package jwt

import (
	"encoding/json"
	"github.com/AndreeJait/go-utility/errow"
	"github.com/golang-jwt/jwt/v4"
)

func CreateToken(param CreateTokenRequest) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256, param.Claims)
	tokenSigned, err := token.SignedString([]byte(param.SecretToken))
	return tokenSigned, err
}

func ParseToken[T interface{}](tokenStr string, secret string) (resp T, err error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if method, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errow.ErrInvalidSigningMethod
		} else if method != jwt.SigningMethodHS256 {
			return nil, errow.ErrInvalidSigningMethod
		}

		return []byte(secret), nil
	})
	if err != nil {
		return resp, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return resp, errow.ErrInvalidToken
	}

	if data, ok := claims["data"]; ok {
		b, err := json.Marshal(data)
		if err != nil {
			return resp, err
		}
		if err := json.Unmarshal(b, &resp); err != nil {
			return resp, err
		}
	}
	return resp, nil
}
