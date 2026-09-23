// @ts-check
import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://castrojo.github.io',
  base: '/bootc-ecosystem',
  output: 'static',
  trailingSlash: 'always',
  vite: {
    build: {
      // Avoid inlining small scripts/assets so every <script> is served
      // as an external, hashed file — keeps CSP script-src 'self' strict
      // (see nginx.conf) without needing 'unsafe-inline'.
      assetsInlineLimit: 0,
    },
  },
});
