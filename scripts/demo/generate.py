#!/usr/bin/env python3
"""Generate a self-contained set of demo feeds for local testing and screenshots.

Creates RSS 2.0 and Atom feeds (with images, missing images, long headlines,
a podcast enclosure, a very large feed and deliberately malicious HTML) plus
an OPML file that groups them into folders. Everything is served from one
local directory, so no internet access is needed:

    python3 scripts/demo/generate.py /tmp/yarrow-demo http://127.0.0.1:8099
    python3 -m http.server 8099 --bind 127.0.0.1 --directory /tmp/yarrow-demo
    YARR_ALLOWED_NETWORKS=127.0.0.1 ./yarrow -db demo.db
    # then import /tmp/yarrow-demo/demo.opml in Settings

Images are drawn procedurally (requires Pillow). All text is original.
"""

import datetime as dt
import html
import os
import random
import sys
from xml.sax.saxutils import escape

try:
    from PIL import Image, ImageDraw, ImageFilter
except ImportError:  # pragma: no cover
    Image = None

OUT = sys.argv[1] if len(sys.argv) > 1 else "/tmp/yarrow-demo"
BASE = (sys.argv[2] if len(sys.argv) > 2 else "http://127.0.0.1:8099").rstrip("/")
NOW = dt.datetime(2026, 10, 13, 18, 30, tzinfo=dt.timezone.utc)
rnd = random.Random(42)

PALETTES = [
    [(244, 214, 168), (231, 143, 92), (88, 64, 92), (36, 40, 66)],
    [(198, 226, 233), (124, 178, 196), (52, 98, 120), (22, 46, 58)],
    [(232, 238, 214), (156, 190, 128), (74, 116, 82), (32, 56, 44)],
    [(250, 226, 214), (226, 152, 160), (128, 86, 132), (50, 40, 70)],
    [(236, 230, 214), (196, 170, 120), (120, 96, 70), (52, 44, 40)],
    [(214, 222, 240), (150, 160, 214), (82, 86, 150), (34, 34, 72)],
]


