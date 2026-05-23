package identity

import (
	"time"

	"ai-platform/internal/model/dto"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
	jwtv5.RegisteredClaims
}

func generateJWT(secret []byte, expireHours int, c dto.TokenClaims) (string, error) {
	claims := &jwtClaims{
		UserID:   c.UserID,
		Username: c.Username,
		Role:     c.Role,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwtv5.NewNumericDate(time.Now()),
		},
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func parseJWT(secret []byte, tokenStr string) (*dto.TokenClaims, error) {
	token, err := jwtv5.ParseWithClaims(tokenStr, &jwtClaims{},
		func(t *jwtv5.Token) (any, error) { return secret, nil },
	)
	if err != nil || !token.Valid {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok {
		return nil, jwtv5.ErrSignatureInvalid
	}
	return &dto.TokenClaims{
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
