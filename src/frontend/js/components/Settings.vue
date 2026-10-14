<template>
  <div class="column settings">
    <header class="topbar-row">
      <button class="icon-btn desktop-only" type="button" aria-label="Toggle sidebar" @click="toggleSidebar">
        <v-icon name="sidebar" />
      </button>
      <h1 class="view-title"><span>Settings</span></h1>
    </header>

    <section aria-labelledby="s-subs">
      <h2 id="s-subs">Subscriptions</h2>
      <div class="list-group">
        <button class="list-row" type="button" @click="state.sheet = 'add-feed'">
          <v-icon class="row-icon" name="plus" /><span class="label">Add feed</span>
        </button>
        <button v-if="state.refreshAvailable" class="list-row" type="button" :disabled="state.refreshing > 0" @click="refreshFeeds()">
          <v-icon class="row-icon" name="rotate-cw" />
          <span class="label">{{ state.refreshing > 0 ? `Refreshing ${state.refreshing} feeds…` : "Refresh all feeds now" }}</span>
        </button>
        <form ref="opmlForm" class="list-row" enctype="multipart/form-data" @submit.prevent>
          <v-icon class="row-icon" name="download" />
          <label class="label" for="opml-file">Import OPML…</label>
          <input id="opml-file" class="visually-hidden" type="file" name="opml" accept=".opml,.xml,text/xml,application/xml" @change="onImport" />
        </form>
        <a class="list-row" href="./opml/export" download>
          <v-icon class="row-icon" name="upload" /><span class="label">Export OPML</span>
        </a>
      </div>
    </section>

    <section aria-labelledby="s-feeds">
      <h2 id="s-feeds">Feeds and folders</h2>
      <div class="list-group">
        <div v-for="folder in sortedFolders" :key="folder.id" class="folder-block">
          <div class="list-row is-folder">
            <v-icon class="row-icon" name="folder" />
            <span class="label"><div>{{ folder.title }}</div></span>
            <span class="count">{{ feedsInFolder(folder.id).length }}</span>
            <button class="icon-btn" type="button" :aria-label="'Folder actions for ' + folder.title" @click="openMenu($event, folderMenu(folder), folder.title)">
              <v-icon name="more-horizontal" />
            </button>
          </div>
          <feed-line v-for="feed in feedsInFolder(folder.id)" :key="feed.id" :feed="feed" nested />
          <div v-if="!feedsInFolder(folder.id).length" class="list-row is-nested sub">Empty folder</div>
        </div>
        <div v-if="unfiled.length" class="folder-block">
          <div class="list-row is-folder"><span class="label"><div>Not in a folder</div></span></div>
          <feed-line v-for="feed in unfiled" :key="feed.id" :feed="feed" />
        </div>
        <button class="list-row" type="button" @click="createFolder()">
          <v-icon class="row-icon" name="folder-plus" /><span class="label">New folder…</span>
        </button>
      </div>
      <p class="hint">Use a feed's menu to rename it, change its URL, move it to another folder, or unsubscribe.</p>
    </section>

    <section aria-labelledby="s-look">
      <h2 id="s-look">Appearance</h2>
      <div class="list-group">
        <div class="setting-row">
          <span>Theme</span>
          <div class="segmented" role="group" aria-label="Theme">
            <button v-for="t in themes" :key="t.value" type="button" :aria-pressed="state.settings.theme_name === t.value" @click="setTheme(t.value)">
              {{ t.label }}
            </button>
          </div>
        </div>
        <div class="setting-row">
          <span>Article font</span>
          <div class="segmented" role="group" aria-label="Article font">
            <button v-for="f in fonts" :key="f.value" type="button" :aria-pressed="(state.settings.theme_font || '') === f.value" @click="saveSettings({ theme_font: f.value })">
              {{ f.label }}
            </button>
          </div>
        </div>
        <div class="setting-row">
          <span>Article text size</span>
          <div class="stepper">
            <button class="icon-btn" type="button" aria-label="Smaller" @click="size(-0.1)"><v-icon name="chevron-down" /></button>
            <output>{{ Math.round((state.settings.theme_size || 1) * 100) }}%</output>
            <button class="icon-btn" type="button" aria-label="Larger" @click="size(0.1)"><v-icon name="chevron-up" /></button>
          </div>
        </div>
      </div>
    </section>

    <section aria-labelledby="s-read">
      <h2 id="s-read">Reading</h2>
      <div class="list-group">
        <div class="setting-row">
          <span>Navigation</span>
          <div class="segmented" role="group" aria-label="Navigation">
            <button v-for="m in navModes" :key="m.value" type="button" :aria-pressed="navMode === m.value" @click="saveSettings({ nav_mode: m.value })">
              {{ m.label }}
            </button>
          </div>
        </div>
        <div class="setting-row">
          <span>Show first</span>
          <div class="segmented" role="group" aria-label="Sort order">
            <button type="button" :aria-pressed="state.settings.sort_newest_first !== false" @click="setSort(true)">Newest</button>
            <button type="button" :aria-pressed="state.settings.sort_newest_first === false" @click="setSort(false)">Oldest</button>
          </div>
        </div>
        <div class="setting-row" v-if="state.refreshAvailable">
          <span>Refresh feeds</span>
          <select class="input compact" :value="state.settings.refresh_rate" @change="setRate(($event.target as HTMLSelectElement).value)" aria-label="Refresh interval">
            <option v-for="r in rates" :key="r.value" :value="r.value">{{ r.label }}</option>
          </select>
        </div>
      </div>
      <p class="hint">{{ navHint }}</p>
    </section>

    <section aria-labelledby="s-more">
      <h2 id="s-more">More</h2>
      <div class="list-group">
        <button class="list-row desktop-flex" type="button" @click="state.sheet = 'shortcuts'">
          <v-icon class="row-icon" name="help-circle" /><span class="label">Keyboard shortcuts</span>
        </button>
        <button v-if="state.requiresAuth" class="list-row" type="button" @click="logout">
          <v-icon class="row-icon" name="log-out" /><span class="label">Sign out</span>
        </button>
      </div>
    </section>

    <section class="about">
      <p>Yarrow {{ state.version }} — a fork of <a href="https://github.com/nkanaev/yarr" target="_blank" rel="noopener noreferrer">Yarr</a> by Nazar Kanaev (MIT).</p>
      <p>Your subscriptions and reading history stay on your server. No analytics, no tracking.</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref } from "vue";
