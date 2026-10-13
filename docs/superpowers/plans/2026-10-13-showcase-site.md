# Showcase Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A static, privacy-respecting landing page for Vanish, deployed to GitHub Pages at `https://vanish.codeforlife.dev`.

**Architecture:** Hand-written HTML/CSS (no framework, no build step) in `site/`, deployed by a GitHub Actions workflow that uploads `site/` as the Pages artifact. A stdlib-only Python checker (`scripts/check_site.py`) enforces the site's rules (no third-party loads, valid local links and anchors, alt text, required metadata, size budget, no over-claiming copy) locally and in CI.

**Tech Stack:** HTML5, CSS custom properties, a few lines of vanilla JS, Python 3 standard library + Pillow (asset prep only), GitHub Actions (`configure-pages`, `upload-pages-artifact`, `deploy-pages`).

**Spec:** `docs/superpowers/specs/2026-10-13-showcase-site-design.md`

## Global Constraints

- English only. No analytics, cookies, third-party scripts, fonts, images or embeds; the page makes no requests to other origins.
- Dark theme by default, light theme via `prefers-color-scheme: light`; colors defined as CSS custom properties; system font stack.
- Works at phone width with a 16px side gutter and no horizontal scroll; semantic landmarks, visible focus states, `alt` text on every image, respects `prefers-reduced-motion`.
- Page weight (everything in `site/`) ≤ 1.5 MB.
- `site/CNAME` contains exactly `vanish.codeforlife.dev`.
- `site/` is the published directory; never publish `docs/`.
- Copy must match the README. Do **not** claim the server can never decrypt anything or that the key never reaches the server (the backend currently stores the key in PostgreSQL metadata). Say "encrypted in your browser before it is sent" and that the key is placed in the link's `#fragment`, which browsers do not include in HTTP requests.
- Do not advertise file sharing until that feature ships.
- Do all work on a feature branch (e.g. `feat/showcase-site`), not on `main`. **Do not commit to `main`, push, change repository settings, or deploy without the repo owner's explicit confirmation at that step** (Task 6).

## Review Focus

1. A screenshot that leaks a real name, email, token or internal URL must never be published (Task 2 inspection gate).
2. A third-party request sneaking in (font, script, analytics, `og:image` on another host) would break the privacy promise (checker, Task 1).
3. Phone-width layout: no horizontal scroll from code blocks, the diagram, or long URLs (Task 3 + Task 5 checks).
4. Copy that over-claims zero-knowledge or lists unshipped features (checker banned phrases, Task 1).
5. JavaScript disabled: the page and all content must still work; copy buttons simply don't appear (Task 3).

---

### Task 1: Site checker (written first, fails first)

**Files:**
- Create: `scripts/check_site.py`
- Create: `scripts/test_check_site.py`

**Interfaces:**
- Produces: `python3 scripts/check_site.py [site_dir]` (default `site`) → exit code 0 when the site passes, 1 otherwise, printing one line per problem. Functions: `check(site_dir: str) -> list[str]` (list of problems) — imported by the unit test.

- [ ] **Step 1: Write the failing test** — `scripts/test_check_site.py`:

