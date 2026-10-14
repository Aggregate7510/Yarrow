// Application state and actions. A single reactive object keeps the code
// easy to follow; components read from `state` and call the functions below.

import { reactive, computed } from "vue";
import api, { HTTPError, NetworkError, lastResponseFromCache } from "./api";
import type {
  CardItem,
  Feed,
  Folder,
  Item,
  ItemStatus,
  NavMode,
  Settings,
  FeedStat,
} from "./api-types";

export type ViewType = "all" | "folder" | "feed" | "saved";
export interface View {
  type: ViewType;
  id: number | null;
}
export type Page = "timeline" | "settings";

export interface MenuEntry {
  label?: string;
  icon?: string;
  action?: () => void;
  href?: string;
  danger?: boolean;
  header?: string;
  divider?: boolean;
}

export interface Toast {
  id: number;
  text: string;
  level: "info" | "error";
}

const PAGE_SIZE = 25;

export const state = reactive({
  settings: (window.app?.settings ?? {}) as Settings,
  requiresAuth: !!window.app?.requiresAuth,
  version: (window.app?.version ?? "") as string,

  folders: [] as Folder[],
  feeds: [] as Feed[],
  stats: {} as Record<number, FeedStat>,
  feedErrors: {} as Record<number, string>,
  refreshAvailable: false,
  refreshing: 0,

  page: "timeline" as Page,
  view: { type: "all", id: null } as View,
  unreadOnly: false,
  search: "",

  items: [] as CardItem[],
  hasMore: false,
  loading: false,
  loadError: "" as string,
  offline: false,
  newArrivals: false,

  reader: {
    open: false,
    id: null as number | null,
    item: null as Item | null,
    loading: false,
    error: "",
    fullText: null as string | null,
    fullTextLoading: false,
  },

  menu: null as null | { title?: string; entries: MenuEntry[]; anchor: DOMRect | null },
  sheet: null as null | "folders" | "add-feed" | "shortcuts",
  toasts: [] as Toast[],
  focusedIndex: -1,
  sidebarCollapsed: readLocal("yarrow.sidebarCollapsed") === "1",
});

function readLocal(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}
export function writeLocal(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {}
}

// ---------------------------------------------------------------- derived

export const feedsById = computed(() => {
  const m: Record<number, Feed> = {};
  for (const f of state.feeds) m[f.id] = f;
  return m;
});

export const foldersById = computed(() => {
  const m: Record<number, Folder> = {};
  for (const f of state.folders) m[f.id] = f;
  return m;
});

export const sortedFolders = computed(() =>
  [...state.folders].sort((a, b) => a.title.localeCompare(b.title, undefined, { sensitivity: "base" })),
);

export function feedsInFolder(folderId: number | null): Feed[] {
  return state.feeds
    .filter(f => (f.folder_id ?? null) === folderId)
    .sort((a, b) => a.title.localeCompare(b.title, undefined, { sensitivity: "base" }));
}

export function unreadFor(view: View): number {
  let total = 0;
  for (const f of state.feeds) {
    const s = state.stats[f.id];
    if (!s) continue;
    if (view.type === "all") total += s.unread;
    else if (view.type === "folder" && f.folder_id === view.id) total += s.unread;
    else if (view.type === "feed" && f.id === view.id) total += s.unread;
  }
  return total;
}

export const starredTotal = computed(() =>
  Object.values(state.stats).reduce((n, s) => n + (s.starred || 0), 0),
);

export const navMode = computed<NavMode>(() => state.settings.nav_mode || "folders");

export function viewTitle(view: View = state.view): string {
  switch (view.type) {
    case "folder":
      return foldersById.value[view.id!]?.title ?? "Folder";
    case "feed":
      return feedsById.value[view.id!]?.title ?? "Feed";
    case "saved":
      return "Saved";
    default:
      return "All Feeds";
  }
}

export function sameView(a: View, b: View) {
  return a.type === b.type && a.id === b.id;
}

// ---------------------------------------------------------------- toasts

let toastSeq = 0;
export function toast(text: string, level: Toast["level"] = "info", ms = 3200) {
  const id = ++toastSeq;
  state.toasts.push({ id, text, level });
  window.setTimeout(() => {
    const i = state.toasts.findIndex(t => t.id === id);
    if (i >= 0) state.toasts.splice(i, 1);
  }, ms);
}

export function describeError(err: unknown): string {
  if (err instanceof NetworkError) return "Can't reach the server. Check your connection.";
  if (err instanceof HTTPError) {
    if (err.status === 401) return "Your session expired. Please sign in again.";
    return `Server error (${err.status}).`;
  }
  return "Something went wrong.";
}

