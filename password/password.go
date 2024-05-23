package password

import (
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// HashAndSalt return hashed password
func HashAndSalt(pwd []byte) (string, error) {

	// generate hash with rounds of 10
	hash, err := bcrypt.GenerateFromPassword(pwd, 10)
	if err != nil {
		return "", errors.Wrap(err, "cannot generate hash")
	}
	return string(hash), nil
}

// ComparePasswords compares between hashed password and plain password
func ComparePasswords(hashedPwd string, plainPwd []byte) bool {
	// Since we'll be getting the hashed password from the DB it
	// will be a string so we'll need to convert it to a byte slice
	byteHash := []byte(hashedPwd)
	err := bcrypt.CompareHashAndPassword(byteHash, plainPwd)
	return err == nil
}
