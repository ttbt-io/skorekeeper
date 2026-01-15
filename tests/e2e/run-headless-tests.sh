#!/bin/bash -e
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

# This script runs the browser tests with chromedp in docker containers.

# Go to project root
cd $(dirname $0)/../..

# Minify assets (always run to ensure dist/ exists for embedding)
if [ -f "tools/minify.mjs" ]; then
  node tools/minify.mjs
fi

export CGO_ENABLED=0

# Build the test binary
# Package path is now .../tests/e2e
go test -c -o tests/e2e/test-app github.com/ttbt-io/skorekeeper/tests/e2e

# Build docker image
docker build -f tests/e2e/Dockerfile -t skorekeeper-app .

# Cleanup binary
rm -f tests/e2e/test-app

# Create demo directory for screenshots
mkdir -p demo

# Parse args
TEST_RUN=""
UPDATE_GOLDENS="false"
MINIFY_BOOL="false"

for arg in "$@"; do
  if [ "$arg" == "-update-goldens" ]; then
    UPDATE_GOLDENS="true"
  elif [ "$arg" == "-minify" ]; then
    MINIFY_BOOL="true"
  elif [ -z "$TEST_RUN" ]; then
    TEST_RUN="$arg"
  else
    TEST_RUN="${TEST_RUN}|$arg"
  fi
done

# Run tests via docker-compose
docker rm -f headless-shell devtest
docker compose -f tests/e2e/docker-compose-browser-tests.yaml down

export TEST_RUN="$TEST_RUN"
export UPDATE_GOLDENS="$UPDATE_GOLDENS"
export MINIFY_BOOL="$MINIFY_BOOL"
export TEST_UID=$(id -u)
export TEST_GID=$(id -g)

docker compose -f tests/e2e/docker-compose-browser-tests.yaml up \
  --abort-on-container-exit \
  --exit-code-from=devtest
RES=$?
docker compose -f tests/e2e/docker-compose-browser-tests.yaml rm -f

if [[ $RES == 0 ]]; then
  echo PASS
else
  echo FAIL
  exit 1
fi
