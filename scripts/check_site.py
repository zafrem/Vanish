#!/usr/bin/env python3
"""Checks the Vanish showcase site against its rules. Standard library only."""
import os
import re
import sys
from html.parser import HTMLParser

CNAME = "vanish.codeforlife.dev"
MAX_BYTES = 1_500_000
EXTERNAL_LINK_HOSTS = {"github.com", "hub.docker.com", "buymeacoffee.com"}
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
