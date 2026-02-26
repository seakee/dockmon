// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package jwt provides helpers for generating and parsing server app JWT tokens.
package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seakee/dockmon/app"
	"github.com/seakee/dockmon/app/model/auth"
)

type ServerClaims struct {
	ID      uint   `json:"id"`
	AppName string `json:"app_name"`
	AppID   string `json:"app_id"`
	jwt.RegisteredClaims
}

// GenerateAppToken creates a signed JWT for a server app.
//
// Parameters:
//   - App: authenticated app entity used to fill token claims.
//   - expireTime: token expiration duration in seconds.
//
// Returns:
//   - token: signed JWT string.
//   - err: signing error.
//
// Example:
//
//	token, err := jwt.GenerateAppToken(appEntity, 3600)
func GenerateAppToken(App *auth.App, expireTime time.Duration) (token string, err error) {
	expTime := time.Now().Add(expireTime * time.Second)
	claims := ServerClaims{
		ID:      App.ID,
		AppName: App.AppName,
		AppID:   App.AppID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "dockmon",
		},
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := []byte(app.GetConfig().System.JwtSecret)

	return tokenClaims.SignedString(jwtSecret)
}

// ParseAppAuth parses and validates a server app JWT token.
//
// Parameters:
//   - token: JWT string from request authorization header.
//
// Returns:
//   - *ServerClaims: parsed claims when token is valid.
//   - error: parsing or signature validation error.
func ParseAppAuth(token string) (*ServerClaims, error) {
	jwtSecret := []byte(app.GetConfig().System.JwtSecret)

	tokenClaims, err := jwt.ParseWithClaims(token, &ServerClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if tokenClaims != nil {
		if claims, ok := tokenClaims.Claims.(*ServerClaims); ok && tokenClaims.Valid {
			return claims, nil
		}
	}

	return nil, err
}