function handleAuthError(err: unknown) {
  if (err instanceof HTTPError && err.status === 401 && state.requiresAuth) {
    window.location.reload();
  }
}

// ---------------------------------------------------------------- settings

export async function saveSettings(patch: Partial<Settings>) {
  Object.assign(state.settings, patch);
  try {
    await api.settings.update(patch);
  } catch (err) {
    toast("Couldn't save setting. " + describeError(err), "error");
  }
}

// persist the selected view in Yarr's own setting keys so the database
// stays meaningful to Yarr as well
function persistView() {
  let feed = "";
  if (state.view.type === "folder") feed = "folder:" + state.view.id;
  if (state.view.type === "feed") feed = "feed:" + state.view.id;
  const filter = state.view.type === "saved" ? "starred" : state.unreadOnly ? "unread" : "";
  if (state.settings.feed !== feed || state.settings.filter !== filter) {
    saveSettings({ feed, filter });
  }
}

export function restoreView() {
  const s = state.settings;
  state.unreadOnly = s.filter === "unread";
  if (s.filter === "starred") {
    state.view = { type: "saved", id: null };
    return;
  }
  const [kind, raw] = (s.feed || "").split(":");
  const id = Number(raw);
  if (kind === "folder" && id) state.view = { type: "folder", id };
  else if (kind === "feed" && id) state.view = { type: "feed", id };
  else state.view = { type: "all", id: null };
}

// ---------------------------------------------------------------- subscriptions

export async function loadSubscriptions() {
  const [folders, feeds] = await Promise.all([api.folders.list(), api.feeds.list()]);
  state.folders = folders;
  state.feeds = feeds;
  // a remembered folder/feed may have been deleted meanwhile
  if (state.view.type === "folder" && !foldersById.value[state.view.id!]) selectView({ type: "all", id: null });
  if (state.view.type === "feed" && !feedsById.value[state.view.id!]) selectView({ type: "all", id: null });
}

export async function loadErrors() {
  try {
    state.feedErrors = await api.feeds.list_errors();
  } catch {}
}

let statusTimer: number | undefined;
let lastUnreadTotal = -1;

export async function loadStatus() {
  window.clearTimeout(statusTimer);
  try {
    const status = await api.status();
    const stats: Record<number, FeedStat> = {};
    for (const s of status.stats) stats[s.feed_id] = s;
    state.stats = stats;
    state.refreshAvailable = status.refresh;
    const wasRefreshing = state.refreshing > 0;
    state.refreshing = status.running;
    state.offline = lastResponseFromCache;

    const total = unreadFor({ type: "all", id: null });
    if (lastUnreadTotal >= 0 && total > lastUnreadTotal && state.page === "timeline") {
      // new articles arrived in the background
      if (window.scrollY < 120 && !state.reader.open) loadItems(true);
      else state.newArrivals = true;
    }
    lastUnreadTotal = total;

    if (status.running > 0) {
      statusTimer = window.setTimeout(loadStatus, 1500);
    } else if (wasRefreshing) {
      loadErrors();
      loadSubscriptions().catch(() => {});
    }
  } catch (err) {
    handleAuthError(err);
    if (err instanceof NetworkError) state.offline = true;
  }
}

/** Ask the server to fetch all feeds and wait (polling) until it is done. */
export async function refreshFeeds(): Promise<void> {
  try {
    await api.feeds.refresh();
  } catch (err) {
    toast("Couldn't start refresh. " + describeError(err), "error");
    return;
  }
  state.refreshing = Math.max(state.refreshing, 1);
  await new Promise<void>(resolve => {
    const tick = async () => {
      await loadStatus();
      if (state.refreshing > 0) window.setTimeout(tick, 1500);
      else resolve();
    };
    window.setTimeout(tick, 600);
  });
  await loadItems(true);
}

// ---------------------------------------------------------------- timeline

let loadSeq = 0;

function listQuery() {
  const q: Record<string, string | number | boolean | undefined> = {
    limit: PAGE_SIZE,
    oldest_first: state.settings.sort_newest_first === false ? true : undefined,
    search: state.search || undefined,
  };
  switch (state.view.type) {
    case "folder":
      q.folder_id = state.view.id!;
      break;
    case "feed":
      q.feed_id = state.view.id!;
      break;
    case "saved":
      q.status = "starred";
      break;
  }
  if (state.view.type !== "saved" && state.unreadOnly) q.status = "unread";
  return q;
}

