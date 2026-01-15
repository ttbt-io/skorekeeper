import fs from 'fs/promises';
import path from 'path';
import esbuild from 'esbuild';

const DIST_DIR = 'frontend/dist';

async function minify() {
    await fs.mkdir(DIST_DIR, { recursive: true });

    // 1. Bundle JS
    console.log('Bundling JS...');
    await esbuild.build({
        entryPoints: ['frontend/init.js'],
        bundle: true,
        minify: true,
        outfile: path.join(DIST_DIR, 'app.min.js'),
        format: 'esm',
        sourcemap: true,
        external: ['/.sso/proxy.mjs'],
    });

    // 2. Minify CSS
    console.log('Minifying CSS...');
    // Ensure style.css exists (run build:css if needed, but assuming it exists)
    try {
        await esbuild.build({
            entryPoints: ['frontend/css/style.css'],
            bundle: true,
            minify: true,
            outfile: path.join(DIST_DIR, 'style.min.css'),
        });
    } catch (e) {
        console.warn('Warning: css/style.css not found. Skipping CSS minification (ensure tailwind build runs first).');
    }

    // 3. Minify separate errorLogger
    console.log('Minifying ErrorLogger...');
    await esbuild.build({
        entryPoints: ['frontend/services/errorLogger.js'],
        bundle: true,
        minify: true,
        outfile: path.join(DIST_DIR, 'errorLogger.min.js'),
    });

    // 4. Generate index.min.html
    console.log('Generating index.min.html...');
    let html = await fs.readFile('frontend/index.html', 'utf-8');

    // Replace CSS
    html = html.replace(
        '<link rel="stylesheet" href="css/style.css">',
        '<link rel="stylesheet" href="dist/style.min.css">'
    );

    // Replace JS
    html = html.replace(
        '<script type="module" src="./init.js"></script>',
        '<script type="module" src="dist/app.min.js"></script>'
    );
    
    html = html.replace(
        '<script src="./services/errorLogger.js"></script>',
        '<script src="dist/errorLogger.min.js"></script>'
    );

    // No asset rewrites needed, they are served from root

    await fs.writeFile(path.join(DIST_DIR, 'index.min.html'), html);
    
    // 5. Generate sw.js for dist
    console.log('Generating dist/sw.js...');
    let swContent = await fs.readFile('frontend/sw.js', 'utf-8');
    // Simplified CORE_ASSETS for minified version
    const minifiedCoreAssets = [
        './',
        './.sso/proxy.mjs',
        './dist/app.min.js',
        './dist/style.min.css',
        './dist/errorLogger.min.js',
        './manifest.json',
        './icon.png',
        './icon-192x192.png',
        './icon-512x512.png',
        './icon-64x64.png',
    ];
    
    // Replace CORE_ASSETS array in sw.js
    // Note: sw.js is served from /sw.js, so paths are relative to root.
    const coreAssetsRegex = /const CORE_ASSETS = \[\s*[\s\S]*?\];/;
    swContent = swContent.replace(coreAssetsRegex, `const CORE_ASSETS = ${JSON.stringify(minifiedCoreAssets, null, 4)};`);
    
    await fs.writeFile(path.join(DIST_DIR, 'sw.js'), swContent);

    console.log('Minification complete.');
}

minify().catch(e => {
    console.error(e);
    process.exit(1);
});