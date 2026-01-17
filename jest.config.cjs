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

module.exports = {
    // The directory where Jest should output its coverage files
    coverageDirectory: "coverage",

    // A list of paths to directories that Jest should use to search for test files
    roots: [
        "<rootDir>/tests/unit"
    ],

    // The test environment that will be used for testing
    testEnvironment: "jsdom",
    setupFilesAfterEnv: ['<rootDir>/tests/unit/setup.js'],
  transform: {
    '^.+.[m|j]s$': 'babel-jest', // Apply babel-jest to all .js and .mjs files
  },
  moduleNameMapper: {
    '^/\\.sso/proxy\\.mjs$': '<rootDir>/frontend/.sso/proxy.mjs',
  },
};