def landscape(path, seed, w=1200, h=675):
    r = random.Random(seed)
    pal = r.choice(PALETTES)
    img = Image.new("RGB", (w, h), pal[0])
    d = ImageDraw.Draw(img)
    for y in range(h):  # sky gradient
        t = y / h
        c = tuple(int(pal[0][i] * (1 - t) + pal[1][i] * t) for i in range(3))
        d.line([(0, y), (w, y)], fill=c)
    sx, sy = r.randint(150, w - 150), r.randint(80, h // 2 - 40)
    d.ellipse([sx - 70, sy - 70, sx + 70, sy + 70], fill=(255, 244, 220))
    for layer, col in enumerate(pal[2:] + [tuple(max(0, c - 18) for c in pal[3])]):
        base = h * (0.55 + 0.14 * layer)
        pts = [(0, h)]
        x = 0
        phase = r.random() * 6
        amp = r.randint(30, 90)
        while x <= w:
            import math

            y = base + amp * math.sin(x / r.randint(120, 260) + phase) + r.randint(-8, 8)
            pts.append((x, y))
            x += 20
        pts.append((w, h))
        d.polygon(pts, fill=col)
    img = img.filter(ImageFilter.GaussianBlur(0.6))
    img.save(path, "JPEG", quality=82)


def tiny_pixel(path):
    Image.new("RGB", (1, 1), (255, 255, 255)).save(path, "GIF")


FEEDS = {
    "Technology": [
        ("circuit-notes", "Circuit Notes", "rss", 24, True),
        ("open-source-weekly", "Open Source Weekly", "atom", 18, True),
        ("pixel-and-silicon", "Pixel & Silicon", "rss", 1500, True),  # very large feed
    ],
    "Cybersecurity": [
        ("patch-tuesday-digest", "Patch Tuesday Digest", "rss", 16, False),
        ("threat-ledger", "Threat Ledger", "rss", 14, True),
    ],
    "Finance": [
        ("ledger-and-lantern", "Ledger & Lantern", "rss", 15, True),
    ],
    "World News": [
        ("harbor-dispatch", "Harbor Dispatch", "atom", 22, True),
    ],
    "Travel": [
        ("slow-trains", "Slow Trains", "rss", 12, True),
        ("field-recordings-podcast", "Field Recordings", "rss", 6, False),  # podcast
    ],
    "Empty Folder": [],
    None: [("the-margins", "The Margins", "rss", 10, False)],  # no folder
}

HEADLINES = {
    "Technology": [
        "A practical guide to running your own home server on a single-board computer",
        "Why the next generation of laptops is betting on efficiency cores",
        "Inside the effort to make web pages lighter without losing what makes them useful",
        "The quiet comeback of RSS among developers who are tired of algorithmic feeds",
        "SQLite turns out to be the right database for far more apps than you'd think",
        "What a decade of container tooling taught us about simple deployments",
        "Accessibility audits are getting easier, but the hard part is still people",
        "A new open hardware board aims at hobbyists who want ARM64 on a budget",
    ],
    "Cybersecurity": [
        "Critical update for a widely used VPN appliance after remote access flaw is disclosed",
        "How server-side request forgery keeps slipping into feature after feature",
        "Browser vendors agree on a timeline for retiring a legacy cookie behaviour",
        "Small offices are the new favourite target for credential-stuffing campaigns",
        "A field guide to reading security advisories without panicking",
    ],
    "Finance": [
        "Central banks signal patience as inflation cools more slowly than hoped",
        "What rising storage prices mean for small cloud customers",
        "Index funds turn fifty: the experiment that changed retail investing",
        "Household budgets adapt to a year of uneven energy costs",
    ],
    "World News": [
        "Coastal cities test floating barriers ahead of the winter storm season",
        "Regional leaders reach a provisional deal on shared river management",
        "Rail strike ends after negotiators agree on a two-year pay framework",
        "Scientists map a newly discovered reef system off a remote archipelago",
    ],
    "Travel": [
        "Twelve hours on the slowest train in the mountains, and why it was worth it",
        "A walking route between three villages that still bake bread in shared ovens",
        "Night ferries are back, and they are better than the airport",
        "How to pack for a week with only a small backpack",
    ],
    None: [
        "Notes on attention, and on the tools that try to take it from us",
        "A reading list for the colder months",
    ],
}

PARAGRAPHS = [
    "The change did not arrive all at once. It crept in through small decisions, each reasonable on its own, until the overall picture looked very different from what anyone had planned at the start.",
    "Engineers involved in the project describe a familiar pattern: an early prototype that worked well enough, a period of rapid growth, and then a long stretch of careful consolidation.",
    "Critics point out that the approach trades flexibility for predictability. Supporters say that is exactly the point, and that most people never needed the flexibility in the first place.",
    "For readers who want to try it themselves, the requirements are modest. A spare evening, a little patience, and a willingness to read the documentation twice are usually enough.",
    "What happens next depends on choices that have not been made yet. The people closest to the work are cautiously optimistic, and unusually candid about what could still go wrong.",
    "Local residents have been watching closely. Several said they appreciated being consulted early, even when the answers were not the ones they had hoped to hear.",
]


def article_html(slug, i, with_image, long=False):
    parts = []
    if with_image:
        parts.append(f'<p><img src="{BASE}/images/{slug}-{i}.jpg" alt="" width="1200" height="675"></p>')
    k = rnd.randint(3, 6)
    for p in rnd.sample(PARAGRAPHS, k):
        parts.append(f"<p>{p}</p>")
    if i % 4 == 0:
        parts.insert(2, "<h2>What changed</h2><ul><li>Fewer moving parts</li><li>Clearer defaults</li><li>Better documentation</li></ul>")
    if i % 5 == 0:
        parts.append("<blockquote>Simple systems are easier to trust, and easier to fix when they break.</blockquote>")
    # tracking pixel that must be stripped
    parts.append(f'<img src="{BASE}/images/pixel.gif" width="1" height="1">')
    return "\n".join(parts)


def malicious_html():
    return (
        '<p>This item contains hostile markup that the sanitizer must neutralise.</p>'
        '<script>document.body.dataset.pwned="script"</script>'
        '<img src="x" onerror="document.body.dataset.pwned=\'onerror\'">'
        '<p><a href="javascript:document.body.dataset.pwned=\'link\'">a javascript: link</a></p>'
        '<iframe src="https://evil.example/frame"></iframe>'
        '<p style="position:fixed;top:0;left:0;width:100%;height:100%">overlay attempt</p>'
        '<svg><script>document.body.dataset.pwned="svg"</script></svg>'
    )


def rss(slug, title, items, folder):
    out = [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/" xmlns:content="http://purl.org/rss/1.0/modules/content/">',
        "<channel>",
        f"<title>{escape(title)}</title>",
        f"<link>{BASE}/{slug}/</link>",
        f"<description>Demo feed: {escape(title)}</description>",
    ]
    for it in items:
        out.append("<item>")
        out.append(f"<title>{escape(it['title'])}</title>")
        out.append(f"<link>{it['link']}</link>")
        out.append(f"<guid isPermaLink=\"false\">{it['guid']}</guid>")
        out.append(f"<pubDate>{it['date'].strftime('%a, %d %b %Y %H:%M:%S +0000')}</pubDate>")
        if it.get("summary_only"):
            out.append(f"<description>{escape(it['summary'])}</description>")
        else:
            out.append(f"<description>{escape(it['summary'])}</description>")
            out.append(f"<content:encoded><![CDATA[{it['html']}]]></content:encoded>")
        if it.get("media"):
            out.append(f'<media:content url="{it["media"]}" medium="image" type="image/jpeg" />')
        if it.get("audio"):
            out.append(f'<enclosure url="{it["audio"]}" length="1000" type="audio/mpeg" />')
        out.append("</item>")
    out += ["</channel>", "</rss>"]
    return "\n".join(out)


def atom(slug, title, items):
    out = [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<feed xmlns="http://www.w3.org/2005/Atom">',
        f"<title>{escape(title)}</title>",
        f'<link href="{BASE}/{slug}/" />',
        f"<id>urn:yarrow-demo:{slug}</id>",
        f"<updated>{NOW.isoformat()}</updated>",
    ]
    for it in items:
        out.append("<entry>")
        out.append(f"<title>{escape(it['title'])}</title>")
        out.append(f'<link href="{it["link"]}" />')
        out.append(f"<id>{it['guid']}</id>")
        out.append(f"<updated>{it['date'].isoformat()}</updated>")
        out.append(f'<content type="html">{escape(it["html"])}</content>')
        out.append("</entry>")
    out.append("</feed>")
    return "\n".join(out)


def main():
    os.makedirs(os.path.join(OUT, "images"), exist_ok=True)
    os.makedirs(os.path.join(OUT, "feeds"), exist_ok=True)
    if Image:
        tiny_pixel(os.path.join(OUT, "images", "pixel.gif"))
    opml = ['<?xml version="1.0" encoding="UTF-8"?>', '<opml version="1.1">', "<head><title>Yarrow demo</title></head>", "<body>"]
    minute = 0
    for folder, feeds in FEEDS.items():
        if folder:
            opml.append(f'<outline text="{html.escape(folder)}" title="{html.escape(folder)}">')
        for slug, title, kind, count, images in feeds:
            items = []
            heads = HEADLINES.get(folder, HEADLINES[None])
            for i in range(count):
                minute += rnd.randint(9, 47) if count < 100 else 7
                date = NOW - dt.timedelta(minutes=minute if count < 100 else minute + i * 30)
                head = heads[i % len(heads)]
                if count >= 100:
                    head = f"{head} (part {i + 1})"
                if i == 1 and slug == "circuit-notes":
                    head = ("An unusually long headline that keeps going to check how cards wrap text across many lines "
                            "on a narrow phone screen without breaking the layout or hiding the menu button")
                with_img = images and (i % 3 != 2)
                if with_img and Image and count < 100:
                    landscape(os.path.join(OUT, "images", f"{slug}-{i}.jpg"), f"{slug}-{i}")
                elif with_img and count >= 100:
                    with_img = False
                it = {
                    "title": head,
                    "link": f"{BASE}/{slug}/{i}.html",
                    "guid": f"{slug}-{i}",
                    "date": date,
                    "html": article_html(slug, i, with_img),
                    "summary": PARAGRAPHS[i % len(PARAGRAPHS)],
                }
                if slug == "patch-tuesday-digest":
                    it["summary_only"] = True  # short summary only, no full content
                if slug == "threat-ledger" and i == 0:
                    it["title"] = "Sanitizer check: this article contains hostile HTML"
                    it["html"] = malicious_html()
                if slug == "field-recordings-podcast":
                    it["title"] = f"Episode {count - i}: {head}"
                    it["html"] = f"<p>{PARAGRAPHS[i % len(PARAGRAPHS)]}</p>"
                    it["audio"] = f"{BASE}/audio/episode-{i}.mp3"
                items.append(it)
            body = atom(slug, title, items) if kind == "atom" else rss(slug, title, items, folder)
            path = os.path.join(OUT, "feeds", f"{slug}.xml")
            with open(path, "w") as f:
                f.write(body)
            line = f'<outline type="rss" text="{html.escape(title)}" title="{html.escape(title)}" xmlUrl="{BASE}/feeds/{slug}.xml" htmlUrl="{BASE}/{slug}/"/>'
            opml.append(("  " if folder else "") + line)
        if folder:
            opml.append("</outline>")
    opml += ["</body>", "</opml>"]
    with open(os.path.join(OUT, "demo.opml"), "w") as f:
        f.write("\n".join(opml))
    print(f"wrote demo feeds to {OUT}")


if __name__ == "__main__":
    main()
