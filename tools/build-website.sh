#!/bin/bash
set -e

# Ensure we are in the project root
if [ ! -d "tools/website-assets" ]; then
    echo "Error: Please run from the project root."
    exit 1
fi

echo "Building website..."

# Create necessary directories
mkdir -p www/assets/js
mkdir -p www/assets/manual
mkdir -p www/assets/css

# Build Tailwind CSS for the website
echo "Building website CSS..."
npx @tailwindcss/cli -i www/assets/css/input.css -o www/assets/css/style.css --content "www/**/*.html"

# Copy Manual Content source of truth
echo "Copying manual content..."
cp frontend/manualContent.js www/assets/js/manualContent.js

echo "Website build complete."
