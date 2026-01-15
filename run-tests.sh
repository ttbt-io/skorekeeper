#!/bin/bash
# Copyright (c) 2026 TTBT Enterprises LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e # Exit immediately if a command exits with a non-zero status.

echo "--- Running Skorekeeper Test Suite ---"

echo "[1/3] Running Frontend Checks (Lint + Unit)..."
./tests/run-js-checks.sh

# Ensure dist directory exists for embedding (required by go vet/build)
mkdir -p frontend/dist
touch frontend/dist/.keep

echo "[2/3] Running Go Checks (Vet + Unit)..."
go vet ./...
go fmt ./...
go test -timeout=2m -failfast ./...

echo "[3/3] Running E2E Headless Tests..."
./tests/e2e/run-headless-tests.sh "$@"

echo "--- All Tests Passed! ---"
