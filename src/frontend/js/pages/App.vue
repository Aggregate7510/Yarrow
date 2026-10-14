<template>
  <div class="shell" :class="{ 'sidebar-collapsed': state.sidebarCollapsed }">
    <v-sidebar />

    <main class="main" :inert="state.reader.open || undefined">
      <div v-show="state.page === 'timeline'" class="timeline-page">
        <v-topbar ref="topbar" />
        <v-timeline />
      </div>
      <v-settings v-if="state.page === 'settings'" />
    </main>

    <nav class="tabbar" aria-label="Main">
      <button type="button" :aria-current="tabActive('all')" @click="tab('all')">
        <v-icon name="inbox" />All
      </button>
      <button v-if="navMode !== 'all'" type="button" :aria-current="tabActive('folders')" @click="tab('folders')">
        <v-icon name="folder" />Folders
      </button>
      <button type="button" :aria-current="tabActive('saved')" @click="tab('saved')">
        <v-icon name="bookmark" />Saved
      </button>
      <button type="button" :aria-current="tabActive('settings')" @click="tab('settings')">
        <v-icon name="settings" />Settings
      </button>
    </nav>

    <v-reader v-if="state.reader.open" />

    <v-sheet v-if="state.sheet === 'folders'" title="Folders" @close="state.sheet = null">
      <div class="list-group sheet-nav">
        <v-nav-list @navigate="state.sheet = null" />
      </div>
    </v-sheet>
    <v-sheet v-if="state.sheet === 'add-feed'" title="Add feed" @close="state.sheet = null">
      <v-add-feed @done="state.sheet = null" />
    </v-sheet>
    <v-sheet v-if="state.sheet === 'shortcuts'" title="Keyboard shortcuts" @close="state.sheet = null">
      <div class="kbd-list">
        <template v-for="[k, d] in shortcutList" :key="k"><kbd>{{ k }}</kbd><span>{{ d }}</span></template>
      </div>
    </v-sheet>

    <v-menu-host />

    <div class="toasts" aria-live="polite">
      <div v-for="t in state.toasts" :key="t.id" class="toast" :class="{ 'is-error': t.level === 'error' }">{{ t.text }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";
import { state, navMode, selectView, closeReader, loadItems, loadStatus, loadSubscriptions, loadErrors, restoreView } from "../store";
import { setupKeys, shortcutList } from "../keys";
import VIcon from "../components/Icon.vue";
import VSidebar from "../components/Sidebar.vue";
import VTopbar from "../components/TopBar.vue";
import VTimeline from "../components/Timeline.vue";
import VReader from "../components/Reader.vue";
import VSettings from "../components/Settings.vue";
import VSheet from "../components/Sheet.vue";
import VNavList from "../components/NavList.vue";
import VAddFeed from "../components/AddFeed.vue";
import VMenuHost from "../components/MenuHost.vue";

const topbar = ref<InstanceType<typeof VTopbar>>();

function tabActive(t: string) {
  if (t === "settings") return state.page === "settings";
  if (state.page === "settings") return false;
  if (t === "folders") return state.view.type === "folder" || state.view.type === "feed";
  return state.view.type === t;
}

// remember the timeline position while Settings is shown
let timelineScroll = 0;
watch(
  () => state.page,
  (page, prevPage) => {
    if (prevPage === "timeline") timelineScroll = window.scrollY;
    nextTick(() => window.scrollTo({ top: page === "timeline" ? timelineScroll : 0 }));
  },
);

function tab(t: string) {
  if (t === "settings") {
    state.page = "settings";
  } else if (t === "folders") {
    state.sheet = "folders";
  } else if (t === "saved") {
    selectView({ type: "saved", id: null });
  } else {
    if (state.page === "timeline" && state.view.type === "all") window.scrollTo({ top: 0, behavior: "smooth" });
    selectView({ type: "all", id: null });
  }
}

// the browser/iOS back gesture closes the reader instead of leaving the app
window.addEventListener("popstate", () => {
  if (state.reader.open) closeReader(true);
});

// catch up when the app returns to the foreground (e.g. PWA reopened)
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") loadStatus();
});

onMounted(async () => {
  setupKeys(() => topbar.value?.toggleSearch());
  restoreView();
  try {
    await loadSubscriptions();
  } catch {}
  loadItems(true);
  loadStatus();
  loadErrors();
});
</script>
