// Feed and folder management, mirroring Yarr's menus and its simple
// prompt/confirm interactions.

import api from "./api";
import type { Feed, Folder } from "./api-types";
import {
  state,
  toast,
  describeError,
  loadSubscriptions,
  loadStatus,
  loadItems,
  selectView,
  sortedFolders,
  openExternal,
  type MenuEntry,
} from "./store";

async function run(action: () => Promise<unknown>, failure: string): Promise<boolean> {
  try {
    await action();
    return true;
  } catch (err) {
    toast(`${failure} ${describeError(err)}`, "error");
    return false;
  }
}

export async function createFolder(): Promise<Folder | null> {
  const title = window.prompt("New folder name")?.trim();
  if (!title) return null;
  let created: Folder | null = null;
  const ok = await run(async () => {
    created = await api.folders.create({ title });
  }, "Couldn't create folder.");
  if (ok) await loadSubscriptions();
  return created;
}

export async function renameFolder(folder: Folder) {
  const title = window.prompt("Rename folder", folder.title)?.trim();
  if (!title || title === folder.title) return;
  if (await run(() => api.folders.update(folder.id, { title }), "Couldn't rename folder.")) {
    await loadSubscriptions();
  }
}

export async function deleteFolder(folder: Folder) {
  const count = state.feeds.filter(f => f.folder_id === folder.id).length;
  const msg =
    count > 0
      ? `Delete folder "${folder.title}"? Its ${count} feed(s) will be kept and moved out of the folder.`
      : `Delete folder "${folder.title}"?`;
  if (!window.confirm(msg)) return;
  if (await run(() => api.folders.delete(folder.id), "Couldn't delete folder.")) {
    if (state.view.type === "folder" && state.view.id === folder.id) selectView({ type: "all", id: null });
    await loadSubscriptions();
  }
}

export async function setFolderExpanded(folder: Folder, expanded: boolean) {
  folder.is_expanded = expanded;
  run(() => api.folders.update(folder.id, { is_expanded: expanded }), "Couldn't save folder state.");
}

export async function renameFeed(feed: Feed) {
  const title = window.prompt("Rename feed", feed.title)?.trim();
  if (!title || title === feed.title) return;
  if (await run(() => api.feeds.update(feed.id, { title }), "Couldn't rename feed.")) {
    feed.title = title;
  }
}

export async function changeFeedLink(feed: Feed) {
  const link = window.prompt("Feed URL", feed.feed_link)?.trim();
  if (!link || link === feed.feed_link) return;
  if (!/^https?:\/\//i.test(link)) {
    toast("The feed URL must start with http:// or https://", "error");
    return;
  }
  if (await run(() => api.feeds.update(feed.id, { feed_link: link }), "Couldn't update link.")) {
    feed.feed_link = link;
    toast("Feed link updated. It will be used on the next refresh.");
  }
}

export async function moveFeed(feed: Feed, folderId: number | null) {
  if (await run(() => api.feeds.update(feed.id, { folder_id: folderId }), "Couldn't move feed.")) {
    feed.folder_id = folderId;
    if (state.view.type === "folder") loadItems(true);
  }
}

export async function moveFeedToNewFolder(feed: Feed) {
  const folder = await createFolder();
  if (folder) await moveFeed(feed, (folder as Folder).id);
}

export async function deleteFeed(feed: Feed) {
  if (!window.confirm(`Unsubscribe from "${feed.title}"? Its articles will be removed (saved ones too).`)) return;
  if (await run(() => api.feeds.delete(feed.id), "Couldn't delete feed.")) {
    if (state.view.type === "feed" && state.view.id === feed.id) selectView({ type: "all", id: null });
    await loadSubscriptions();
    await loadStatus();
    loadItems(true);
  }
}

/** Yarr's feed menu: website, feed link, rename, change link, move to, delete. */
export function feedMenu(feed: Feed): MenuEntry[] {
  const entries: MenuEntry[] = [];
  if (feed.link) entries.push({ label: "Website", icon: "globe", action: () => openExternal(feed.link) });
  if (feed.feed_link) entries.push({ label: "Feed link", icon: "rss", action: () => openExternal(feed.feed_link) });
  if (entries.length) entries.push({ divider: true });
  entries.push({ label: "Rename", icon: "edit", action: () => renameFeed(feed) });
  entries.push({ label: "Change link", icon: "edit", action: () => changeFeedLink(feed) });
  entries.push({ header: "Move to" });
  for (const folder of sortedFolders.value) {
    if (folder.id !== feed.folder_id) {
      entries.push({ label: folder.title, icon: "folder", action: () => moveFeed(feed, folder.id) });
    }
  }
  if (feed.folder_id) entries.push({ label: "No folder", icon: "folder-minus", action: () => moveFeed(feed, null) });
  entries.push({ label: "New folder…", icon: "folder-plus", action: () => moveFeedToNewFolder(feed) });
  entries.push({ divider: true });
  entries.push({ label: "Unsubscribe", icon: "trash", danger: true, action: () => deleteFeed(feed) });
  return entries;
}

/** Yarr's folder menu: rename, delete. */
export function folderMenu(folder: Folder): MenuEntry[] {
  return [
    { label: "Rename", icon: "edit", action: () => renameFolder(folder) },
    { divider: true },
    { label: "Delete folder", icon: "trash", danger: true, action: () => deleteFolder(folder) },
  ];
}

export async function importOPML(form: HTMLFormElement) {
  if (await run(() => api.upload_opml(form), "Import failed.")) {
    toast("Subscriptions imported. Fetching articles…");
    await loadSubscriptions();
    await loadStatus();
    loadItems(true);
  }
}