```python
import os
import tempfile
import unittest

from check_site import check

GOOD_HTML = """<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Vanish</title>
<meta name="description" content="d">
<link rel="canonical" href="https://vanish.codeforlife.dev/">
<meta property="og:title" content="t">
<link rel="stylesheet" href="styles.css">
</head><body>
<a href="#top">top</a><h1 id="top">Hi</h1>
<img src="assets/a.png" alt="A thing">
<a href="https://github.com/zafrem/Vanish">repo</a>
</body></html>"""


def make_site(index=GOOD_HTML, cname="vanish.codeforlife.dev\n", extra=None):
    d = tempfile.mkdtemp()
    os.makedirs(os.path.join(d, "assets"))
    files = {"index.html": index, "styles.css": "body{}", "CNAME": cname, "assets/a.png": "x"}
    files.update(extra or {})
    for name, content in files.items():
        with open(os.path.join(d, name), "w", encoding="utf-8") as f:
            f.write(content)
    return d


class CheckSiteTest(unittest.TestCase):
    def test_good_site_passes(self):
        self.assertEqual(check(make_site()), [])

    def test_missing_files(self):
        d = tempfile.mkdtemp()
        problems = check(d)
        self.assertTrue(any("index.html" in p for p in problems))

    def test_cname_must_match(self):
        self.assertTrue(any("CNAME" in p for p in check(make_site(cname="other.example\n"))))

    def test_third_party_resource_rejected(self):
        bad = GOOD_HTML.replace("</head>", '<script src="https://cdn.example.com/x.js"></script></head>')
        self.assertTrue(any("third-party" in p for p in check(make_site(index=bad))))

    def test_external_stylesheet_and_font_rejected(self):
        bad = GOOD_HTML.replace("</head>", '<link rel="stylesheet" href="https://fonts.googleapis.com/x"></head>')
        self.assertTrue(any("third-party" in p for p in check(make_site(index=bad))))

    def test_css_remote_url_rejected(self):
        d = make_site(extra={"styles.css": "@import url(https://x.example/a.css);"})
        self.assertTrue(any("third-party" in p for p in check(d)))

    def test_unknown_external_link_rejected(self):
        bad = GOOD_HTML.replace("</body>", '<a href="https://evil.example/">x</a></body>')
        self.assertTrue(any("external link" in p for p in check(make_site(index=bad))))

    def test_missing_local_file_rejected(self):
        bad = GOOD_HTML.replace("assets/a.png", "assets/missing.png")
        self.assertTrue(any("missing.png" in p for p in check(make_site(index=bad))))

    def test_broken_anchor_rejected(self):
        bad = GOOD_HTML.replace('href="#top"', 'href="#nowhere"')
        self.assertTrue(any("#nowhere" in p for p in check(make_site(index=bad))))

    def test_img_without_alt_rejected(self):
        bad = GOOD_HTML.replace(' alt="A thing"', "")
        self.assertTrue(any("alt" in p for p in check(make_site(index=bad))))

    def test_missing_metadata_rejected(self):
        bad = GOOD_HTML.replace('<meta name="description" content="d">', "")
        self.assertTrue(any("description" in p for p in check(make_site(index=bad))))

    def test_banned_claim_rejected(self):
        bad = GOOD_HTML.replace("<h1", "<p>The server can never decrypt your data.</p><h1")
        self.assertTrue(any("banned" in p for p in check(make_site(index=bad))))

    def test_size_budget(self):
        d = make_site(extra={"assets/big.bin": "x" * (1_600_000)})
        self.assertTrue(any("size" in p for p in check(d)))


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run to verify it fails**

Run (from repo root): `python3 -I -m unittest discover -s scripts -p 'test_check_site.py' 2>&1 | tail -3`
Expected: FAIL — `ModuleNotFoundError: No module named 'check_site'`. (`-I` ignores the cwd; `discover -s scripts` puts `scripts/` on the path, which is intended here because it is our own code.)

- [ ] **Step 3: Create `scripts/check_site.py`:**

```python
#!/usr/bin/env python3
"""Checks the Vanish showcase site against its rules. Standard library only."""
import os
import re
import sys
from html.parser import HTMLParser

CNAME = "vanish.codeforlife.dev"
MAX_BYTES = 1_500_000
EXTERNAL_LINK_HOSTS = {"github.com", "hub.docker.com"}
BANNED_PHRASES = [
    "server can never",
    "cannot decrypt",
    "can't decrypt",
    "can never see",
    "key never leaves",
    "never leaves your",
    "even we can't",
    "neither can anyone",
]
REQUIRED_HEAD = [
    ("title", lambda s: s.title.strip()),
    ("meta description", lambda s: s.meta.get("description")),
    ("canonical link", lambda s: s.canonical),
    ("og:title", lambda s: s.meta.get("og:title")),
]


class Page(HTMLParser):
    def __init__(self):
        super().__init__()
        self.title = ""
        self._in_title = False
        self.meta = {}
        self.canonical = None
        self.ids = set()
        self.refs = []       # (tag, attr, value, attrs)
        self.imgs = []       # attrs dicts
        self.text = []

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if "id" in a:
            self.ids.add(a["id"])
        if tag == "title":
            self._in_title = True
        elif tag == "meta":
            key = a.get("name") or a.get("property")
            if key:
                self.meta[key] = a.get("content", "")
        elif tag == "link" and a.get("rel") == "canonical":
            self.canonical = a.get("href")
        elif tag == "img":
            self.imgs.append(a)
        for attr in ("src", "href"):
            if attr in a:
                self.refs.append((tag, attr, a[attr], a))

    def handle_endtag(self, tag):
        if tag == "title":
            self._in_title = False

    def handle_data(self, data):
        if self._in_title:
            self.title += data
        self.text.append(data)


def is_remote(url):
    return bool(re.match(r"^(https?:)?//", url))


def host_of(url):
    return re.sub(r"^(https?:)?//", "", url).split("/")[0].lower()


