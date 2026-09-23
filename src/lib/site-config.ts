/**
 * Single source of truth for repo/site identity strings.
 *
 * This repo is planned to move from `castrojo/bootc-ecosystem` (GitHub Pages)
 * to `projectbluefin/metrics` (eventually served at metrics.projectbluefin.io).
 * See MIGRATION.md for the full checklist. Centralizing these values here
 * means the actual cutover only needs to touch this file plus the handful of
 * places listed in MIGRATION.md that can't read from it (CI YAML, Justfile,
 * astro.config.mjs's static `site`/`base`, docs).
 */

export const REPO_OWNER = 'castrojo';
export const REPO_NAME = 'bootc-ecosystem';
export const REPO_SLUG = `${REPO_OWNER}/${REPO_NAME}`;
export const REPO_URL = `https://github.com/${REPO_SLUG}`;
export const REPO_ISSUES_NEW_URL = `${REPO_URL}/issues/new`;
export const GHCR_IMAGE = `ghcr.io/${REPO_OWNER}/${REPO_NAME}`;
