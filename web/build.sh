#!/bin/sh
# Builds the static landing page. `ham build` compiles the HTML but doesn't
# copy static assets (that's normally Rollup's job); since this site has no
# bundled JS, we skip the whole Node/Rollup toolchain and just copy the
# shared design-system CSS/JS ourselves. This same app.css/app.js pair is
# also served to the Go-rendered app pages, so the whole site — landing
# gallery included — shares one visual language.
set -e
cd "$(dirname "$0")"
ham build
mkdir -p public/assets/css public/assets/js public/assets/img
cp src/app.css public/assets/css/app.css
cp src/app.js public/assets/js/app.js
cp src/assets/*.png public/assets/img/
echo "web/public ready"