/** Load the first page (reset=true) or the next page of the current view. */
export async function loadItems(reset = false) {
  if (!reset && (state.loading || !state.hasMore)) return;
  const seq = ++loadSeq;
  const query = listQuery();
  if (!reset && state.items.length) query.after = state.items[state.items.length - 1].id;
  state.loading = true;
  state.loadError = "";
  if (reset) state.newArrivals = false;
  try {
    const res = await api.items.cards(query);
    if (seq !== loadSeq) return; // a newer request superseded this one
    state.offline = lastResponseFromCache;
    if (reset) {
      state.items = res.list;
      state.focusedIndex = -1;
    } else {
      // keyset pagination never repeats rows, but guard anyway
      const seen = new Set(state.items.map(i => i.id));
      for (const it of res.list) if (!seen.has(it.id)) state.items.push(it);
    }
    state.hasMore = res.has_more;
  } catch (err) {
    if (seq !== loadSeq) return;
    handleAuthError(err);
    state.loadError = describeError(err);
    if (err instanceof NetworkError) state.offline = true;
  } finally {
    if (seq === loadSeq) state.loading = false;
  }
}

export function selectView(view: View) {
  const changed = !sameView(view, state.view);
  state.view = view;
  state.page = "timeline";
  state.sheet = null;
  if (changed) {
    state.search = "";
    state.items = [];
    state.hasMore = false;
    persistView();
    loadItems(true);
    window.scrollTo({ top: 0 });
  }
}

export function setUnreadOnly(on: boolean) {
  if (state.unreadOnly === on) return;
  state.unreadOnly = on;
  persistView();
  loadItems(true);
  window.scrollTo({ top: 0 });
}

export function setSearch(text: string) {
  state.search = text.trim();
  loadItems(true);
}

// ---------------------------------------------------------------- item state

function adjustStats(feedId: number, from: ItemStatus, to: ItemStatus) {
  const s = state.stats[feedId] || (state.stats[feedId] = { feed_id: feedId, unread: 0, starred: 0 });
  if (from === "unread") s.unread = Math.max(0, s.unread - 1);
  if (from === "starred") s.starred = Math.max(0, s.starred - 1);
  if (to === "unread") s.unread += 1;
  if (to === "starred") s.starred += 1;
  lastUnreadTotal = unreadFor({ type: "all", id: null });
}

export async function setItemStatus(id: number, feedId: number, from: ItemStatus, to: ItemStatus) {
  if (from === to) return;
  const apply = (status: ItemStatus) => {
    const card = state.items.find(i => i.id === id);
    if (card) card.status = status;
    if (state.reader.item?.id === id) state.reader.item.status = status;
  };
  apply(to);
  adjustStats(feedId, from, to);
  try {
    await api.items.update(id, { status: to });
  } catch (err) {
    apply(from);
    adjustStats(feedId, to, from);
    toast("Couldn't update article. " + describeError(err), "error");
  }
}

/** Yarr semantics: an item is unread, read, or starred (saved implies read). */
export function toggleRead(item: { id: number; feed_id: number; status: ItemStatus }) {
  const to: ItemStatus = item.status === "unread" ? "read" : "unread";
  return setItemStatus(item.id, item.feed_id, item.status, to);
}

export function toggleSaved(item: { id: number; feed_id: number; status: ItemStatus }) {
  const to: ItemStatus = item.status === "starred" ? "read" : "starred";
  return setItemStatus(item.id, item.feed_id, item.status, to);
}

export async function markAllRead() {
  const q: Record<string, string | number> = {};
  if (state.view.type === "folder") q.folder_id = state.view.id!;
  if (state.view.type === "feed") q.feed_id = state.view.id!;
  try {
    await api.items.mark_read(q);
  } catch (err) {
    toast("Couldn't mark as read. " + describeError(err), "error");
    return;
  }
  for (const it of state.items) if (it.status === "unread") it.status = "read";
  await loadStatus();
  toast("Marked as read");
}

/** Mark every item shown above `item` in the current timeline as read. */
export async function markAboveRead(item: CardItem) {
  const idx = state.items.findIndex(i => i.id === item.id);
  if (idx <= 0) return;
  const q: Record<string, string | number | boolean> = { above_id: item.id };
  if (state.view.type === "folder") q.folder_id = state.view.id!;
  if (state.view.type === "feed") q.feed_id = state.view.id!;
  if (state.settings.sort_newest_first === false) q.oldest_first = true;
  try {
    await api.items.mark_read(q);
  } catch (err) {
    toast("Couldn't mark as read. " + describeError(err), "error");
    return;
  }
  let n = 0;
  for (let i = 0; i < idx; i++) {
    if (state.items[i].status === "unread") {
      state.items[i].status = "read";
      n++;
    }
  }
  await loadStatus();
  toast(n === 1 ? "1 article marked as read" : `${n} articles marked as read`);
}