def check(site_dir):
    problems = []
    index = os.path.join(site_dir, "index.html")

    for name in ("index.html", "styles.css", "CNAME"):
        if not os.path.isfile(os.path.join(site_dir, name)):
            problems.append(f"missing required file: {name}")
    if problems:
        return problems

    with open(os.path.join(site_dir, "CNAME"), encoding="utf-8") as f:
        if f.read().strip() != CNAME:
            problems.append(f"CNAME must contain exactly {CNAME}")

    total = 0
    for root, _, files in os.walk(site_dir):
        for name in files:
            total += os.path.getsize(os.path.join(root, name))
    if total > MAX_BYTES:
        problems.append(f"site size {total} bytes exceeds budget {MAX_BYTES}")

    with open(index, encoding="utf-8") as f:
        html = f.read()
    page = Page()
    page.feed(html)

    for label, getter in REQUIRED_HEAD:
        if not getter(page):
            problems.append(f"missing {label}")

    for tag, attr, value, attrs in page.refs:
        if value.startswith(("mailto:", "tel:", "data:")):
            continue
        if value.startswith("#"):
            if value != "#" and value[1:] not in page.ids:
                problems.append(f"broken anchor {value}")
            continue
        if is_remote(value):
            loads_resource = tag in ("img", "script", "source", "iframe") or (
                tag == "link" and attrs.get("rel") != "canonical"
            )
            if loads_resource:
                problems.append(f"third-party resource loaded: <{tag}> {value}")
            elif tag == "a" and host_of(value) not in EXTERNAL_LINK_HOSTS:
                problems.append(f"external link to non-allowlisted host: {value}")
            continue
        path = os.path.join(site_dir, value.split("#")[0].split("?")[0])
        if not os.path.exists(path):
            problems.append(f"local reference does not exist: {value}")

    for a in page.imgs:
        if "alt" not in a or (a["alt"] == "" and a.get("aria-hidden") != "true"):
            problems.append(f"img without alt text: {a.get('src')}")

    # og:image / og:url may point at our own origin only
    for key in ("og:image", "og:url"):
        value = page.meta.get(key)
        if value and is_remote(value) and host_of(value) != CNAME:
            problems.append(f"{key} must use https://{CNAME}: {value}")

    css_path = os.path.join(site_dir, "styles.css")
    with open(css_path, encoding="utf-8") as f:
        css = f.read()
    for m in re.finditer(r"url\(\s*['\"]?(https?:)?//|@import\s+(url\()?['\"]?(https?:)?//", css):
        problems.append(f"third-party resource loaded from CSS near: {css[m.start():m.start() + 60]!r}")

    visible = " ".join(page.text).lower()
    for phrase in BANNED_PHRASES:
        if phrase in visible:
            problems.append(f"banned claim in copy: {phrase!r}")

    return problems


if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "site"
    found = check(target)
    for p in found:
        print(f"FAIL: {p}")
    if found:
        sys.exit(1)
    print(f"OK: {target} passes all checks")
```

- [ ] **Step 4: Run to verify the unit tests pass**

Run: `python3 -I -m unittest discover -s scripts -p 'test_check_site.py' -v 2>&1 | tail -20`
Expected: `Ran 13 tests ... OK`. Then confirm the real check fails today: `python3 scripts/check_site.py` → `FAIL: missing required file: index.html`, exit 1.

- [ ] **Step 5: Commit** (on the feature branch)

```bash
git checkout -b feat/showcase-site
git add scripts/check_site.py scripts/test_check_site.py
git commit -m "test(site): add site checker for privacy, links, accessibility and size"
```

---

### Task 2: Assets — screenshot review, favicon, social image

**Files:**
- Create: `site/assets/` containing `shot-*.jpg` (reviewed, resized), `logo.png`, `favicon.png`, `og.png`
- Create: `site/CNAME`

**Interfaces:**
- Produces: asset paths used by Task 3: `assets/logo.png` (256 px), `assets/favicon.png` (64 px), `assets/og.png` (1200×630), and screenshots `assets/shot-login.jpg`, `assets/shot-send.jpg`, `assets/shot-link.jpg`, `assets/shot-inbox.jpg`, `assets/shot-history.jpg` (only those that pass review).

- [ ] **Step 1: INSPECT every screenshot (gate — do not skip).** Open each image with the Read tool and look for real names, email addresses, tokens/keys, internal hostnames, or anything else that should not be public:

`images/login.jpg`, `images/Send message.jpg`, `images/onetime_url.jpg`, `images/Message_main.jpg`, `images/Histroy.jpg`

Write down, per image: what it shows, whether it contains sensitive data, and the final decision (include / crop / blur / omit). If an image has sensitive content you cannot cleanly crop or blur, omit it and adjust the screenshot section in Task 3. Show the list of decisions to the repo owner before publishing (Task 6).

- [ ] **Step 2: Create `site/CNAME`:**

```bash
mkdir -p site/assets
printf 'vanish.codeforlife.dev\n' > site/CNAME
```

- [ ] **Step 3: Produce the assets** with Pillow (available as `PIL`; if missing, `python3 -m venv /tmp/claude-1000/-home-baruna-playground-private-Vanish/c3bc6f52-4325-4d6b-8f79-39510b9ad8dc/scratchpad/venv && . that/bin/activate && pip install pillow`). Save this helper in the scratchpad directory (not in the repo) and run it from the repo root; add `crop`/`blur` boxes for any image that needed them in Step 1:

```python
from PIL import Image, ImageFilter

SHOTS = {  # source -> destination
    "images/login.jpg": "site/assets/shot-login.jpg",
    "images/Send message.jpg": "site/assets/shot-send.jpg",
    "images/onetime_url.jpg": "site/assets/shot-link.jpg",
    "images/Message_main.jpg": "site/assets/shot-inbox.jpg",
    "images/Histroy.jpg": "site/assets/shot-history.jpg",
}
BLUR_BOXES = {}   # e.g. {"images/Histroy.jpg": [(left, top, right, bottom), ...]} from Step 1
OMIT = set()      # sources that failed review

for src, dst in SHOTS.items():
    if src in OMIT:
        continue
    im = Image.open(src).convert("RGB")
    for box in BLUR_BOXES.get(src, []):
        region = im.crop(box).filter(ImageFilter.GaussianBlur(14))
        im.paste(region, box)
    if im.width > 1200:
        im = im.resize((1200, round(im.height * 1200 / im.width)), Image.LANCZOS)
    im.save(dst, "JPEG", quality=78, optimize=True, progressive=True)  # drops EXIF

