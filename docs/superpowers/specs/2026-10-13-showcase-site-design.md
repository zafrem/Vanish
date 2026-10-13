# Showcase Site — Design

Date: 2026-10-13
Status: Draft, awaiting review
URL: https://vanish.codeforlife.dev (GitHub Pages, custom domain)

## Goal

A public, static landing page that explains what Vanish is, why it exists, how its zero-knowledge flow works, and how to get started, with screenshots of the app. It showcases the repo; it does not run a live Vanish instance (Pages is static hosting with no Redis/backend).

## Non-goals

- A live demo, in-browser crypto widget, or login. (Possible later addition.)
- A multi-page documentation site or a build pipeline. Existing `docs/` stays as the repo's documentation.
- Analytics, cookies, third-party scripts, fonts, or embeds.
- Translations. English only.
- Advertising the file-sharing feature before it ships (see `2026-10-13-file-sharing-design.md`).

## Location and layout

```
site/
  index.html
  styles.css
  main.js          (optional; copy-to-clipboard buttons only)
  CNAME            (contains: vanish.codeforlife.dev)
  assets/          (logo, favicon, compressed screenshots)
.github/workflows/pages.yml
```

`site/` is used instead of `docs/` because `docs/` already contains project documentation that must not be published by the Pages "serve from /docs" option.

## Page

Hand-written HTML/CSS, no framework, no build step.

- **Design:** dark theme by default matching the app, with a light variant via `prefers-color-scheme`. Colors defined as CSS custom properties. System font stack. Responsive down to phone width with a 16px gutter and no horizontal scroll. Semantic landmarks, visible focus states, sufficient contrast, `alt` text on all images, respects `prefers-reduced-motion`.
- **Head:** title, meta description, canonical URL, Open Graph/Twitter tags, favicon derived from `images/logo.png`.
- **Privacy:** the page makes no third-party requests.

Sections in order:

1. **Hero:** logo, "Share secrets that vanish after one read", buttons: View on GitHub, Quick start.
2. **Why it exists:** the origin story from the README, shortened.
3. **How it works:** inline SVG diagram (labeled for both themes): encrypt in the browser → server stores only ciphertext + IV with a TTL → key stays in the URL `#fragment` → message is deleted on first read. Accompanied by a short text equivalent.
4. **Features:** zero-knowledge, client-side AES-256-GCM, burn-on-read, clipboard protection, in-memory (Redis) storage, configurable TTL (1 hour to 7 days).
5. **Screenshots:** the app images from `images/`, copied to `site/assets/` and compressed.
6. **Everywhere you work:** web app, CLI, Chrome extension, Slack/email/Okta integrations.
7. **Quick start:** `./start.sh docker` and `docker-compose up -d` with copy buttons (the page still works without JS).
8. **Footer:** GitHub, docs (`docs/` on GitHub), license, Docker Hub image `zafrem/vanish`.

Content is taken from the README and `docs/` and must stay factually consistent with them. No claims beyond what the code does. In particular, do not state that the server can never decrypt anything while the encryption key is stored in PostgreSQL metadata (see "Known issues" in the file-sharing spec); use the README's wording about client-side encryption and plaintext never being stored or logged.

## Screenshot review (required before publishing)

Every image is opened and inspected for real names, emails, tokens, keys, internal URLs or other sensitive data. Anything found is cropped or blurred, or the image is omitted. Images are resized (max ~1600px wide) and compressed. The originals in `images/` are not modified.

## Hosting and deployment

- `.github/workflows/pages.yml`: triggers on push to `main` for paths `site/**` and `.github/workflows/pages.yml`, plus `workflow_dispatch`. Uses `actions/configure-pages`, `actions/upload-pages-artifact` (path `site`), and `actions/deploy-pages`, each pinned to a specific released version. Permissions: `contents: read`, `pages: write`, `id-token: write`. Concurrency group `pages`. Does not touch `cli-ci.yml` or `docker-publish.yml`.
- Repository setting: Pages source = "GitHub Actions". Custom domain = `vanish.codeforlife.dev` (also expressed by `site/CNAME`).
- **DNS (done by the repo owner, not by this work):** a `CNAME` record `vanish` → `zafrem.github.io` at the `codeforlife.dev` DNS provider. After it propagates, GitHub provisions the HTTPS certificate and "Enforce HTTPS" can be enabled. Propagation and certificate issuance can take minutes to hours and cannot be verified until the record exists.
- Pages on a private repository requires a paid GitHub plan; the repository must be public or the plan must allow it. Confirm before relying on it.

## Approvals and outward-facing steps

Nothing is committed to `main`, pushed, deployed, or changed in repository settings without explicit confirmation at that step. Local preview is allowed (`python3 -m http.server` in `site/`).

## Testing / verification

- Local preview at desktop and phone widths in both color schemes; no horizontal scroll; no console errors; no network requests to other origins.
- HTML validity check and a link check (internal anchors and external links).
- Lighthouse (or equivalent) pass for accessibility and performance; page weight target under ~1.5 MB including screenshots.
- After deploy: the workflow run succeeds, the `*.github.io` URL serves the page, and, once DNS exists, `https://vanish.codeforlife.dev` serves it with a valid certificate.

## Open items

- Confirm whether the repository is public (needed for free Pages).
- Repo owner creates the DNS record.
- Add file sharing to section 6 only after that feature ships.
