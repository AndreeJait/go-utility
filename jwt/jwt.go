package jwt

import (
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
