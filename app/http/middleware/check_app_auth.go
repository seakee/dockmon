// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/seakee/dockmon/app/pkg/e"
	apiJWT "github.com/seakee/dockmon/app/pkg/jwt"
)

// CheckAppAuth returns middleware that validates Authorization tokens.
//
// Returns:
//   - gin.HandlerFunc: middleware that aborts unauthorized requests.
//
// Behavior:
//   - Parses and verifies JWT from the Authorization header.
//   - Writes localized error response and aborts request on failure.
func (m middleware) CheckAppAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		errCode, err := checkByToken(c)
		if errCode != e.SUCCESS {
			m.i18n.JSON(c, errCode, nil, err)
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkByToken validates a JWT token and injects app metadata into Gin context.
//
// Parameters:
//   - c: current Gin context carrying HTTP headers.
//
// Returns:
//   - errCode: application-level error code.
//   - err: parsing or validation error, nil on success.
//
// Example:
//
//	errCode, err := checkByToken(c)
func checkByToken(c *gin.Context) (errCode int, err error) {
	errCode = e.InvalidParams

	token := c.Request.Header.Get("Authorization")
	if token != "" {
		var serverClaims *apiJWT.ServerClaims

		errCode = e.SUCCESS

		serverClaims, err = apiJWT.ParseAppAuth(token)
		if err != nil {
			// Convert JWT library errors into project-specific error codes.
			switch err {
			case jwt.ErrTokenExpired:
				errCode = e.ServerAuthorizationExpired
			default:
				errCode = e.ServerUnauthorized
			}
		} else {
			// Cache app identity in context for downstream handlers.
			c.Set("app_id", serverClaims.AppID)
			c.Set("app_name", serverClaims.AppName)
		}
	}

	return
}