import api from "../api";
import type { Feed, NavMode } from "../api-types";
import {
  state,
  navMode,
  sortedFolders,
  feedsInFolder,
  saveSettings,
  refreshFeeds,
  openMenu,
  loadItems,
  writeLocal,
} from "../store";
import { createFolder, feedMenu, folderMenu, importOPML } from "../manage";
import { applyTheme } from "../theme";
import VIcon from "./Icon.vue";
import VFavicon from "./Favicon.vue";

const opmlForm = ref<HTMLFormElement>();
const unfiled = computed(() => feedsInFolder(null));

const themes = [
  { value: "light", label: "Light" },
  { value: "sepia", label: "Sepia" },
  { value: "night", label: "Dark" },
  { value: "system", label: "Auto" },
];
const fonts = [
  { value: "", label: "Sans" },
  { value: "serif", label: "Serif" },
  { value: "monospace", label: "Mono" },
];
const navModes: { value: NavMode; label: string }[] = [
  { value: "folders", label: "Folders" },
  { value: "expanded", label: "Expanded" },
  { value: "all", label: "All feeds" },
];
const rates = [
  { value: 0, label: "Manually" },
  { value: 10, label: "Every 10 minutes" },
  { value: 30, label: "Every 30 minutes" },
  { value: 60, label: "Every hour" },
  { value: 120, label: "Every 2 hours" },
  { value: 240, label: "Every 4 hours" },
  { value: 720, label: "Every 12 hours" },
  { value: 1440, label: "Once a day" },
];
const navHint = computed(
  () =>
    ({
      folders: "Folders: navigate by folder only; individual feeds stay here in Settings.",
      expanded: "Expanded: folders can be opened to show and browse each feed.",
      all: "All feeds: one combined timeline with minimal navigation.",
    })[navMode.value],
);

function setTheme(name: string) {
  saveSettings({ theme_name: name });
  applyTheme(name);
}
function size(d: number) {
  const v = Math.round(Math.min(1.6, Math.max(0.8, (state.settings.theme_size || 1) + d)) * 10) / 10;
  saveSettings({ theme_size: v });
}
function setSort(newest: boolean) {
  saveSettings({ sort_newest_first: newest });
  loadItems(true);
}
function setRate(v: string) {
  saveSettings({ refresh_rate: Number(v) });
}
function toggleSidebar() {
  state.sidebarCollapsed = !state.sidebarCollapsed;
  writeLocal("yarrow.sidebarCollapsed", state.sidebarCollapsed ? "1" : "0");
}
async function onImport() {
  if (opmlForm.value) await importOPML(opmlForm.value);
  opmlForm.value?.reset();
}
async function logout() {
  try {
    await api.logout();
  } finally {
    if ("caches" in window) {
      const keys = await caches.keys();
      await Promise.all(keys.map(k => caches.delete(k)));
    }
    window.location.reload();
  }
}

const FeedLine = defineComponent({
  props: { feed: { type: Object as () => Feed, required: true }, nested: Boolean },
  setup(props) {
    return () => {
      const err = state.feedErrors[props.feed.id];
      return h("div", { class: ["list-row", props.nested && "is-nested"] }, [
        h(VFavicon, { feed: props.feed }),
        h("span", { class: "label" }, [
          h("div", props.feed.title),
          err
            ? h("div", { class: "sub is-error" }, "Last refresh failed: " + err)
            : h("div", { class: "sub" }, props.feed.feed_link),
        ]),
        h(
          "button",
          {
            class: "icon-btn",
            type: "button",
            "aria-label": "Feed actions for " + props.feed.title,
            onClick: (e: Event) => openMenu(e, feedMenu(props.feed), props.feed.title),
          },
          [h(VIcon, { name: "more-horizontal" })],
        ),
      ]);
    };
  },
});
</script>
