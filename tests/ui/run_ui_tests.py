#!/usr/bin/env python3
"""End-to-end browser tests for Yarrow (Playwright + Chromium).

Starts a throw-away Yarrow server and a local demo-feed server, imports the
demo OPML, then drives the UI at phone, tablet and desktop sizes. Screenshots
of every major screen are written to docs/screenshots/.

    pip install playwright && python -m playwright install chromium
    make build            # or: go build ... -o out/yarrow ./cmd/yarr
    python3 tests/ui/run_ui_tests.py --binary out/yarrow

Exit status is non-zero if any test fails. WebKit (real iOS Safari engine)
is used instead of Chromium when --browser webkit is given and installed.
"""

import argparse
import json
import re
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import traceback
import urllib.request

from playwright.sync_api import sync_playwright, expect

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SHOTS = os.path.join(ROOT, "docs", "screenshots")

IPHONE = dict(viewport={"width": 390, "height": 844}, device_scale_factor=3, is_mobile=True, has_touch=True,
              user_agent="Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
IPAD = dict(viewport={"width": 820, "height": 1180}, device_scale_factor=2, is_mobile=True, has_touch=True,
            user_agent="Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
DESKTOP = dict(viewport={"width": 1440, "height": 900}, device_scale_factor=1)


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


class Env:
    def __init__(self, binary):
        self.tmp = tempfile.mkdtemp(prefix="yarrow-ui-")
        self.feed_port = free_port()
        self.port = free_port()
        self.base = f"http://127.0.0.1:{self.port}/"
        feeds = os.path.join(self.tmp, "feeds")
        subprocess.run([sys.executable, os.path.join(ROOT, "scripts", "demo", "generate.py"), feeds,
                        f"http://127.0.0.1:{self.feed_port}"], check=True, stdout=subprocess.DEVNULL)
        self.feeds_dir = feeds
        self.http = subprocess.Popen([sys.executable, "-m", "http.server", str(self.feed_port), "--bind", "127.0.0.1",
                                      "--directory", feeds], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        env = dict(os.environ, YARR_ALLOWED_NETWORKS="127.0.0.1")
        self.log = open(os.path.join(self.tmp, "server.log"), "w")
        self.srv = subprocess.Popen([binary, "-db", os.path.join(self.tmp, "ui.db"), "-addr", f"127.0.0.1:{self.port}"],
                                    env=env, stdout=self.log, stderr=self.log)
        self.wait_http(self.base + "healthz")
        self.wait_http(f"http://127.0.0.1:{self.feed_port}/demo.opml")
        self.import_opml(os.path.join(feeds, "demo.opml"))
        self.wait_refresh()

    def wait_http(self, url, timeout=15):
        end = time.time() + timeout
        while time.time() < end:
            try:
                urllib.request.urlopen(url, timeout=2)
                return
            except Exception:
                time.sleep(0.2)
        raise RuntimeError("timeout waiting for " + url)

    def import_opml(self, path):
        boundary = "----yarrowtest"
        data = open(path, "rb").read()
        body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"opml\"; filename=\"demo.opml\"\r\n"
                f"Content-Type: text/xml\r\n\r\n").encode() + data + f"\r\n--{boundary}--\r\n".encode()
        req = urllib.request.Request(self.base + "opml/import", data=body, method="POST",
                                     headers={"Content-Type": f"multipart/form-data; boundary={boundary}"})
        urllib.request.urlopen(req)

    def api(self, path, method="GET", payload=None):
        data = json.dumps(payload).encode() if payload is not None else None
        req = urllib.request.Request(self.base + path, data=data, method=method,
                                     headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req) as r:
            body = r.read()
            return json.loads(body) if body.strip() else None

    def wait_refresh(self):
        time.sleep(1)
        for _ in range(100):
            st = self.api("api/status")
            if st["running"] == 0 and len(st["stats"]) >= 10:
                return
            time.sleep(0.3)
        raise RuntimeError("feeds did not refresh")

    def item_status(self, item_id):
        return self.api(f"api/items/{item_id}")["status"]

    def close(self):
        for p in (self.srv, self.http):
            p.terminate()
            p.wait(5)
        self.log.close()
        shutil.rmtree(self.tmp, ignore_errors=True)


TESTS = []


def test(fn):
    TESTS.append(fn)
    return fn


def open_app(browser, env, device, theme=None, nav_mode=None):
    if theme or nav_mode:
        patch = {}
        if theme:
            patch["theme_name"] = theme
        if nav_mode:
            patch["nav_mode"] = nav_mode
        env.api("api/settings", "PUT", patch)
    ctx = browser.new_context(**device)
    page = ctx.new_page()
    page.errors = []
    page.on("pageerror", lambda e: page.errors.append(str(e)))
    page.on("console", lambda m: page.errors.append(m.text) if m.type == "error" else None)
    page.goto(env.base)
    page.wait_for_selector(".card")
    page.wait_for_timeout(400)
    return ctx, page


def reset_view(env):
    env.api("api/settings", "PUT", {"feed": "", "filter": "", "theme_name": "light", "nav_mode": "folders",
                                    "sort_newest_first": True})


def shot(page, name):
    os.makedirs(SHOTS, exist_ok=True)
    page.wait_for_timeout(250)
    page.screenshot(path=os.path.join(SHOTS, name + ".png"), animations="disabled")


def no_horizontal_overflow(page):
    w = page.evaluate("[document.documentElement.scrollWidth, window.innerWidth]")
    assert w[0] <= w[1] + 1, f"horizontal overflow: {w}"


# ----------------------------------------------------------------- tests

@test
def phone_timeline_renders_cards(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    cards = page.locator(".card")
    assert cards.count() >= 10, cards.count()
    first = cards.first
    assert first.locator(".card-title").inner_text().strip()
    assert first.locator(".card-snippet").inner_text().strip(), "snippet missing"
    assert page.locator(".card .card-media").count() > 0, "no images rendered"
    assert page.locator(".card:not(.has-media)").count() > 0, "expected some image-less cards"
    assert page.locator(".tabbar").is_visible()
    assert not page.locator(".sidebar").is_visible()
    no_horizontal_overflow(page)
    shot(page, "phone-all-feeds")
    assert not [e for e in page.errors if "Content Security Policy" in e or "Refused" in e], page.errors
    assert not page.errors, page.errors
    ctx.close()


@test
def long_headline_wraps(b, env):
    reset_view(env)
    env.api("api/settings", "PUT", {"feed": "feed:" + str(feed_id(env, "Circuit Notes"))})
    ctx, page = open_app(b, env, IPHONE)
    long = page.locator(".card-title", has_text="An unusually long headline")
    expect(long).to_be_visible()
    box = long.bounding_box()
    assert box["width"] <= 390, box
    assert box["height"] > 60, "long headline should wrap onto several lines"
    no_horizontal_overflow(page)
    ctx.close()


def feed_id(env, title):
    return next(f["id"] for f in env.api("api/feeds") if f["title"] == title)


def folder_id(env, title):
    return next(f["id"] for f in env.api("api/folders") if f["title"] == title)


@test
def folder_only_navigation(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    chips = page.locator(".chips .chip")
    names = [c.inner_text().split("\n")[0].strip() for c in chips.all()]
    assert "Technology" in names and "Cybersecurity" in names, names
    # folder-only: no individual feed names anywhere in the navigation
    page.locator(".tabbar button", has_text="Folders").click()
    sheet = page.locator(".sheet")
    expect(sheet).to_be_visible()
    assert sheet.locator(".is-feed").count() == 0, "feeds must be hidden in folder-only mode"
    shot(page, "phone-folder-sheet")
    sheet.locator("button.label", has_text="Cybersecurity").click()
    expect(page.locator(".view-title")).to_contain_text("Cybersecurity")
    page.wait_for_timeout(500)
    allowed = {f["id"] for f in env.api("api/feeds") if f["folder_id"] == folder_id(env, "Cybersecurity")}
    names_in_cards = set(t.strip() for t in page.locator(".card .card-source b").all_text_contents())
    titles = {f["title"] for f in env.api("api/feeds") if f["id"] in allowed}
    assert names_in_cards and names_in_cards <= titles, (names_in_cards, titles)
    shot(page, "phone-folder-timeline")
    ctx.close()


@test
def infinite_scroll_without_duplicates(b, env):
    reset_view(env)
    env.api("api/settings", "PUT", {"feed": "folder:" + str(folder_id(env, "Technology"))})
    ctx, page = open_app(b, env, IPHONE)
    before = page.locator(".card").count()
    for _ in range(8):
        page.mouse.wheel(0, 6000)
        page.wait_for_timeout(350)
    ids = page.eval_on_selector_all(".card", "els => els.map(e => e.dataset.id)")
    assert len(ids) > before + 40, (before, len(ids))
    assert len(ids) == len(set(ids)), "duplicate cards after pagination"
    # dates must be non-increasing (newest first)
    ctx.close()


@test
def scroll_position_preserved_and_read_on_open_only(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    unread_before = sum(s["unread"] for s in env.api("api/status")["stats"])
    page.mouse.wheel(0, 2500)
    page.wait_for_timeout(500)
    # scrolling past articles must not mark them read
    assert sum(s["unread"] for s in env.api("api/status")["stats"]) == unread_before
    y = page.evaluate("window.scrollY")
    assert y > 1000, y
    card = page.locator(".card").nth(6)
    card.scroll_into_view_if_needed()
    page.wait_for_timeout(300)
    item_id = card.get_attribute("data-id")
    top = lambda: page.locator(f'.card[data-id="{item_id}"]').evaluate("e => e.getBoundingClientRect().top")
    before = top()
    card.locator(".card-title").click()
    expect(page.locator(".reader")).to_be_visible()
    expect(page.locator(".article h1")).to_be_visible()
    shot(page, "phone-reader")
    page.wait_for_timeout(300)
    assert env.item_status(item_id) == "read", "opening an article marks it read"
    page.locator(".reader-back").click()
    expect(page.locator(".reader")).to_have_count(0)
    # the card is exactly where it was on screen (scrollY itself may differ by
    # scroll anchoring when cards above finish rendering)
    assert abs(top() - before) < 4, f"position lost: {before} -> {top()}"
    assert "is-read" in page.locator(f'.card[data-id="{item_id}"]').get_attribute("class")
    # the browser back gesture closes the reader too
    page.locator(".card").nth(7).locator(".card-title").click()
    expect(page.locator(".reader")).to_be_visible()
    page.go_back()
    expect(page.locator(".reader")).to_have_count(0)
    assert abs(top() - before) < 40
    ctx.close()


@test
def article_menu_actions(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    card = page.locator(".card").nth(2)
    item_id = card.get_attribute("data-id")
    card.locator(".card-more").click()
    menu = page.locator(".menu")
    expect(menu).to_be_visible()
    labels = menu.locator(".menu-item").all_inner_texts()
    for want in ["Mark as read", "Save for later", "Open original", "Copy link", "Mark above as read"]:
        assert any(want in l for l in labels), (want, labels)
    shot(page, "phone-article-menu")
    menu.locator(".menu-item", has_text="Save for later").click()
    page.wait_for_timeout(300)
    assert env.item_status(item_id) == "starred"
    # Saved tab lists it
    page.locator(".tabbar button", has_text="Saved").click()
    expect(page.locator(f'.card[data-id="{item_id}"]')).to_be_visible()
    shot(page, "phone-saved")
    # mark as read / unread round trip on another card
    page.locator(".tabbar button", has_text="All").click()
    page.wait_for_timeout(400)
    c2 = page.locator(".card").nth(4)
    id2 = c2.get_attribute("data-id")
    status = env.item_status(id2)
    c2.locator(".card-more").click()
    page.locator(".menu .menu-item", has_text="Mark as").first.click()
    page.wait_for_timeout(300)
    assert env.item_status(id2) != status
    ctx.close()


@test
def mark_above_as_read(b, env):
    reset_view(env)
    env.api("api/settings", "PUT", {"feed": "folder:" + str(folder_id(env, "World News"))})
    env.api("api/items?folder_id=" + str(folder_id(env, "World News")), "PUT")  # start clean...
    # ...then make everything unread again
    for it in env.api(f"api/items?folder_id={folder_id(env, 'World News')}&limit=100")["list"]:
        env.api(f"api/items/{it['id']}", "PUT", {"status": "unread"})
    ctx, page = open_app(b, env, IPHONE)
    cards = page.locator(".card")
    target = cards.nth(3)
    ids = page.eval_on_selector_all(".card", "els => els.map(e => e.dataset.id)")
    target.locator(".card-more").click()
    page.locator(".menu .menu-item", has_text="Mark above as read").click()
    page.wait_for_timeout(500)
    for i in ids[:3]:
        assert env.item_status(i) == "read", i
    for i in ids[3:6]:
        assert env.item_status(i) == "unread", i
    ctx.close()


@test
def unread_filter(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    page.locator(".segmented button", has_text="Unread").click()
    page.wait_for_timeout(500)
    classes = page.eval_on_selector_all(".card", "els => els.map(e => e.className)")
    assert classes and all("is-read" not in c for c in classes)
    ctx.close()


@test
def empty_folder_state(b, env):
    reset_view(env)
    env.api("api/settings", "PUT", {"feed": "folder:" + str(folder_id(env, "Empty Folder"))})
    ctx = b.new_context(**IPHONE)
    page = ctx.new_page()
    page.goto(env.base)
    expect(page.locator(".timeline-state h2")).to_have_text("This folder is empty")
    shot(page, "phone-empty-folder")
    ctx.close()


@test
def malicious_html_is_neutralised(b, env):
    reset_view(env)
    env.api("api/settings", "PUT", {"feed": "feed:" + str(feed_id(env, "Threat Ledger"))})
    ctx, page = open_app(b, env, IPHONE)
    page.locator(".card", has_text="Sanitizer check").locator(".card-title").click()
    expect(page.locator(".article .content")).to_be_visible()
    page.wait_for_timeout(500)
    assert page.evaluate("document.body.dataset.pwned") is None, "script executed"
    content = page.locator(".article .content")
    assert content.locator("script").count() == 0
    html = content.inner_html()
    attrs = content.evaluate("""el => Array.from(el.querySelectorAll('*')).flatMap(n =>
        Array.from(n.attributes).map(a => a.name + '=' + a.value))""")
    assert not [a for a in attrs if a.startswith("on") or "javascript:" in a or "evil.example" in a or a.startswith("style=")], attrs
    assert content.locator("iframe").count() == 0
    for a in content.locator("a").all():
        assert a.get_attribute("rel") and "noopener" in a.get_attribute("rel")
    ctx.close()


@test
def tracking_pixels_removed(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    page.locator(".card.has-media").first.locator(".card-title").click()
    expect(page.locator(".article .content")).to_be_visible()
    assert "pixel.gif" not in page.locator(".article .content").inner_html()
    ctx.close()


@test
def dark_and_sepia_themes(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE, theme="night")
    bg = page.evaluate("getComputedStyle(document.body).backgroundColor")
    assert bg == "rgb(17, 22, 19)", bg
    shot(page, "phone-dark")
    page.locator(".card").first.locator(".card-title").click()
    expect(page.locator(".article h1")).to_be_visible()
    shot(page, "phone-dark-reader")
    ctx.close()
    ctx, page = open_app(b, env, IPHONE, theme="sepia")
    assert page.evaluate("getComputedStyle(document.body).backgroundColor") == "rgb(239, 230, 211)"
    ctx.close()
    # "system" follows the OS preference
    env.api("api/settings", "PUT", {"theme_name": "system"})
    ctx = b.new_context(**IPHONE, color_scheme="dark")
    page = ctx.new_page()
    page.goto(env.base)
    page.wait_for_selector(".card")
    assert page.evaluate("getComputedStyle(document.body).backgroundColor") == "rgb(17, 22, 19)"
    ctx.close()


@test
def tablet_layout(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPAD)
    no_horizontal_overflow(page)
    w = page.locator(".card").first.bounding_box()["width"]
    assert 500 < w <= 660, w
    shot(page, "tablet-all-feeds")
    ctx.close()


@test
def desktop_sidebar_and_modes(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, DESKTOP)
    assert page.locator(".sidebar").is_visible()
    assert not page.locator(".tabbar").is_visible()
    assert page.locator(".sidebar .is-feed").count() == 0
    shot(page, "desktop-all-feeds")
    page.locator(".sidebar button.label", has_text="Technology").click()
    expect(page.locator(".view-title")).to_contain_text("Technology")
    ctx.close()

    ctx, page = open_app(b, env, DESKTOP, nav_mode="expanded")
    assert page.locator(".sidebar .is-feed").count() > 0, "expanded mode lists feeds"
    page.locator(".sidebar .is-feed", has_text="Slow Trains").click()
    expect(page.locator(".view-title")).to_contain_text("Slow Trains")
    shot(page, "desktop-expanded-mode")
    ctx.close()

    ctx, page = open_app(b, env, DESKTOP, nav_mode="all")
    assert page.locator(".sidebar .nav-group").count() == 0, "all-feeds mode hides folders"
    ctx.close()
    ctx, page = open_app(b, env, IPHONE, nav_mode="all")
    assert page.locator(".tabbar button", has_text="Folders").count() == 0
    assert page.locator(".chips").count() == 0
    ctx.close()


MACBOOKS = {
    "macbook-air-13": dict(viewport={"width": 1280, "height": 800}, device_scale_factor=2),
    "macbook-14": dict(viewport={"width": 1512, "height": 982}, device_scale_factor=2),
    "macbook-16": dict(viewport={"width": 1728, "height": 1117}, device_scale_factor=2),
}


@test
def desktop_split_reading_pane(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, DESKTOP)
    # empty pane beside the timeline before anything is opened
    expect(page.locator(".reader.is-pane .pane-empty")).to_be_visible()
    shot(page, "desktop-split-empty")
    timeline_box = page.locator(".card").first.bounding_box()
    pane_box = page.locator(".reader.is-pane").bounding_box()
    assert timeline_box["x"] + timeline_box["width"] <= pane_box["x"] + 1, (timeline_box, pane_box)

    cards = page.locator(".card")
    target = cards.nth(2)
    item_id = target.get_attribute("data-id")
    target.locator(".card-title").click()
    expect(page.locator(".reader .article h1")).to_be_visible()
    # the timeline stays visible and the opened card is highlighted
    assert page.locator(".card").first.is_visible()
    expect(page.locator(f'.card[data-id="{item_id}"]')).to_have_class(re.compile("is-selected"))
    assert page.locator(".article").bounding_box()["width"] <= 760
    assert env.item_status(item_id) != "unread"  # read (or still saved from an earlier test)
    # no history entry per article in split mode
    assert page.evaluate("history.state && history.state.yarrowReader") in (None, False)
    shot(page, "desktop-split-reader")

    # j/k move through articles directly in the pane
    title = page.locator(".article h1").inner_text()
    page.keyboard.press("j")
    expect(page.locator(".article h1")).not_to_have_text(title)
    expect(page.locator(".card.is-selected")).to_have_count(1)
    assert page.locator(".card.is-selected").get_attribute("data-id") != item_id

    # space pages through the article, not the timeline
    page.wait_for_timeout(700)  # let the list finish scrolling to the selected card
    y = page.evaluate("window.scrollY")
    page.locator(".reader").evaluate("e => e.scrollTo(0, 0)")
    page.keyboard.press(" ")
    page.wait_for_timeout(600)
    assert page.locator(".reader").evaluate("e => e.scrollTop") > 100
    assert page.evaluate("window.scrollY") == y

    page.keyboard.press("Escape")
    expect(page.locator(".reader .pane-empty")).to_be_visible()
    expect(page.locator(".card.is-selected")).to_have_count(0)
    page.keyboard.press("?")
    expect(page.locator(".sheet .kbd-list")).to_be_visible()
    ctx.close()


@test
def desktop_hover_and_context_menu(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, DESKTOP)
    card = page.locator(".card").nth(4)
    item_id = card.get_attribute("data-id")
    card.hover()
    quick = card.locator(".card-quick")
    expect(quick.first).to_be_visible()
    shot(page, "desktop-hover-actions")
    card.locator(".card-quick[aria-label='Save for later']").click()
    page.wait_for_timeout(300)
    assert env.item_status(item_id) == "starred"
    # right-click opens the article menu at the pointer
    box = page.locator(".card").nth(5).bounding_box()
    page.mouse.click(box["x"] + 60, box["y"] + box["height"] - 40, button="right")
    menu = page.locator(".menu")
    expect(menu).to_be_visible()
    mbox = menu.bounding_box()
    assert abs(mbox["x"] - (box["x"] + 60)) < 30, (mbox, box)
    assert any("Copy link" in t for t in menu.locator(".menu-item").all_inner_texts())
    shot(page, "desktop-context-menu")
    page.keyboard.press("Escape")
    expect(menu).to_have_count(0)
    # touch devices don't get hover actions
    ctx.close()
    ctx, page = open_app(b, env, IPHONE)
    assert not page.locator(".card .card-quick").first.is_visible()
    ctx.close()


@test
def desktop_full_page_reader_option(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, DESKTOP)
    page.locator(".sidebar .nav-item", has_text="Settings").click()
    page.locator(".segmented button", has_text="Full page").click()
    page.locator(".sidebar .nav-item", has_text="All Feeds").click()
    expect(page.locator(".reader")).to_have_count(0)
    page.keyboard.press("j")
    page.keyboard.press("o")
    expect(page.locator(".reader .article h1")).to_be_visible()
    w = page.locator(".article").bounding_box()["width"]
    assert w <= 690, w
    shot(page, "desktop-reader")
    page.keyboard.press("Escape")
    expect(page.locator(".reader")).to_have_count(0)
    # the choice is remembered per browser
    page.reload()
    page.wait_for_selector(".card")
    expect(page.locator(".reader")).to_have_count(0)
    page.evaluate("localStorage.setItem('yarrow.readingPane', 'split')")
    ctx.close()


@test
def macbook_window_sizes(b, env):
    reset_view(env)
    for name, device in MACBOOKS.items():
        ctx, page = open_app(b, env, device)
        no_horizontal_overflow(page)
        assert page.locator(".sidebar").is_visible()
        card = page.locator(".card").first.bounding_box()
        pane = page.locator(".reader.is-pane").bounding_box()
        assert card["width"] >= 360, (name, card)
        assert pane["width"] >= 500, (name, pane)
        page.locator(".card").nth(1).locator(".card-title").click()
        expect(page.locator(".reader .article h1")).to_be_visible()
        shot(page, f"{name}-split")
        # the sidebar collapses with backslash and gives the list more room
        page.keyboard.press("\\")
        page.wait_for_timeout(300)
        assert page.locator(".card").first.bounding_box()["width"] >= card["width"]
        page.keyboard.press("\\")
        ctx.close()
    # dark theme on the most common size
    ctx, page = open_app(b, env, MACBOOKS["macbook-14"], theme="night")
    page.locator(".card").nth(3).locator(".card-title").click()
    expect(page.locator(".reader .article h1")).to_be_visible()
    shot(page, "macbook-14-split-dark")
    ctx.close()
    env.api("api/settings", "PUT", {"theme_name": "light"})


@test
def settings_manage_folders_and_feeds(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    page.locator(".tabbar button", has_text="Settings").click()
    expect(page.locator(".settings")).to_be_visible()
    shot(page, "phone-settings")
    # create folder (window.prompt)
    page.once("dialog", lambda d: d.accept("Reading List"))
    page.locator(".list-row", has_text="New folder").click()
    expect(page.locator(".list-row.is-folder", has_text="Reading List")).to_be_visible()
    # move a feed into it via the feed's menu
    page.locator(".list-row", has_text="The Margins").locator(".icon-btn").click()
    page.locator(".menu .menu-item", has_text="Reading List").click()
    page.wait_for_timeout(300)
    fid = feed_id(env, "The Margins")
    assert next(f for f in env.api("api/feeds") if f["id"] == fid)["folder_id"] == folder_id(env, "Reading List")
    # rename the folder
    page.once("dialog", lambda d: d.accept("Long Reads"))
    page.locator(".list-row.is-folder", has_text="Reading List").locator(".icon-btn").click()
    page.locator(".menu .menu-item", has_text="Rename").click()
    expect(page.locator(".list-row.is-folder", has_text="Long Reads")).to_be_visible()
    # rename a feed
    page.once("dialog", lambda d: d.accept("Margins"))
    page.locator(".list-row", has_text="The Margins").locator(".icon-btn").click()
    page.locator(".menu .menu-item", has_text="Rename").click()
    page.wait_for_timeout(300)
    assert any(f["title"] == "Margins" for f in env.api("api/feeds"))
    # delete the folder: feed survives without a folder
    page.once("dialog", lambda d: d.accept())
    page.locator(".list-row.is-folder", has_text="Long Reads").locator(".icon-btn").click()
    page.locator(".menu .menu-item", has_text="Delete folder").click()
    page.wait_for_timeout(400)
    assert not any(f["title"] == "Long Reads" for f in env.api("api/folders"))
    assert next(f for f in env.api("api/feeds") if f["id"] == fid)["folder_id"] is None
    # navigation and refresh settings persist
    page.locator(".segmented button", has_text="Expanded").click()
    page.locator("select[aria-label='Refresh interval']").select_option("30")
    page.wait_for_timeout(300)
    s = env.api("api/settings")
    assert s["nav_mode"] == "expanded" and s["refresh_rate"] == 30, s
    env.api("api/settings", "PUT", {"nav_mode": "folders", "refresh_rate": 60})
    ctx.close()


@test
def add_and_remove_feed(b, env):
    reset_view(env)
    extra = os.path.join(env.feeds_dir, "feeds", "extra.xml")
    with open(extra, "w") as f:
        f.write("""<?xml version="1.0"?><rss version="2.0"><channel><title>Extra Feed</title><link>http://127.0.0.1/</link>
<item><title>Hello from the extra feed</title><link>http://127.0.0.1/x</link><guid>x1</guid><description>Body text</description></item>
</channel></rss>""")
    ctx, page = open_app(b, env, IPHONE)
    page.locator(".topbar .icon-btn[aria-label='More actions']").click()
    page.locator(".menu .menu-item", has_text="Add feed").click()
    page.locator("#feed-url").fill(f"http://127.0.0.1:{env.feed_port}/feeds/extra.xml")
    page.locator("#feed-folder").select_option(label="Technology")
    shot(page, "phone-add-feed")
    page.locator(".add-feed button[type=submit]").click()
    expect(page.locator(".view-title")).to_contain_text("Extra Feed")
    expect(page.locator(".card", has_text="Hello from the extra feed")).to_be_visible()
    fid = feed_id(env, "Extra Feed")
    assert next(f for f in env.api("api/feeds") if f["id"] == fid)["folder_id"] == folder_id(env, "Technology")
    # private-network addresses are refused with a clear message
    page.locator(".topbar .icon-btn[aria-label='More actions']").click()
    page.locator(".menu .menu-item", has_text="Add feed").click()
    page.locator("#feed-url").fill("http://169.254.169.254/latest/meta-data/")
    page.locator(".add-feed button[type=submit]").click()
    expect(page.locator(".form-error")).to_contain_text("private network")
    page.keyboard.press("Escape")
    # unsubscribe through the feed menu
    page.once("dialog", lambda d: d.accept())
    page.locator(".topbar .icon-btn[aria-label='More actions']").click()
    page.locator(".menu .menu-item", has_text="Unsubscribe").click()
    page.wait_for_timeout(500)
    assert not any(f["title"] == "Extra Feed" for f in env.api("api/feeds"))
    ctx.close()


@test
def opml_export_contains_folders(b, env):
    with urllib.request.urlopen(env.base + "opml/export") as r:
        body = r.read().decode()
    assert "Technology" in body and "Circuit Notes" in body, body[:400]


@test
def search(b, env):
    reset_view(env)
    ctx, page = open_app(b, env, IPHONE)
    page.locator(".topbar .icon-btn[aria-label='Search']").click()
    page.locator("#searchbar").fill("ferries")
    page.wait_for_timeout(900)
    titles = page.locator(".card .card-title").all_inner_texts()
    assert titles and all("ferries" in t.lower() for t in titles), titles
    ctx.close()


@test
def slow_network_and_errors(b, env):
    reset_view(env)
    # page.route cannot see requests answered by a service worker
    ctx = b.new_context(**IPHONE, service_workers="block")
    page = ctx.new_page()

    # throttle like a poor mobile connection (Chromium DevTools protocol)
    cdp = ctx.new_cdp_session(page)
    cdp.send("Network.enable")
    cdp.send("Network.emulateNetworkConditions",
             {"offline": False, "latency": 1500, "downloadThroughput": 50_000, "uploadThroughput": 20_000})
    page.goto(env.base)
    expect(page.locator(".skeleton").first).to_be_visible(timeout=15000)
    shot(page, "phone-loading")
    page.wait_for_selector(".card", timeout=30000)
    cdp.send("Network.emulateNetworkConditions",
             {"offline": False, "latency": 0, "downloadThroughput": -1, "uploadThroughput": -1})
    # API failure shows an actionable error with retry
    page.route("**/api/items?*", lambda r: r.fulfill(status=500, body="boom"))
    page.locator(".chips .chip", has_text="Finance").click()
    expect(page.locator(".timeline-state h2")).to_have_text("Couldn't load articles")
    page.unroute("**/api/items?*")
    page.locator(".timeline-state .btn", has_text="Try again").click()
    page.wait_for_selector(".card")
    ctx.close()


@test
def pwa_manifest_and_icons(b, env):
    m = env.api("manifest.json")
    assert m["display"] == "standalone" and m["start_url"] == "/"
    for icon in m["icons"]:
        with urllib.request.urlopen(env.base.rstrip("/") + icon["src"]) as r:
            assert r.status == 200 and r.headers["Content-Type"] == "image/png"
    with urllib.request.urlopen(env.base + "sw.js") as r:
        assert r.status == 200
    ctx = b.new_context(**DESKTOP)
    page = ctx.new_page()
    page.goto(env.base)  # http://127.0.0.1 counts as a secure context
    page.wait_for_selector(".card")
    ok = page.evaluate("navigator.serviceWorker.ready.then(r => !!r.active)")
    assert ok, "service worker did not activate"
    ctx.close()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--binary", default=os.path.join(ROOT, "out", "yarrow"))
    ap.add_argument("--browser", default="chromium", choices=["chromium", "webkit", "firefox"])
    ap.add_argument("-k", default="", help="only run tests whose name contains this")
    args = ap.parse_args()

    env = Env(os.path.abspath(args.binary))
    failures = 0
    try:
        with sync_playwright() as p:
            browser = getattr(p, args.browser).launch()
            for fn in TESTS:
                if args.k and args.k not in fn.__name__:
                    continue
                t0 = time.time()
                try:
                    fn(browser, env)
                    print(f"PASS  {fn.__name__} ({time.time() - t0:.1f}s)")
                except Exception:
                    failures += 1
                    print(f"FAIL  {fn.__name__}")
                    traceback.print_exc()
            browser.close()
    finally:
        env.close()
    print(f"\n{len(TESTS) - failures if not args.k else '?'} passed, {failures} failed")
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