// ---------------------------------------------------------------- reader

export async function openReader(id: number, opts: { push?: boolean } = {}) {
  const card = state.items.find(i => i.id === id);
  const idx = state.items.findIndex(i => i.id === id);
  if (idx >= 0) state.focusedIndex = idx;
  const wasOpen = state.reader.open;
  Object.assign(state.reader, {
    open: true,
    id,
    item: null,
    loading: true,
    error: "",
    fullText: null,
    fullTextLoading: false,
  });
  if (opts.push !== false && !wasOpen) {
    history.pushState({ yarrowReader: id }, "");
  }
  try {
    const item = await api.items.get(id);
    if (state.reader.id !== id) return;
    state.reader.item = item;
    if (item.status === "unread") {
      // opening an article marks it read (never merely scrolling past it)
      setItemStatus(item.id, item.feed_id, "unread", "read");
    } else if (card && card.status !== item.status) {
      card.status = item.status;
    }
  } catch (err) {
    if (state.reader.id !== id) return;
    state.reader.error = describeError(err);
  } finally {
    if (state.reader.id === id) state.reader.loading = false;
  }
  // prefetch the next page when reading the last loaded article
  if (idx >= state.items.length - 2) loadItems();
}

export function closeReader(fromPopState = false) {
  if (!state.reader.open) return;
  state.reader.open = false;
  if (!fromPopState && history.state?.yarrowReader) history.back();
}

export function readerNeighbor(delta: number): CardItem | null {
  const idx = state.items.findIndex(i => i.id === state.reader.id);
  if (idx < 0) return null;
  return state.items[idx + delta] ?? null;
}

export function openNeighbor(delta: number) {
  const next = readerNeighbor(delta);
  if (next) openReader(next.id, { push: false });
}

export async function loadFullText() {
  const item = state.reader.item;
  if (!item?.link) return;
  if (state.reader.fullText !== null) {
    state.reader.fullText = null; // toggle back to feed content
    return;
  }
  state.reader.fullTextLoading = true;
  try {
    const res = await api.crawl(item.link);
    if (state.reader.item?.id !== item.id) return;
    if (!res.content || res.content.startsWith("error:")) {
      toast("Couldn't extract the full article.", "error");
    } else {
      state.reader.fullText = res.content;
    }
  } catch (err) {
    toast("Couldn't load the full article. " + describeError(err), "error");
  } finally {
    state.reader.fullTextLoading = false;
  }
}

// ---------------------------------------------------------------- menus

export function openMenu(event: Event | null, entries: MenuEntry[], title?: string) {
  const el = event?.currentTarget as HTMLElement | null;
  state.menu = { entries, title, anchor: el ? el.getBoundingClientRect() : null };
}

export function closeMenu() {
  state.menu = null;
}

export async function copyText(text: string) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
    } else {
      // plain-HTTP LAN access has no async clipboard API
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.className = "visually-hidden";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      ta.remove();
    }
    toast("Link copied");
  } catch {
    toast("Couldn't copy the link", "error");
  }
}

export function openExternal(url: string) {
  if (!url) return;
  window.open(url, "_blank", "noopener,noreferrer");
}

export function articleMenu(item: CardItem | Item): MenuEntry[] {
  const entries: MenuEntry[] = [];
  if (item.status !== "starred") {
    entries.push({
      label: item.status === "unread" ? "Mark as read" : "Mark as unread",
      icon: item.status === "unread" ? "check-circle" : "circle",
      action: () => toggleRead(item),
    });
  }
  entries.push({
    label: item.status === "starred" ? "Remove from Saved" : "Save for later",
    icon: item.status === "starred" ? "bookmark-full" : "bookmark",
    action: () => toggleSaved(item),
  });
  entries.push({ divider: true });
  if (item.link) {
    entries.push({ label: "Open original", icon: "external-link", action: () => openExternal(item.link) });
    entries.push({ label: "Copy link", icon: "link", action: () => copyText(item.link) });
    if (typeof navigator.share === "function") {
      entries.push({
        label: "Share…",
        icon: "share",
        action: () => navigator.share({ title: item.title, url: item.link }).catch(() => {}),
      });
    }
  }
  const idx = state.items.findIndex(i => i.id === item.id);
  if (idx > 0 && state.view.type !== "saved" && !state.search && "snippet" in item) {
    entries.push({ divider: true });
    entries.push({
      label: "Mark above as read",
      icon: "chevrons-up",
      action: () => markAboveRead(item as CardItem),
    });
  }
  return entries;
}
