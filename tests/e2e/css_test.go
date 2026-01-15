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

package e2e

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
)

func TestCSSContentType(t *testing.T) {
	baseURL := startTestServer(t)
	localURL := strings.Replace(baseURL, "devtest.local", "localhost", 1)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/", "text/html; charset=utf-8"},
		{"/index.html", "text/html; charset=utf-8"},
		{"/css/style.css", "text/css; charset=utf-8"},
		{"/init.js", "application/javascript"},
		{"/.sso/proxy.mjs", "application/javascript"},
	} {
		resp, err := client.Get(localURL + tc.path)
		if err != nil {
			t.Fatalf("Failed to fetch %q: %v", tc.path, err)
		}
		resp.Body.Close()

		if got := resp.Header.Get("Content-Type"); got != tc.want {
			t.Errorf("Content-Type for %q = %q, want %q", tc.path, got, tc.want)
		}
	}
}
