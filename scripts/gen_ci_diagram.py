#!/usr/bin/env python3
"""Generates the CI/CD handoff diagram used in the README, Docker Hub page and site.

Writes, from one source of truth:
  images/ci-handoff.svg   themed (light/dark) SVG for the README
  images/ci-handoff.png   light PNG for Docker Hub (needs Defaulting to user installation because normal site-packages is not writeable
Collecting cairosvg
  Downloading cairosvg-2.9.1-py3-none-any.whl.metadata (2.7 kB)
Collecting cairocffi (from cairosvg)
  Downloading cairocffi-1.7.1-py3-none-any.whl.metadata (3.3 kB)
Collecting cssselect2 (from cairosvg)
  Downloading cssselect2-0.10.1-py3-none-any.whl.metadata (2.9 kB)
Requirement already satisfied: defusedxml in /home/baruna/.local/lib/python3.10/site-packages (from cairosvg) (0.7.1)
Requirement already satisfied: pillow in /home/baruna/.local/lib/python3.10/site-packages (from cairosvg) (12.2.0)
Requirement already satisfied: tinycss2 in /home/baruna/.local/lib/python3.10/site-packages (from cairosvg) (1.2.1)
Requirement already satisfied: cffi>=1.1.0 in /home/baruna/.local/lib/python3.10/site-packages (from cairocffi->cairosvg) (1.16.0)
Requirement already satisfied: pycparser in /home/baruna/.local/lib/python3.10/site-packages (from cffi>=1.1.0->cairocffi->cairosvg) (2.22)
Requirement already satisfied: webencodings in /home/baruna/.local/lib/python3.10/site-packages (from cssselect2->cairosvg) (0.5.1)
Downloading cairosvg-2.9.1-py3-none-any.whl (46 kB)
Downloading cairocffi-1.7.1-py3-none-any.whl (75 kB)
Downloading cssselect2-0.10.1-py3-none-any.whl (15 kB)
Installing collected packages: cssselect2, cairocffi, cairosvg

Successfully installed cairocffi-1.7.1 cairosvg-2.9.1 cssselect2-0.10.1)
  site/index.html         refreshes the inline <svg> that has id "ci-title"

Run from anywhere:  python3 scripts/gen_ci_diagram.py [--no-png]
"""
import argparse
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TITLE = "How a CI/CD job hands a secret to a teammate"
DESC = ("A CI/CD job encrypts a credential and uploads only the ciphertext to the Vanish server. "
        "The job shares just the one-time link with a teammate through the build log or chat. "
        "The teammate opens the link once, and the server deletes the message on that first read.")

def body(mid):
    return f'''<defs>
    <marker id="{mid}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
      <path d="M0 0 L10 5 L0 10 z" class="arrowhead"/>
    </marker>
  </defs>
  <g class="node">
    <rect x="10" y="40" width="170" height="100" rx="12"/>
    <text x="95" y="76" text-anchor="middle" class="t-head">CI/CD job</text>
    <text x="95" y="100" text-anchor="middle" class="t-sub">creates a credential,</text>
    <text x="95" y="118" text-anchor="middle" class="t-sub">runs vanish send</text>
  </g>
  <g class="node">
    <rect x="295" y="40" width="170" height="100" rx="12"/>
    <text x="380" y="76" text-anchor="middle" class="t-head">Vanish server</text>
    <text x="380" y="100" text-anchor="middle" class="t-sub">holds ciphertext</text>
    <text x="380" y="118" text-anchor="middle" class="t-sub">and an expiry</text>
  </g>
  <g class="node">
    <rect x="580" y="40" width="170" height="100" rx="12"/>
    <text x="665" y="76" text-anchor="middle" class="t-head">Teammate</text>
    <text x="665" y="100" text-anchor="middle" class="t-sub">opens the link once,</text>
    <text x="665" y="118" text-anchor="middle" class="t-sub">then it is gone</text>
  </g>
  <line x1="180" y1="90" x2="293" y2="90" class="edge" marker-end="url(#{mid})"/>
  <line x1="465" y1="90" x2="578" y2="90" class="edge" marker-end="url(#{mid})"/>
  <text x="237" y="78" text-anchor="middle" class="t-edge">1 ciphertext</text>
  <text x="522" y="78" text-anchor="middle" class="t-edge">3 read once</text>
  <path d="M95 140 V190 H665 V144" fill="none" class="edge" stroke-dasharray="6 5" marker-end="url(#{mid})"/>
  <text x="380" y="180" text-anchor="middle" class="t-edge">2 link only</text>
  <text x="380" y="214" text-anchor="middle" class="t-sub">The build log or chat carries the link. The key stays in the #fragment.</text>'''

