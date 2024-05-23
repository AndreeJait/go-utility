package jwt

import (
	"github.com/AndreeJait/go-utility/errow"
	"github.com/golang-jwt/jwt/v4"
)

func CreateToken(param CreateTokenRequest) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256, param.Claims)
	tokenSigned, err := token.SignedString([]byte(param.SecretToken))
	return tokenSigned, err
}

func ParseToken[T interface{}](tokenStr string, secret string) (claims jwt.MapClaims, err error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if method, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errow.ErrInvalidSigningMethod
		} else if method != jwt.SigningMethodHS256 {
			return nil, errow.ErrInvalidSigningMethod
		}

		return []byte(secret), nil
	})

	if err != nil {
		return claims, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return claims, errow.ErrInvalidToken
	}
	return claims, nil
}
