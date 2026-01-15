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
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// jwtAuthMiddleware handles JWT authentication using JWKS.
func jwtAuthMiddleware(opts Options, next http.Handler) http.Handler {
	var (
		keys        jwk.Set
		lastRefresh time.Time
		mu          sync.RWMutex
	)

	// refreshKeys fetches the JWKS from the URL.
	refreshKeys := func() error {
		if opts.AuthJWKSURL == "" {
			return fmt.Errorf("no JWKS URL provided")
		}
		// Context with timeout for fetch
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		set, err := jwk.Fetch(ctx, opts.AuthJWKSURL)
		if err != nil {
			return fmt.Errorf("failed to fetch JWKS: %w", err)
		}

		mu.Lock()
		keys = set
		lastRefresh = time.Now()
		mu.Unlock()
		return nil
	}

	// Initial fetch attempt (non-fatal if it fails, will retry on request)
	if opts.AuthJWKSURL != "" {
		if err := refreshKeys(); err != nil {
			log.Printf("Warning: Failed to fetch JWKS on startup: %v", err)
		}
	} else {
		log.Println("Warning: No AuthJWKSURL provided. JWT validation will fail unless MockAuth is used.")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 2. Extract JWT from cookie
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

		// 3. Parse and Verify Token
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

			mu.RLock()
			localKeys := keys
			localLastRefresh := lastRefresh
			mu.RUnlock()

			// Helper to find key in set
			findKey := func(set jwk.Set, id string) (interface{}, error) {
				if set == nil {
					return nil, fmt.Errorf("JWKS not initialized")
				}
				key, ok := set.LookupKeyID(id)
				if !ok {
					return nil, fmt.Errorf("key %s not found in JWKS", id)
				}
				var raw interface{}
				if err := jwk.Export(key, &raw); err != nil {
					return nil, fmt.Errorf("failed to materialize key: %w", err)
				}
				return raw, nil
			}

			// Try with current keys
			key, err := findKey(localKeys, kid)
			if err == nil {
				return key, nil
			}

			// If not found or keys stale, try refreshing (only if enough time passed to avoid thundering herd on JWKS endpoint)
			if time.Since(localLastRefresh) > 1*time.Minute {
				// Release read lock before acquiring write lock in refreshKeys
				if err := refreshKeys(); err != nil {
					log.Printf("Error refreshing JWKS: %v", err)
					return nil, err
				}
				mu.RLock()
				localKeys = keys
				mu.RUnlock()
				return findKey(localKeys, kid)
			}

			return nil, err
		})

		if err != nil {
			// Invalid token (expired, bad sig, etc.) -> Anonymous
			// We log generic validation errors at debug level to avoid log spam from random probes
			if opts.Debug {
				log.Printf("JWT Validation failed: %v", err)
			}
			next.ServeHTTP(w, r)
			return
		}

		if !token.Valid {
			next.ServeHTTP(w, r)
			return
		}

		// 4. Extract Claims and Set Context
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if email, ok := claims["email"].(string); ok && email != "" {
				ctx := context.WithValue(r.Context(), userIDKey, normalizeEmail(email))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