logo = Image.open("images/logo.png").convert("RGBA")
logo.resize((256, 256), Image.LANCZOS).save("site/assets/logo.png", optimize=True)
logo.resize((64, 64), Image.LANCZOS).save("site/assets/favicon.png", optimize=True)

og = Image.new("RGB", (1200, 630), (15, 23, 42))
mark = logo.resize((360, 360), Image.LANCZOS)
og.paste(mark, ((1200 - 360) // 2, (630 - 360) // 2), mark)
og.save("site/assets/og.png", optimize=True)
```

- [ ] **Step 4: Re-inspect the output.** Open each `site/assets/shot-*.jpg` with the Read tool and confirm no sensitive content remains and any blur/crop is clean. Check sizes: `du -sh site/assets && ls -l site/assets` — total must stay well under 1 MB (target < 700 KB) so the page fits the 1.5 MB budget.

- [ ] **Step 5: Commit**

```bash
git add site/CNAME site/assets
git commit -m "feat(site): add reviewed screenshots, favicon, social image and CNAME"
```

---

### Task 3: The page — `index.html`, `styles.css`, `main.js`

**Files:**
- Create: `site/index.html`
- Create: `site/styles.css`
- Create: `site/main.js`

**Interfaces:**
- Consumes: assets from Task 2; `scripts/check_site.py` from Task 1 as the acceptance test.
- Produces: the finished static site in `site/`.

Adjust the `<figure>` blocks in the screenshots section to the images that passed review in Task 2 (remove a figure if its image was omitted) and refine each `alt` to describe what the image actually shows.

- [ ] **Step 1: Confirm the checker currently fails**

Run: `python3 scripts/check_site.py`
Expected: `FAIL: missing required file: index.html`.

- [ ] **Step 2: Create `site/index.html`:**

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Vanish — secrets that vanish after one read</title>
  <meta name="description" content="Vanish is an ephemeral secret-sharing platform. Secrets are encrypted in your browser, stored briefly, and destroyed the moment they are read.">
  <meta name="color-scheme" content="dark light">
  <link rel="canonical" href="https://vanish.codeforlife.dev/">
  <link rel="icon" type="image/png" href="assets/favicon.png">
  <meta property="og:type" content="website">
  <meta property="og:title" content="Vanish — secrets that vanish after one read">
  <meta property="og:description" content="Share passwords, API keys and certificates without leaving a trail. Encrypted in your browser, destroyed on first read.">
  <meta property="og:url" content="https://vanish.codeforlife.dev/">
  <meta property="og:image" content="https://vanish.codeforlife.dev/assets/og.png">
  <meta name="twitter:card" content="summary_large_image">
  <link rel="stylesheet" href="styles.css">
  <script src="main.js" defer></script>
</head>
<body>
  <a class="skip" href="#main">Skip to content</a>

  <header class="site-header">
    <div class="wrap bar">
      <a class="brand" href="#top" aria-label="Vanish home">
        <img src="assets/logo.png" alt="" aria-hidden="true" width="32" height="32">
        <span>Vanish</span>
      </a>
      <nav aria-label="Primary">
        <a href="#how">How it works</a>
        <a href="#features">Features</a>
        <a href="#start">Quick start</a>
        <a class="btn btn-small" href="https://github.com/zafrem/Vanish">GitHub</a>
      </nav>
    </div>
  </header>

  <main id="main">
    <section id="top" class="hero">
      <div class="wrap">
        <img class="hero-logo" src="assets/logo.png" alt="Vanish logo" width="128" height="128">
        <h1>Share secrets that <span class="grad">vanish</span> after one read</h1>
        <p class="lead">Vanish is an ephemeral messaging platform for single-use information transfer: passwords, API keys, certificates. Encrypted in your browser, stored briefly, destroyed the moment they are read.</p>
        <p class="cta">
          <a class="btn" href="#start">Quick start</a>
          <a class="btn btn-ghost" href="https://github.com/zafrem/Vanish">View on GitHub</a>
        </p>
      </div>
    </section>

    <section id="why" class="band">
      <div class="wrap narrow">
        <h2>Why it exists</h2>
        <blockquote>
          <p>One day, someone sent me a token over a messenger app, and I completely forgot about it. When I stumbled upon it later and used it, it hit me: this is sensitive data, and it had been sitting in a chat. I wanted a way to transfer secrets without prying eyes.</p>
        </blockquote>
        <p>Secrets pasted into chat or email stay there for years. Vanish gives them a lifetime of one read.</p>
      </div>
    </section>

    <section id="how">
      <div class="wrap">
        <h2>How it works</h2>
        <svg class="diagram" viewBox="0 0 720 200" role="img" aria-labelledby="diagram-title diagram-desc">
          <title id="diagram-title">How a Vanish message travels</title>
          <desc id="diagram-desc">The sender's browser encrypts the secret, the Vanish server stores only the ciphertext with an expiry time, and the recipient's browser decrypts it. The server deletes the message on first read.</desc>
          <defs>
            <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
              <path d="M0 0 L10 5 L0 10 z" class="arrowhead"/>
            </marker>
          </defs>
          <g class="node">
            <rect x="10" y="50" width="190" height="100" rx="12"/>
            <text x="105" y="92" text-anchor="middle" class="t-head">Sender's browser</text>
            <text x="105" y="118" text-anchor="middle" class="t-sub">encrypts with AES-256-GCM</text>
          </g>
          <g class="node">
            <rect x="265" y="50" width="190" height="100" rx="12"/>
            <text x="360" y="92" text-anchor="middle" class="t-head">Vanish server</text>
            <text x="360" y="118" text-anchor="middle" class="t-sub">holds ciphertext + expiry</text>
          </g>
          <g class="node">
            <rect x="520" y="50" width="190" height="100" rx="12"/>
            <text x="615" y="92" text-anchor="middle" class="t-head">Recipient's browser</text>
            <text x="615" y="118" text-anchor="middle" class="t-sub">decrypts, then it's gone</text>
          </g>
          <line x1="200" y1="100" x2="263" y2="100" class="edge" marker-end="url(#arrow)"/>
          <line x1="455" y1="100" x2="518" y2="100" class="edge" marker-end="url(#arrow)"/>
          <text x="232" y="86" text-anchor="middle" class="t-edge">1</text>
          <text x="487" y="86" text-anchor="middle" class="t-edge">2</text>
        </svg>
        <ol class="steps">
          <li><strong>Encrypt.</strong> Your secret is encrypted in your browser with AES-256-GCM before it is sent.</li>
          <li><strong>Store briefly.</strong> The server keeps the encrypted message in memory (Redis) with a time-to-live from 1 hour to 7 days.</li>
          <li><strong>Share a link.</strong> The decryption key is placed in the link's <code>#fragment</code>, the part of a URL that browsers don't include in HTTP requests.</li>
          <li><strong>Burn on read.</strong> The first read returns the message and deletes it in one atomic step. Opening the link again finds nothing.</li>
        </ol>
      </div>
    </section>

    <section id="features" class="band">
      <div class="wrap">
        <h2>Features</h2>
        <ul class="cards">
          <li><h3>Encrypted before it leaves your browser</h3><p>AES-256-GCM through the Web Crypto API. Plaintext is not stored or logged.</p></li>
          <li><h3>Burn on read</h3><p>A message can be read exactly once, then it is permanently destroyed.</p></li>
          <li><h3>Truly ephemeral</h3><p>Messages live in Redis memory with a TTL you choose, from 1 hour to 7 days.</p></li>
          <li><h3>Clipboard protection</h3><p>Retrieved secrets go straight to your clipboard instead of being displayed on the page.</p></li>
          <li><h3>Built for teams</h3><p>Messages are addressed to a specific recipient, with Okta sign-in and Slack or email notifications.</p></li>
          <li><h3>Open source</h3><p>Read the code, run it yourself with Docker, and adapt it to your environment.</p></li>
        </ul>
      </div>
    </section>

    <section id="screens">
      <div class="wrap">
        <h2>See it in action</h2>
        <div class="shots">
          <figure><img src="assets/shot-send.jpg" alt="The Vanish page for writing a secret, choosing a recipient and an expiry time" loading="lazy"><figcaption>Write a secret, pick a recipient and an expiry.</figcaption></figure>
          <figure><img src="assets/shot-link.jpg" alt="The one-time link shown after a secret is created" loading="lazy"><figcaption>Share the one-time link.</figcaption></figure>
          <figure><img src="assets/shot-inbox.jpg" alt="The Vanish page where a recipient reveals and copies a secret" loading="lazy"><figcaption>The recipient reveals it once, straight to the clipboard.</figcaption></figure>
          <figure><img src="assets/shot-history.jpg" alt="The message history view listing sent and received messages without their contents" loading="lazy"><figcaption>History shows who and when, never the contents.</figcaption></figure>
        </div>
      </div>
    </section>

    <section id="everywhere" class="band">
      <div class="wrap">
        <h2>Where you work</h2>
        <ul class="cards">
          <li><h3>Web app</h3><p>A React single-page app for creating and opening secrets.</p></li>
          <li><h3>Command line</h3><p><code>vanish send</code> pipes secrets from scripts and terminals.</p></li>
          <li><h3>Chrome extension</h3><p>Send secrets without leaving the page you're on.</p></li>
          <li><h3>Integrations</h3><p>Okta, Slack, email and Vault, each optional and off by default.</p></li>
        </ul>
      </div>
    </section>

    <section id="start">
      <div class="wrap narrow">
        <h2>Quick start</h2>
        <p>You need Docker and Docker Compose. Clone the repository, then start the whole stack:</p>
        <div class="code">
          <pre><code id="cmd-start">./start.sh docker</code></pre>
          <button class="copy" type="button" data-copy-target="cmd-start" hidden>Copy</button>
        </div>
        <p>Or use Docker Compose directly:</p>
        <div class="code">
          <pre><code id="cmd-compose">docker-compose up -d</code></pre>
          <button class="copy" type="button" data-copy-target="cmd-compose" hidden>Copy</button>
        </div>
        <p>Then open <code>http://localhost:3000</code>. For local development, <code>./start.sh dev</code> runs the Go backend and Vite frontend with hot reload. See the <a href="https://github.com/zafrem/Vanish/tree/main/docs">documentation</a> for configuration and deployment.</p>
      </div>
    </section>
  </main>

  <footer class="site-footer">
    <div class="wrap">
      <p>
        <a href="https://github.com/zafrem/Vanish">GitHub</a> ·
        <a href="https://github.com/zafrem/Vanish/tree/main/docs">Docs</a> ·
        <a href="https://github.com/zafrem/Vanish/blob/main/LICENSE">License</a> ·
        <a href="https://hub.docker.com/r/zafrem/vanish">Docker image</a>
      </p>
      <p class="muted">This site sets no cookies and makes no third-party requests.</p>
    </div>
  </footer>
</body>
</html>
```

- [ ] **Step 3: Create `site/styles.css`:**

```css
:root {
  --bg: #0b1220;
  --bg-band: #111a2e;
  --card: #16213a;
  --border: #26344f;
  --text: #e6ebf5;
  --muted: #9aa8c2;
  --accent-a: #ef4444;
  --accent-b: #f97316;
  --link: #7cc4ff;
  --code-bg: #0a101d;
  --focus: #ffd166;
  --font: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif;
  --mono: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace;
}

@media (prefers-color-scheme: light) {
  :root {
    --bg: #ffffff;
    --bg-band: #f3f6fb;
    --card: #ffffff;
    --border: #d5deec;
    --text: #0f1b33;
    --muted: #4c5b78;
    --accent-a: #dc2626;
    --accent-b: #ea580c;
    --link: #0b5cad;
    --code-bg: #eef2f9;
    --focus: #9a5b00;
  }
}

*, *::before, *::after { box-sizing: border-box; }

html { scroll-behavior: smooth; }
@media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } * { transition: none !important; } }

body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 1rem/1.65 var(--font);
  -webkit-text-size-adjust: 100%;
}

img, svg { max-width: 100%; height: auto; }
a { color: var(--link); }
a:focus-visible, button:focus-visible { outline: 3px solid var(--focus); outline-offset: 2px; }
code { font-family: var(--mono); font-size: .92em; background: var(--code-bg); padding: .1em .35em; border-radius: 4px; overflow-wrap: anywhere; }

.wrap { max-width: 1040px; margin: 0 auto; padding: 0 16px; }
.narrow { max-width: 720px; }

.skip { position: absolute; left: -999px; top: 0; background: var(--card); padding: 8px 12px; z-index: 10; }
.skip:focus { left: 8px; top: 8px; }

/* header */
.site-header { position: sticky; top: 0; z-index: 5; background: color-mix(in srgb, var(--bg) 92%, transparent); border-bottom: 1px solid var(--border); backdrop-filter: blur(6px); }
.bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 56px; }
.brand { display: inline-flex; align-items: center; gap: 10px; color: var(--text); text-decoration: none; font-weight: 700; }
nav { display: flex; align-items: center; gap: 4px 16px; flex-wrap: wrap; justify-content: flex-end; }
nav a { color: var(--muted); text-decoration: none; font-size: .95rem; }
nav a:hover { color: var(--text); }
@media (max-width: 560px) { nav a:not(.btn) { display: none; } }

/* buttons */
.btn { display: inline-block; background: linear-gradient(90deg, var(--accent-a), var(--accent-b)); color: #fff; font-weight: 600; text-decoration: none; padding: 12px 22px; border-radius: 10px; }
.btn:hover { filter: brightness(1.08); }
.btn-small { padding: 6px 14px; font-size: .9rem; color: #fff !important; }
.btn-ghost { background: transparent; color: var(--text); border: 1px solid var(--border); }

/* sections */
section { padding: 56px 0; }
.band { background: var(--bg-band); border-block: 1px solid var(--border); }
h1 { font-size: clamp(2rem, 6vw, 3.4rem); line-height: 1.15; margin: 16px 0; }
h2 { font-size: clamp(1.5rem, 4vw, 2rem); margin: 0 0 24px; }
h3 { font-size: 1.05rem; margin: 0 0 6px; }
.grad { background: linear-gradient(90deg, var(--accent-a), var(--accent-b)); -webkit-background-clip: text; background-clip: text; color: transparent; }
.muted { color: var(--muted); font-size: .9rem; }

.hero { text-align: center; padding: 64px 0 72px; }
.hero-logo { display: block; margin: 0 auto; border-radius: 24px; }
.lead { max-width: 640px; margin: 0 auto 28px; color: var(--muted); font-size: 1.15rem; }
.cta { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; margin: 0; }

blockquote { margin: 0 0 16px; padding: 4px 0 4px 18px; border-left: 4px solid var(--accent-b); color: var(--muted); font-style: italic; }
blockquote p { margin: 0; }

/* diagram */
.diagram { display: block; margin: 0 auto 28px; max-width: 760px; }
.diagram .node rect { fill: var(--card); stroke: var(--border); stroke-width: 1.5; }
.diagram .t-head { fill: var(--text); font: 600 16px var(--font); }
.diagram .t-sub { fill: var(--muted); font: 13px var(--font); }
.diagram .t-edge { fill: var(--accent-b); font: 700 14px var(--font); }
.diagram .edge { stroke: var(--accent-b); stroke-width: 2; }
.diagram .arrowhead { fill: var(--accent-b); }

.steps { padding-left: 1.2em; display: grid; gap: 12px; max-width: 760px; margin: 0 auto; }

/* cards */
.cards { list-style: none; margin: 0; padding: 0; display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
.cards li { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 18px; }
.cards p { margin: 0; color: var(--muted); }

/* screenshots */
.shots { display: grid; gap: 20px; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); }
.shots figure { margin: 0; background: var(--card); border: 1px solid var(--border); border-radius: 12px; overflow: hidden; }
.shots img { display: block; width: 100%; }
.shots figcaption { padding: 10px 14px; color: var(--muted); font-size: .92rem; }

/* code blocks */
.code { position: relative; margin: 8px 0 20px; }
.code pre { margin: 0; padding: 14px 76px 14px 14px; background: var(--code-bg); border: 1px solid var(--border); border-radius: 10px; overflow-x: auto; }
.code code { background: none; padding: 0; overflow-wrap: normal; }
.copy { position: absolute; top: 8px; right: 8px; background: var(--card); color: var(--text); border: 1px solid var(--border); border-radius: 8px; padding: 4px 10px; font: inherit; font-size: .85rem; cursor: pointer; }
.copy[hidden] { display: none; }

.site-footer { border-top: 1px solid var(--border); padding: 28px 0 40px; text-align: center; }
.site-footer p { margin: 6px 0; }
```

- [ ] **Step 4: Create `site/main.js`** (progressive enhancement only; the page works without it):

```js
// Reveal the copy buttons only when JavaScript and the Clipboard API exist.
(function () {
  if (!navigator.clipboard || !navigator.clipboard.writeText) return;

  document.querySelectorAll('button.copy[data-copy-target]').forEach(function (button) {
    var target = document.getElementById(button.getAttribute('data-copy-target'));
    if (!target) return;
    button.hidden = false;
    button.addEventListener('click', function () {
      navigator.clipboard.writeText(target.textContent.trim()).then(function () {
        button.textContent = 'Copied';
        setTimeout(function () { button.textContent = 'Copy'; }, 1500);
      }).catch(function () {
        button.textContent = 'Press Ctrl+C';
      });
    });
  });
})();
```

- [ ] **Step 5: Run the checker**

Run: `python3 scripts/check_site.py && du -sh site`
Expected: `OK: site passes all checks`. Fix any reported problem (most likely a figure whose image was omitted in Task 2, or an `alt` you removed). Total `site/` size must be under 1.5 MB.

- [ ] **Step 6: Commit**

```bash
git add site/index.html site/styles.css site/main.js
git commit -m "feat(site): add the Vanish landing page"
```

---

### Task 4: Pages deployment workflow

**Files:**
- Create: `.github/workflows/pages.yml`

**Interfaces:**
- Consumes: `scripts/check_site.py`, `site/` (Tasks 1–3).
- Produces: a workflow named `Deploy site to GitHub Pages` that runs on pushes to `main` touching `site/**`, `scripts/check_site.py`, or the workflow itself, plus manual dispatch. It does not touch `cli-ci.yml` or `docker-publish.yml`.

- [ ] **Step 1: Create `.github/workflows/pages.yml`:**

```yaml
name: Deploy site to GitHub Pages

on:
  push:
    branches: [main]
    paths:
      - 'site/**'
      - 'scripts/check_site.py'
      - '.github/workflows/pages.yml'
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - uses: actions/checkout@v4

      - name: Check site rules
        run: |
          python3 -I -m unittest discover -s scripts -p 'test_check_site.py'
          python3 scripts/check_site.py site

      - uses: actions/configure-pages@v5

      - uses: actions/upload-pages-artifact@v3
        with:
          path: site

      - id: deployment
        uses: actions/deploy-pages@v4
```

- [ ] **Step 2: Validate the YAML locally**

Run: `python3 -c "import yaml,sys; d=yaml.safe_load(open('.github/workflows/pages.yml')); print(list(d['jobs']['deploy']['steps'][0].keys())); print(d[True]['push']['paths'])"`
Expected: prints the step keys and the three paths (YAML parses `on` as `True`). If PyYAML is missing, instead run `python3 - <<'EOF'` with a plain read and eyeball the indentation, or `ruby -ryaml -e 'YAML.load_file(".github/workflows/pages.yml")'`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/pages.yml
git commit -m "ci(site): deploy site/ to GitHub Pages"
```

---

### Task 5: Local verification

**Files:** none changed (fixes go back to Tasks 2–3 files).

- [ ] **Step 1: Serve locally**

Run (from the repo root, in the background): `python3 -m http.server 8099 --directory site`
Open `http://localhost:8099/`.

- [ ] **Step 2: Automated checks**

```bash
python3 -I -m unittest discover -s scripts -p 'test_check_site.py'
python3 scripts/check_site.py site
curl -s http://localhost:8099/ | grep -o -E '(src|href)="https?://[^"]*"' | sort -u
```
Expected: tests OK; checker OK; the last command lists only `<a href>` links to `github.com` / `hub.docker.com` plus the canonical and `og:*` URLs on `vanish.codeforlife.dev`; no `src=` entries at all.

- [ ] **Step 3: Visual checks.** Use a browser tool or the repo owner's browser (Claude in Chrome / built-in browser skill if available; otherwise ask the owner to confirm) at 1280 px and 375 px width, in both color schemes (emulate `prefers-color-scheme`). Confirm: no horizontal scroll at 375 px (code blocks scroll inside their box, the diagram scales), hero, diagram, cards and screenshots render, focus rings are visible when tabbing, the skip link works, the copy buttons appear and copy, and with JavaScript disabled the page is complete minus the buttons. Check the browser console is empty and the network panel shows requests only to `localhost:8099`.

- [ ] **Step 4: Accessibility/performance pass.** If Lighthouse is available (`npx lighthouse http://localhost:8099 --only-categories=accessibility,performance,best-practices,seo --quiet --chrome-flags="--headless"`), target ≥ 95 for accessibility, best practices and SEO. Otherwise check contrast of `--muted` text on `--bg`/`--bg-band`/`--card` in both themes (≥ 4.5:1) with any contrast tool and fix variables in `styles.css` as needed.

- [ ] **Step 5: Stop the server and commit any fixes**

```bash
kill %1 2>/dev/null || true
git status --short
git add site && git commit -m "fix(site): address verification findings" || true
```

---

### Task 6: Publish (REQUIRES the repo owner's explicit confirmation at each step)

Everything above is local. Present the repo owner with: the screenshot review decisions (Task 2 Step 1), the final page (local preview or screenshots), and `git diff --stat main...HEAD`. **Do not run any step below until they say yes to that specific step.**

- [ ] **Step 1: Confirm prerequisites with the owner.** (a) The repository `zafrem/Vanish` is public, or their GitHub plan allows Pages on it. (b) They will create the DNS record. (c) They are happy with the screenshots that are included.

- [ ] **Step 2 (needs approval): Push the branch and open a PR to `main`.**

```bash
git push -u origin feat/showcase-site
gh pr create --title "Add Vanish showcase site (GitHub Pages)" --body "Adds site/, a site checker and a Pages deploy workflow. See docs/superpowers/specs/2026-10-13-showcase-site-design.md.

🤖 Generated with [Claude Code](https://claude.com/claude-code)"
```

- [ ] **Step 3 (needs approval): Enable Pages with the Actions source.**

```bash
gh api -X POST repos/zafrem/Vanish/pages -f build_type=workflow
```
(If Pages already exists: `gh api -X PUT repos/zafrem/Vanish/pages -f build_type=workflow`.)

- [ ] **Step 4 (needs approval): Merge the PR** (the owner may prefer to merge it themselves). The workflow runs on `main`; watch it with `gh run watch` and confirm it succeeds. The site is then live at `https://zafrem.github.io/Vanish/` (assets use relative paths, so this works) until the custom domain resolves.

- [ ] **Step 5 (owner action): DNS.** The owner adds, at their `codeforlife.dev` DNS provider: `vanish` CNAME → `zafrem.github.io` (no trailing path). Verify when propagated: `dig +short CNAME vanish.codeforlife.dev` returns `zafrem.github.io.`.

- [ ] **Step 6 (needs approval, after DNS): Set the custom domain and enforce HTTPS.**

```bash
gh api -X PUT repos/zafrem/Vanish/pages -f cname=vanish.codeforlife.dev
# After GitHub issues the certificate (can take minutes to hours):
gh api -X PUT repos/zafrem/Vanish/pages -F https_enforced=true
```

- [ ] **Step 7: Verify the live site.**

```bash
curl -sI https://vanish.codeforlife.dev/ | head -5
curl -s https://vanish.codeforlife.dev/ | grep -c "<title>Vanish"
```
Expected: `HTTP/2 200` and `1`. Also open the URL in a browser, check the padlock/certificate and the console. If the certificate is not ready yet, report that status instead of claiming success.

---

## Self-Review (completed)

- **Spec coverage:** layout and page sections (Task 3); design/accessibility/privacy constraints (Global Constraints, Tasks 1, 3, 5); screenshot review before publishing (Task 2); Pages workflow with minimal permissions and path filter (Task 4); `CNAME` (Task 2); DNS, public-repo caveat and no publishing without confirmation (Task 6); verification list (Task 5, plus the checker); copy limitations and no unshipped features (Global Constraints, banned-phrase check).
- **Placeholder scan:** none. Per-image alt text and figures are explicitly adjusted to the actual screenshots in Task 3, after the Task 2 review decides which images exist.
- **Consistency:** asset file names (`logo.png`, `favicon.png`, `og.png`, `shot-*.jpg`) match between Tasks 2 and 3; the checker's required files and CNAME value match the spec; the workflow runs the same commands as Task 5.
- **Known caveats:** the Pages API calls in Task 6 need `gh` authenticated with admin rights on the repo; certificate issuance and DNS propagation cannot be verified in advance.
