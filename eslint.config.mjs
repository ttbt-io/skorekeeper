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

import js from "@eslint/js";
import stylistic from '@stylistic/eslint-plugin';
import globals from "globals";

export default [
  {
    ignores: ["frontend/vendor/*", "frontend/dist/*"],
  },
  js.configs.recommended,
  {
    files: ["*.js", "frontend/**/*.js", "tests/unit/**/*.js"],
    languageOptions: {
      ecmaVersion: 2022, // Or latest supported by ESLint for general use
      sourceType: "module",
      globals: {
        ...globals.browser,
        ...globals.node,
        ...globals.jest, // Add Jest globals
        // Add any other specific global variables your frontend code uses
        // For example, if you use a global `app` variable:
        // app: 'readonly'
      },
    },
    plugins: {
      '@stylistic': stylistic
    },
    rules: {
      "@stylistic/indent": ["error", 4], // Use 4 spaces for indentation
      "@stylistic/curly-newline": ["error", "always"],
      "semi": ["error", "always"], // Require semicolons
      "quotes": ["error", "single"], // Use single quotes
      "no-trailing-spaces": "error",
      "comma-dangle": ["error", "always-multiline"],
      "object-curly-spacing": ["error", "always"],
      "array-bracket-spacing": ["error", "never"],
      "space-before-function-paren": ["error", "never"],
      "curly": ["error", "all"],
      "no-unused-vars": ["error", { "argsIgnorePattern": "^_" }],
    },
  },
];
