package jwt

import "errors"

var (
	ErrMalformed    = errors.New("jwt: cannot read token")
	ErrSignature    = errors.New("jwt: invalid signature")
	ErrExpired      = errors.New("jwt: token expired")
	ErrNotYetValid  = errors.New("jwt: token is not yet valid")
	ErrClaims       = errors.New("jwt: invalid claim")
	ErrUnknownKID   = errors.New("jwt: keyring does not hold kid")
	ErrBadAlgorithm = errors.New("jwt: signing algorithm not allowed")
	ErrBadType      = errors.New("jwt: header typ is not at+jwt")
)
