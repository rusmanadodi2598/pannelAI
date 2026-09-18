// Package service orchestrates app-serv use cases without HTTP concerns.
//
// @file      internal/service/password.go
// @for       Provides the production bcrypt password hashing adapter.
// @uses      golang.org/x/crypto/bcrypt.
// @reason    Dashboard credentials require an adaptive one-way hash rather
//
//	than reversible storage or a process-local comparison.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package service

import "golang.org/x/crypto/bcrypt"

// PasswordHasher abstracts password hashing for service tests and production.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// BcryptHasher implements PasswordHasher using bcrypt's adaptive work factor.
type BcryptHasher struct{}

// Hash returns a bcrypt password hash.
func (BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// Compare checks a plaintext password against a stored bcrypt hash.
func (BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
