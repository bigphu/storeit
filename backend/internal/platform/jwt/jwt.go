package jwt

import (
	"errors"
	"fmt"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	typeHeader     = "at+jwt"
	signingAlg     = "HS256"
	leewayDuration = 30 * time.Second
)

type Provider struct {
	ring     keyring
	issuer   string
	audience string
	ttl      time.Duration
	parser   *gojwt.Parser
}

type Token struct {
	Value     string
	ID        uuid.UUID
	ExpiresAt time.Time
}

func New(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ring, err := parseKeys(cfg.Keys, cfg.ActiveKID)
	if err != nil {
		return nil, fmt.Errorf("jwt: %w", err)
	}

	return &Provider{
		ring:     ring,
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.AccessTokenTTL,

		parser: gojwt.NewParser(
			gojwt.WithValidMethods([]string{signingAlg}),
			gojwt.WithIssuer(cfg.Issuer),
			gojwt.WithAudience(cfg.Audience),
			gojwt.WithExpirationRequired(),
			gojwt.WithLeeway(leewayDuration),
		),
	}, nil
}

// Issue phát access token cho accountID, mang theo permissions trong claim perms
func (p *Provider) Issue(accountID uuid.UUID, permissions []string) (Token, error) {
	jti, err := uuid.NewRandom()
	if err != nil {
		return Token{}, fmt.Errorf("jwt: create jti: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(p.ttl)

	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    p.issuer,
			Subject:   accountID.String(),
			Audience:  gojwt.ClaimStrings{p.audience},
			ExpiresAt: gojwt.NewNumericDate(expiresAt),
			NotBefore: gojwt.NewNumericDate(now),
			IssuedAt:  gojwt.NewNumericDate(now),
			ID:        jti.String(),
		},
		Permissions: permissions,
	}

	kid, secret := p.ring.active()

	tok := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	tok.Header["kid"] = kid
	tok.Header["typ"] = typeHeader

	signed, err := tok.SignedString(secret)
	if err != nil {
		return Token{}, fmt.Errorf("jwt: signing token: %w", err)
	}

	return Token{
		Value:     signed,
		ID:        jti,
		ExpiresAt: expiresAt,
	}, nil
}

func (p *Provider) Verify(raw string) (*Claims, error) {
	var claims Claims

	tok, err := p.parser.ParseWithClaims(raw, &claims, p.keyfunc)
	if err != nil {
		return nil, translate(err)
	}

	if typ, _ := tok.Header["typ"].(string); !strings.EqualFold(typ, typeHeader) {
		return nil, fmt.Errorf("%w: %q", ErrBadType, typ)
	}

	return &claims, nil
}

func (p *Provider) keyfunc(tok *gojwt.Token) (any, error) {
	if _, ok := tok.Method.(*gojwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("%w: %s", ErrBadAlgorithm, tok.Method.Alg())
	}

	kid, _ := tok.Header["kid"].(string)
	secret, ok := p.ring.lookup(kid)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKID, kid)
	}
	return secret, nil
}

func translate(err error) error {
	switch {
	case errors.Is(err, ErrUnknownKID), errors.Is(err, ErrBadAlgorithm):
		return err
	case errors.Is(err, gojwt.ErrTokenMalformed):
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	case errors.Is(err, gojwt.ErrTokenSignatureInvalid):
		return fmt.Errorf("%w: %v", ErrSignature, err)
	case errors.Is(err, gojwt.ErrTokenExpired):
		return fmt.Errorf("%w: %v", ErrExpired, err)
	case errors.Is(err, gojwt.ErrTokenNotValidYet):
		return fmt.Errorf("%w: %v", ErrNotYetValid, err)
	default:
		// iss/aud sai, claim bắt buộc thiếu, v.v.
		return fmt.Errorf("%w: %v", ErrClaims, err)
	}
}
