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

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(__dirname, '..');
const FRONTEND = path.join(ROOT, 'frontend');
const SW_PATH = path.join(FRONTEND, 'sw.js');

function updateSW() {
    console.log('Updating Service Worker manifest...');

    // 1. Gather Core Assets
    const rootFiles = fs.readdirSync(FRONTEND, { withFileTypes: true })
        .filter(e => !e.isDirectory())
        .map(e => e.name)
        .filter(name => (name.endsWith('.js') || name.endsWith('.html') || name.endsWith('.json') || (name.startsWith('icon') && name.endsWith('.png'))) && name !== 'sw.js')
        .map(name => './' + name);

    const rendererFiles = fs.readdirSync(path.join(FRONTEND, 'renderers'))
        .filter(name => name.endsWith('.js'))
        .map(name => './renderers/' + name);

    const modelFiles = fs.readdirSync(path.join(FRONTEND, 'models'))
        .filter(name => name.endsWith('.js'))
        .map(name => './models/' + name);

    const controllerFiles = fs.readdirSync(path.join(FRONTEND, 'controllers'))
        .filter(name => name.endsWith('.js'))
        .map(name => './controllers/' + name);

    const serviceFiles = fs.readdirSync(path.join(FRONTEND, 'services'))
        .filter(name => name.endsWith('.js'))
        .map(name => './services/' + name);

    const gameFiles = fs.readdirSync(path.join(FRONTEND, 'game'))
        .filter(name => name.endsWith('.js'))
        .map(name => './game/' + name);

    const uiFiles = fs.readdirSync(path.join(FRONTEND, 'ui'))
        .filter(name => name.endsWith('.js'))
        .map(name => './ui/' + name);

    const cssFiles = ['./css/style.css'];
    const ssoFiles = ['./.sso/proxy.mjs'];

    const coreAssets = [
        './',
        ...rootFiles,
        ...cssFiles,
        ...rendererFiles,
        ...modelFiles,
        ...controllerFiles,
        ...serviceFiles,
        ...gameFiles,
        ...uiFiles,
        ...ssoFiles,
    ].sort();

    // Deduplicate
    const uniqueCoreAssets = [...new Set(coreAssets)];

    // 2. Gather Optional Assets (Manual)
    const manualDir = path.join(FRONTEND, 'assets', 'manual');
    let optionalAssets = [];
    if (fs.existsSync(manualDir)) {
        optionalAssets = fs.readdirSync(manualDir)
            .filter(name => name.endsWith('.png'))
            .map(name => './assets/manual/' + name)
            .sort();
    }

    // 3. Read current sw.js
    const oldContent = fs.readFileSync(SW_PATH, 'utf8');

    // 4. Prepare asset strings
    const coreAssetsString = `const CORE_ASSETS = [\n${uniqueCoreAssets.map(a => `    '${a}',`).join('\n')}\n];`;
    const optionalAssetsString = `const OPTIONAL_ASSETS = [\n    // Manual assets\n${optionalAssets.map(a => `    '${a}',`).join('\n')}\n];`;

    // 5. Extract current assets for comparison
    const coreRegex = /const CORE_ASSETS = [\s\S]*?\];/;
    const optionalRegex = /const OPTIONAL_ASSETS = [\s\S]*?\];/;

    const currentCore = oldContent.match(coreRegex)?.[0];
    const currentOptional = oldContent.match(optionalRegex)?.[0];

    // 6. Check for changes
    if (currentCore === coreAssetsString && currentOptional === optionalAssetsString) {
        console.log('Service Worker manifest is up to date. No changes detected.');
        return;
    }

    console.log('Manifest changed. Updating sw.js...');

    // 7. Bump Cache Name and Update Assets
    let newContent = oldContent;

    newContent = newContent.replace(/const CACHE_NAME = 'skorekeeper-v(\d+)';/, (match, version) => {
        const nextVersion = parseInt(version, 10) + 1;
        console.log(`Bumping cache version: v${version} -> v${nextVersion}`);
        return `const CACHE_NAME = 'skorekeeper-v${nextVersion}';`;
    });

    newContent = newContent.replace(coreRegex, coreAssetsString);
    newContent = newContent.replace(optionalRegex, optionalAssetsString);

    // 8. Write back
    fs.writeFileSync(SW_PATH, newContent, 'utf8');
    console.log('sw.js updated successfully.');
}

updateSW();