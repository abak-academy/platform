package infra

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Sub          string   `json:"sub"`
	Role         string   `json:"role"`
	SchoolID     *string  `json:"school_id,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	// MustChange marks credentials an admin issued; the JWT middleware
	// confines such tokens to the password-change flow.
	MustChange bool `json:"must_change_password,omitempty"`
	jwt.RegisteredClaims
}

type JWTSigner struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTSigner(secret string, ttl time.Duration) *JWTSigner {
	return &JWTSigner{secret: []byte(secret), ttl: ttl}
}

// SignAccess mints an access token. The optional mustChange flag (variadic so
// existing call sites are unchanged) marks admin-issued credentials that must
// rotate at first login.
func (s *JWTSigner) SignAccess(sub, role string, schoolID *string, capabilities []string, mustChange ...bool) (tokenString, jti string, err error) {
	jti = fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now()
	var flag bool
	if len(mustChange) > 0 {
		flag = mustChange[0]
	}
	claims := Claims{
		Sub:          sub,
		Role:         role,
		SchoolID:     schoolID,
		Capabilities: capabilities,
		MustChange:   flag,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err = token.SignedString(s.secret)
	return tokenString, jti, err
}

func (s *JWTSigner) ParseAccess(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
