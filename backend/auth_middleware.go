// Copyright (c) 2026 TTBT Enterprises LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/c2FmZQ/tlsproxy/jwks"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-retryablehttp"
)

type jwksLogger struct{}

func (l jwksLogger) Errorf(format string, args ...any) {
	log.Printf("JWKS Error: "+format, args...)
}

// jwtAuthMiddleware handles JWT authentication using JWKS.
func jwtAuthMiddleware(opts Options, next http.Handler) http.Handler {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10
	retryClient.Logger = nil // Disable verbose logging

	remote := jwks.NewRemote(retryClient, jwksLogger{})

	if opts.AuthJWKSURL != "" {
		issuers := parseJWKSConfig(opts.AuthJWKSURL)
		remote.SetIssuers(issuers)
	} else {
		log.Println("Warning: No AuthJWKSURL provided. JWT validation will fail unless MockAuth is used.")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote.Ready(r.Context())

		// 1. Extract JWT from cookie
		cookieName := opts.AuthCookieName
		if cookieName == "" {
			cookieName = "skorekeeper_auth"
		}
		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value == "" {
			// No token provided, proceed as anonymous
			next.ServeHTTP(w, r)
			return
		}

		tokenString := cookie.Value
		audience := fmt.Sprintf("https://%s/", r.Host)

		// 2. Parse and Validate JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate algorithm
			switch token.Method.(type) {
			case *jwt.SigningMethodRSA, *jwt.SigningMethodECDSA, *jwt.SigningMethodEd25519:
				// Allowed
			default:
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("token missing 'kid' header")
			}

			return remote.GetKey(kid)
		}, jwt.WithAudience(audience))

		if err == nil && token.Valid {
			// 3. Verify Issuer if configured for this key
			kid, _ := token.Header["kid"].(string)
			if expectedIss, ok := remote.IssuerForKey(kid); ok && expectedIss != "" {
				iss, err := token.Claims.GetIssuer()
				if err != nil || iss != expectedIss {
					if opts.Debug {
						if err != nil {
							log.Printf("JWT Validation failed: token is missing issuer claim (or it's invalid), but expected '%s' for key %s. err: %v", expectedIss, kid, err)
						} else {
							log.Printf("JWT Validation failed: issuer mismatch. Got '%s', expected '%s'", iss, expectedIss)
						}
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// 4. Extract Claims and Set Context
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if email, ok := claims["email"].(string); ok && email != "" {
					ctx := context.WithValue(r.Context(), userIDKey, normalizeEmail(email))
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		} else {
			if opts.Debug {
				log.Printf("JWT Validation failed: %v", err)
			}
		}

		// Anonymous
		next.ServeHTTP(w, r)
	})
}

// parseJWKSConfig parses the config string into jwks.Issuer slice.
// Config format: "ISSUER=URL,URL,ISSUER=URL"
func parseJWKSConfig(config string) []jwks.Issuer {
	var issuers []jwks.Issuer
	if config == "" {
		return issuers
	}

	parts := strings.Split(config, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var issuer, url string
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			issuer = strings.TrimSpace(kv[0])
			url = strings.TrimSpace(kv[1])
		} else {
			url = part
		}

		issuers = append(issuers, jwks.Issuer{
			Issuer:  issuer,
			JWKSURI: url,
		})
	}
	return issuers
}