STYLE = '''<style>
    .node rect { fill: #ffffff; stroke: #d5deec; stroke-width: 1.5; }
    .t-head { fill: #0f1b33; font: 600 16px system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
    .t-sub { fill: #4c5b78; font: 13px system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
    .t-edge { fill: #ea580c; font: 700 14px system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
    .edge { stroke: #ea580c; stroke-width: 2; }
    .arrowhead { fill: #ea580c; }
    .bg { fill: #ffffff; }
    @media (prefers-color-scheme: dark) {
      .node rect { fill: #16213a; stroke: #26344f; }
      .t-head { fill: #e6ebf5; }
      .t-sub { fill: #9aa8c2; }
      .t-edge { fill: #f97316; }
      .edge { stroke: #f97316; }
      .arrowhead { fill: #f97316; }
      .bg { fill: #0b1220; }
    }
  </style>'''

def standalone(bg=True, dark_media=True):
    st = STYLE if dark_media else STYLE.split("@media")[0] + "</style>"
    r = '<rect class="bg" width="760" height="230"/>\n  ' if bg else ''
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 760 230" width="760" height="230" role="img" aria-labelledby="t d">\n'
            f'  <title id="t">{TITLE}</title>\n  <desc id="d">{DESC}</desc>\n  {st}\n  {r}{body("arrow")}\n</svg>\n')

def inline():
    return (f'<svg class="diagram" viewBox="0 0 760 230" role="img" aria-labelledby="ci-title ci-desc">\n'
            f'          <title id="ci-title">{TITLE}</title>\n          <desc id="ci-desc">{DESC}</desc>\n          {body("arrow-ci")}\n        </svg>')

TITLE = "How a CI/CD job hands a secret to a teammate"
DESC = ("A CI/CD job encrypts a credential and uploads only the ciphertext to the Vanish server. "
        "The job shares just the one-time link with a teammate through the build log or chat. "
        "The teammate opens the link once, and the server deletes the message on that first read.")

def body(mid):
    return f'''<defs>
    <marker id="{mid}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
      <path d="M0 0 L10 5 L0 10 z" class="arrowhead"/>
    </marker>
  </defs>
  <g class="node">
    <rect x="10" y="40" width="170" height="100" rx="12"/>
    <text x="95" y="76" text-anchor="middle" class="t-head">CI/CD job</text>
    <text x="95" y="100" text-anchor="middle" class="t-sub">creates a credential,</text>
    <text x="95" y="118" text-anchor="middle" class="t-sub">runs vanish send</text>
  </g>
  <g class="node">
    <rect x="295" y="40" width="170" height="100" rx="12"/>
    <text x="380" y="76" text-anchor="middle" class="t-head">Vanish server</text>
    <text x="380" y="100" text-anchor="middle" class="t-sub">holds ciphertext</text>
    <text x="380" y="118" text-anchor="middle" class="t-sub">and an expiry</text>
  </g>
  <g class="node">
    <rect x="580" y="40" width="170" height="100" rx="12"/>
    <text x="665" y="76" text-anchor="middle" class="t-head">Teammate</text>
    <text x="665" y="100" text-anchor="middle" class="t-sub">opens the link once,</text>
    <text x="665" y="118" text-anchor="middle" class="t-sub">then it is gone</text>
  </g>
  <line x1="180" y1="90" x2="293" y2="90" class="edge" marker-end="url(#{mid})"/>
  <line x1="465" y1="90" x2="578" y2="90" class="edge" marker-end="url(#{mid})"/>
  <text x="237" y="78" text-anchor="middle" class="t-edge">1 ciphertext</text>
  <text x="522" y="78" text-anchor="middle" class="t-edge">3 read once</text>
  <path d="M95 140 V190 H665 V144" fill="none" class="edge" stroke-dasharray="6 5" marker-end="url(#{mid})"/>
  <text x="380" y="180" text-anchor="middle" class="t-edge">2 link only</text>
  <text x="380" y="214" text-anchor="middle" class="t-sub">The build log or chat carries the link. The key stays in the #fragment.</text>'''

