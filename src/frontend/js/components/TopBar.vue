<template>
  <header ref="bar" class="topbar" :class="{ 'is-scrolled': scrolled, 'is-compact': compact && !searchOpen }">
    <div class="column">
      <div class="topbar-row">
        <button
          class="icon-btn desktop-only"
          type="button"
          :aria-label="state.sidebarCollapsed ? 'Show sidebar' : 'Hide sidebar'"
          :title="(state.sidebarCollapsed ? 'Show sidebar' : 'Hide sidebar') + ' (\\)'"
          @click="toggleSidebar">
          <v-icon name="sidebar" />
        </button>
        <button
          v-if="canPickFolder"
          class="view-title"
          type="button"
          aria-haspopup="dialog"
          @click="state.sheet = 'folders'">
          <span>{{ title }}</span>
          <v-icon class="mobile-only" name="chevron-down" />
        </button>
        <h1 v-else class="view-title"><span>{{ title }}</span></h1>
        <button
          v-if="state.view.type !== 'saved'"
          class="icon-btn desktop-only"
          type="button"
          aria-label="Mark all as read"
          title="Mark all as read"
          @click="confirmMarkAll">
          <v-icon name="check" />
        </button>
        <button class="icon-btn" type="button" aria-label="Search" title="Search (/)" :aria-pressed="searchOpen" @click="toggleSearch">
          <v-icon name="search" />
        </button>
        <button
          v-if="state.refreshAvailable"
          class="icon-btn"
          type="button"
          aria-label="Refresh feeds"
          title="Refresh feeds (r)"
          :disabled="state.refreshing > 0"
          @click="refreshFeeds()">
          <span v-if="state.refreshing > 0" class="spinner"></span>
          <v-icon v-else name="rotate-cw" />
        </button>
        <button class="icon-btn" type="button" aria-label="More actions" @click="onViewMenu">
          <v-icon name="more-horizontal" />
        </button>
      </div>

      <div v-if="searchOpen" class="search-row">
        <input
          ref="searchInput"
          id="searchbar"
          class="input"
          type="search"
          enterkeyhint="search"
          :placeholder="'Search ' + title"
          v-model="query"
          @keydown.enter="($event.target as HTMLInputElement).blur()"
          @keydown.esc="toggleSearch" />
      </div>

      <nav v-if="showChips" class="chips is-mobile-only" aria-label="Folders">
        <button class="chip" type="button" :aria-pressed="state.view.type === 'all'" @click="selectView({ type: 'all', id: null })">
          All
        </button>
        <button
          v-for="f in sortedFolders"
          :key="f.id"
          class="chip"
          type="button"
          :aria-pressed="state.view.type === 'folder' && state.view.id === f.id"
          @click="selectView({ type: 'folder', id: f.id })">
          {{ f.title }}
          <span v-if="unread(f.id)" class="count">{{ unread(f.id) }}</span>
        </button>
      </nav>

      <div class="topbar-sub" v-if="state.view.type !== 'saved'">
        <div class="segmented" role="group" aria-label="Show">
          <button type="button" :aria-pressed="!state.unreadOnly" @click="setUnreadOnly(false)">All</button>
          <button type="button" :aria-pressed="state.unreadOnly" @click="setUnreadOnly(true)">Unread</button>
        </div>
        <span class="unread-count" v-if="viewUnread">{{ viewUnread }} unread</span>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import {
  state,
  navMode,
  sortedFolders,
  feedsById,
  foldersById,
  viewTitle,
  unreadFor,
  selectView,
  setUnreadOnly,
  setSearch,
  refreshFeeds,
  markAllRead,
  openMenu,
  writeLocal,
  type MenuEntry,
} from "../store";
import { feedMenu, folderMenu } from "../manage";
import { debounce } from "../utils";
import VIcon from "./Icon.vue";

const scrolled = ref(false);
// on phones the chips/filter rows tuck away while scrolling down and come
// back as soon as the reader scrolls up, like a native large-title bar
const compact = ref(false);
let lastY = 0;
const searchOpen = ref(!!state.search);
const query = ref(state.search);
const searchInput = ref<HTMLInputElement>();

const title = computed(() => viewTitle());
const canPickFolder = computed(() => navMode.value !== "all");
const showChips = computed(
  () => navMode.value === "folders" && state.view.type !== "saved" && state.view.type !== "feed" && state.folders.length > 0,
);
const viewUnread = computed(() => (state.view.type === "saved" ? 0 : unreadFor(state.view)));
const unread = (id: number) => unreadFor({ type: "folder", id });

const runSearch = debounce((q: string) => setSearch(q), 350);
watch(query, q => runSearch(q));
watch(
  () => state.view,
  () => {
    query.value = "";
    searchOpen.value = false;
  },
);

function toggleSearch() {
  searchOpen.value = !searchOpen.value;
  if (searchOpen.value) nextTick(() => searchInput.value?.focus());
  else if (query.value) query.value = "";
}
defineExpose({ toggleSearch });

function confirmMarkAll() {
  const n = viewUnread.value;
  if (!n) return;
  if (n < 20 || window.confirm(`Mark ${n} articles in ${title.value} as read?`)) markAllRead();
}

function toggleSidebar() {
  state.sidebarCollapsed = !state.sidebarCollapsed;
  writeLocal("yarrow.sidebarCollapsed", state.sidebarCollapsed ? "1" : "0");
}

function onViewMenu(e: Event) {
  const entries: MenuEntry[] = [];
  if (state.view.type !== "saved") {
    entries.push({ label: "Mark all as read", icon: "check", action: () => markAllRead() });
  }
  entries.push({ label: "Add feed", icon: "plus", action: () => (state.sheet = "add-feed") });
  if (state.view.type === "folder") {
    const folder = foldersById.value[state.view.id!];
    if (folder) entries.push({ divider: true }, ...folderMenu(folder));
  }
  if (state.view.type === "feed") {
    const feed = feedsById.value[state.view.id!];
    if (feed) entries.push({ divider: true }, ...feedMenu(feed));
  }
  openMenu(e, entries, title.value);
}

const bar = ref<HTMLElement>();
let ro: ResizeObserver | undefined;
// the bar is fixed on phones; reserve its full (expanded) height in the page
// so collapsing it never shifts the timeline
function measure() {
  if (!bar.value || compact.value) return;
  document.documentElement.style.setProperty("--topbar-h", bar.value.offsetHeight + "px");
}

const onScroll = () => {
  const y = window.scrollY;
  scrolled.value = y > 4;
  if (y < 80) compact.value = false;
  else if (y > lastY + 6) compact.value = true;
  else if (y < lastY - 6) compact.value = false;
  lastY = y;
};
onMounted(() => {
  window.addEventListener("scroll", onScroll, { passive: true });
  ro = new ResizeObserver(measure);
  if (bar.value) ro.observe(bar.value);
  measure();
});
onUnmounted(() => {
  window.removeEventListener("scroll", onScroll);
  ro?.disconnect();
});
</script>
