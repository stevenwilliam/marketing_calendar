package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrTokenInvalid = errors.New("token tidak sah")
	ErrTokenExpired = errors.New("token kedaluwarsa")
)

// Claims is the access token. It carries the user and nothing that can go
// stale dangerously — permissions are resolved per request from the database,
// so revoking a role takes effect on the next request rather than in fifteen
// minutes.
type Claims struct {
	UserID uuid.UUID `json:"uid"`
	JTI    string    `json:"jti"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl}
}

func (t *TokenIssuer) Issue(userID uuid.UUID, now time.Time) (token string, jti string, expiresAt time.Time, err error) {
	jti = uuid.NewString()
	expiresAt = now.Add(t.ttl)
	c := Claims{
		UserID: userID,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ID:        jti,
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	return s, jti, expiresAt, err
}

// Parse pins the algorithm to HS256. Without the pin, `alg: none` and
// algorithm confusion (an RS256 token verified with the public key as an HMAC
// secret) both authenticate anybody.
func (t *TokenIssuer) Parse(token string) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if c.UserID == uuid.Nil {
		return nil, ErrTokenInvalid
	}
	return &c, nil
}
