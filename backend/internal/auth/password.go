package auth

import "github.com/alexedwards/argon2id"

// hashPassword aplica Argon2id com os parâmetros padrão da lib
// (64MB, 1 iteração, 4 threads — revisar conforme o hardware de produção).
func hashPassword(plain string) (string, error) {
	return argon2id.CreateHash(plain, argon2id.DefaultParams)
}

// verifyPassword compara em tempo constante.
func verifyPassword(plain, hash string) bool {
	ok, err := argon2id.ComparePasswordAndHash(plain, hash)
	return err == nil && ok
}
