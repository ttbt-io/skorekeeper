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
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// jwtAuthMiddleware handles JWT authentication using JWKS.
func jwtAuthMiddleware(opts Options, next http.Handler) http.Handler {
	km := NewKeyManager(opts.AuthJWKSURL)
	if opts.AuthJWKSURL != "" {
		// Initial fetch
		km.RefreshAll()
		// Start background refresh
		go km.StartBackgroundRefresh(context.Background())
	} else {
		log.Println("Warning: No AuthJWKSURL provided. JWT validation will fail unless MockAuth is used.")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// 2. Iterate through providers to find a valid match
		// We loop because we can't extract the unverified issuer claim to verify the issuer.
		// Instead, we try each provider. If a provider has an issuer configured, we enforce it.
		// Only the provider that actually holds the signing key (check via kid) will succeed in the KeyFunc.
		km.mu.RLock()
		defer km.mu.RUnlock()

		for _, provider := range km.Providers {
			var parseOpts []jwt.ParserOption
			parseOpts = append(parseOpts, jwt.WithAudience(audience))
			if provider.Issuer != "" {
				parseOpts = append(parseOpts, jwt.WithIssuer(provider.Issuer))
			}

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

				// Look up key ONLY in this provider's set
				key, ok := provider.Keys.LookupKeyID(kid)
				if !ok {
					return nil, fmt.Errorf("key %s not found in provider %s", kid, provider.URL)
				}
				var raw interface{}
				if err := jwk.Export(key, &raw); err != nil {
					return nil, fmt.Errorf("failed to materialize key: %w", err)
				}
				return raw, nil
			}, parseOpts...)

			if err == nil && token.Valid {
				// 3. Extract Claims and Set Context
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					if email, ok := claims["email"].(string); ok && email != "" {
						ctx := context.WithValue(r.Context(), userIDKey, normalizeEmail(email))
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			} else {
				if opts.Debug {
					log.Printf("JWT Validation failed for provider %s: %v", provider.URL, err)
				}
			}
		}

		// No provider validated the token -> Anonymous
		next.ServeHTTP(w, r)
	})
}

// JWKSProvider represents a single JWKS endpoint configuration.
type JWKSProvider struct {
	Issuer string
	URL    string
	Keys   jwk.Set
}

// KeyManager manages multiple JWKS providers.
type KeyManager struct {
	Providers []*JWKSProvider
	mu        sync.RWMutex
}

// NewKeyManager parses the config string and initializes the manager.
// Config format: "ISSUER=URL,URL,ISSUER=URL"
func NewKeyManager(config string) *KeyManager {
	km := &KeyManager{}
	if config == "" {
		return km
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

		km.Providers = append(km.Providers, &JWKSProvider{
			Issuer: issuer,
			URL:    url,
			Keys:   jwk.NewSet(),
		})
	}
	return km
}

// RefreshAll fetches keys for all providers.
func (km *KeyManager) RefreshAll() {
	// We assume km.Providers is immutable after initialization, so we can iterate without lock.
	// If Providers list were dynamic, we'd need to RLock, copy slice, RUnlock, then iterate.
	var wg sync.WaitGroup
	for _, p := range km.Providers {
		// Use wg.Go() (Go 1.25+) to handle Add/Done automatically
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			set, err := jwk.Fetch(ctx, p.URL)
			if err != nil {
				log.Printf("Failed to fetch JWKS from %s: %v", p.URL, err)
				return
			}

			// Only lock when updating the shared state
			km.mu.Lock()
			p.Keys = set
			km.mu.Unlock()
		})
	}
	wg.Wait()
}

// StartBackgroundRefresh runs a periodic refresh loop.
func (km *KeyManager) StartBackgroundRefresh(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			km.RefreshAll()
		}
	}
}
