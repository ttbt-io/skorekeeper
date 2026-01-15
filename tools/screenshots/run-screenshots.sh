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

cd $(dirname $0)/../..

export CGO_ENABLED=0
mkdir -p frontend/assets/manual
touch frontend/assets/manual/README.md

# Build the screenshot generator binary
go build -o tools/screenshots/generator-app github.com/ttbt-io/skorekeeper/tools/screenshots

# Build docker image
docker build -f tools/screenshots/Dockerfile -t skorekeeper-screenshots .

# Cleanup binary
rm -f tools/screenshots/generator-app

# Cleanup containers
docker rm -f chrome-screenshots screenshot-generator
docker compose -f tools/screenshots/docker-compose.yaml down

export TEST_UID=$(id -u)
export TEST_GID=$(id -g)

# Run generation
docker compose -f tools/screenshots/docker-compose.yaml up \
  --abort-on-container-exit \
  --exit-code-from=generator

RES=$?
docker compose -f tools/screenshots/docker-compose.yaml rm -f

if [[ $RES == 0 ]]; then
  echo "Screenshots generated successfully."
else
  echo "Screenshot generation FAILED."
  exit 1
fi