STYLE = '''<style>
    .node rect { fill: #ffffff; stroke: #d5deec; stroke-width: 1.5; }
    .t-head { fill: #0f1b33; font: 600 16px system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
    .t-sub { fill: #4c5b78; font: 13px system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
    .t-edge { fill: #ea580c; font: 700 14px system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
    .edge { stroke: #ea580c; stroke-width: 2; }
    .arrowhead { fill: #ea580c; }
    .bg { fill: #ffffff; }
    @media (prefers-color-scheme: dark) {
      .node rect { fill: #16213a; stroke: #26344f; }
      .t-head { fill: #e6ebf5; }
      .t-sub { fill: #9aa8c2; }
      .t-edge { fill: #f97316; }
      .edge { stroke: #f97316; }
      .arrowhead { fill: #f97316; }
      .bg { fill: #0b1220; }
    }
  </style>'''

def standalone(bg=True, dark_media=True):
    st = STYLE if dark_media else STYLE.split("@media")[0] + "</style>"
    r = '<rect class="bg" width="760" height="230"/>\n  ' if bg else ''
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 760 230" width="760" height="230" role="img" aria-labelledby="t d">\n'
            f'  <title id="t">{TITLE}</title>\n  <desc id="d">{DESC}</desc>\n  {st}\n  {r}{body("arrow")}\n</svg>\n')

def inline():
    return (f'<svg class="diagram" viewBox="0 0 760 230" role="img" aria-labelledby="ci-title ci-desc">\n'
            f'          <title id="ci-title">{TITLE}</title>\n          <desc id="ci-desc">{DESC}</desc>\n          {body("arrow-ci")}\n        </svg>')


def refresh_site(path):
    with open(path, encoding="utf-8") as f:
        html = f.read()
    pattern = re.compile(r'<svg class="diagram"[^>]*aria-labelledby="ci-title.*?</svg>', re.S)
    if not pattern.search(html):
        sys.exit("inline CI/CD diagram not found in " + path)
    new = pattern.sub(lambda m: inline(), html, count=1)
    if new != html:
        with open(path, "w", encoding="utf-8") as f:
            f.write(new)
    return new != html


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--no-png", action="store_true", help="skip the PNG (no cairosvg needed)")
    args = ap.parse_args()

    svg_path = os.path.join(ROOT, "images", "ci-handoff.svg")
    with open(svg_path, "w", encoding="utf-8") as f:
        f.write(standalone())
    print("wrote", svg_path)

    if not args.no_png:
        try:
            import cairosvg
        except ImportError:
            sys.exit("cairosvg is required for the PNG: pip install cairosvg (or pass --no-png)")
        png_path = os.path.join(ROOT, "images", "ci-handoff.png")
        cairosvg.svg2png(bytestring=standalone(dark_media=False).encode(), write_to=png_path, scale=2)
        print("wrote", png_path)

    changed = refresh_site(os.path.join(ROOT, "site", "index.html"))
    print("site/index.html", "updated" if changed else "already up to date")


if __name__ == "__main__":
    main()
