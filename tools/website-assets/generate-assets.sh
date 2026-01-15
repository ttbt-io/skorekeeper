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

# Go to project root
cd $(dirname $0)/../..

export CGO_ENABLED=0

echo "Building generator binary..."
go build -o tools/website-assets/generator-app tools/website-assets/main.go

echo "Bootstrapping demo-game.json..."
./tools/website-assets/generator-app --generate-only --output-dir=frontend/assets
if [[ ! -f frontend/assets/demo-game.json ]]; then
  echo "Failed to bootstrap demo-game.json"
  exit 1
fi

echo "Re-building generator binary (to embed assets)..."
go build -o tools/website-assets/generator-app tools/website-assets/main.go

echo "Building docker image..."
docker build -f tools/website-assets/Dockerfile -t skorekeeper-website-assets .

echo "Cleaning up binary..."
rm -f tools/website-assets/generator-app

# Create output dir if it doesn't exist
mkdir -p tools/website-assets/output
rm -f tools/website-assets/output/demo-game.json

export TEST_UID=$(id -u)
export TEST_GID=$(id -g)

echo "Running generator..."
docker rm -f website-assets-chrome website-assets-generator 2>/dev/null || true
docker compose -f tools/website-assets/docker-compose.yaml up \
  --abort-on-container-exit \
  --exit-code-from=generator

RES=$?
docker compose -f tools/website-assets/docker-compose.yaml rm -f

if [[ $RES == 0 ]]; then
  echo "Assets generated successfully in tools/website-assets/output/"
  
  if [[ -f tools/website-assets/output/demo-game.json ]]; then
    echo "Installing demo game..."
    cp tools/website-assets/output/demo-game.json frontend/assets/demo-game.json
  fi
  echo "Installing images..."
  cp tools/website-assets/output/*.png www/assets/
else
  echo "Asset generation FAILED."
  exit 1
fi
