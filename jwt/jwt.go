package jwt

import (
	"github.com/AndreeJait/go-utility/errow"
	"github.com/golang-jwt/jwt/v4"
	"time"
)

func CreateToken[T interface{}](param CreateTokenRequest[T]) (string, time.Time, error) {
	timeExpired := param.TimeW.Now()
	timeExpired = timeExpired.Add(param.ExpiredDuration)
	claims := MyClaims[T]{
		Claims: jwt.RegisteredClaims{
			Issuer:    param.ServiceName,
			ExpiresAt: jwt.NewNumericDate(timeExpired),
		},
		UserID:   param.Identify.GetUserID(),
		Username: param.Identify.GetUsername(),
	}
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256, claims)

	tokenSigned, err := token.SignedString([]byte(param.SecretToken))
	return tokenSigned, timeExpired, err
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
