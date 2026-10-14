<template>
  <div>
    <div class="ptr" :style="{ height: pull + 'px' }" aria-hidden="true">
      <span v-if="pullRefreshing" class="spinner"></span>
      <v-icon v-else name="arrow-up" :style="{ transform: `rotate(${pull >= PULL_TRIGGER ? 0 : 180}deg)` }" />
    </div>

    <div v-if="state.newArrivals" class="pill-notice">
      <button type="button" @click="showNew"><v-icon small name="arrow-up" /> New articles</button>
    </div>

    <div class="column">
      <div v-if="state.offline" class="offline-banner" role="status">
        <v-icon small name="wifi-off" /> You're offline. Showing articles saved on this device.
      </div>

      <div class="timeline" role="feed" :aria-busy="state.loading">
        <v-card
          v-for="(item, i) in state.items"
          :key="item.id"
          :item="item"
          :focused="i === state.focusedIndex"
          :aria-setsize="-1"
          :aria-posinset="i + 1" />

        <template v-if="state.loading && !state.items.length">
          <div class="skeleton" v-for="n in 4" :key="'s' + n"><i></i><i class="wide"></i><i></i><i></i></div>
        </template>
      </div>

      <div v-if="state.loadError && !state.items.length" class="timeline-state" role="alert">
        <h2>Couldn't load articles</h2>
        <p>{{ state.loadError }}</p>
        <button class="btn" type="button" @click="loadItems(true)">Try again</button>
      </div>
      <div v-else-if="!state.loading && !state.items.length" class="timeline-state">
        <template v-if="!state.feeds.length">
          <h2>No subscriptions yet</h2>
          <p>Add a website or feed URL, or import an OPML file from your old reader.</p>
          <button class="btn" type="button" @click="state.sheet = 'add-feed'"><v-icon name="plus" /> Add feed</button>
        </template>
        <template v-else-if="state.search">
          <h2>No matches</h2>
          <p>Nothing in {{ viewTitle() }} matches "{{ state.search }}".</p>
        </template>
        <template v-else-if="state.view.type === 'saved'">
          <h2>Nothing saved</h2>
          <p>Use "Save for later" in an article's menu to keep it here.</p>
        </template>
        <template v-else-if="state.view.type === 'folder' && !folderHasFeeds">
          <h2>This folder is empty</h2>
          <p>Move feeds into it from Settings, using a feed's menu.</p>
          <button class="btn is-quiet" type="button" @click="state.page = 'settings'">Open settings</button>
        </template>
        <template v-else-if="state.unreadOnly">
          <h2>All caught up</h2>
          <p>There are no unread articles here.</p>
          <button class="btn is-quiet" type="button" @click="setUnreadOnly(false)">Show all articles</button>
        </template>
        <template v-else>
          <h2>No articles yet</h2>
          <p>New articles appear after the next refresh.</p>
        </template>
      </div>

      <div v-if="state.items.length" ref="sentinel" class="load-more">
        <span v-if="state.hasMore || state.loading" class="spinner" aria-label="Loading more"></span>
        <button v-else-if="state.loadError" class="btn is-quiet" type="button" @click="loadItems()">
          Couldn't load more. Try again
        </button>
        <div v-else class="timeline-end">You've reached the end.</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, nextTick } from "vue";
import { state, loadItems, refreshFeeds, setUnreadOnly, viewTitle } from "../store";
import VCard from "./ArticleCard.vue";
import VIcon from "./Icon.vue";

const PULL_TRIGGER = 72;
const sentinel = ref<HTMLElement>();
const pull = ref(0);
const pullRefreshing = ref(false);

const folderHasFeeds = computed(() => state.feeds.some(f => f.folder_id === state.view.id));

function showNew() {
  window.scrollTo({ top: 0, behavior: "smooth" });
  loadItems(true);
}

// infinite scroll: start loading well before the end is visible
let observer: IntersectionObserver | undefined;
function observe() {
  observer?.disconnect();
  observer = new IntersectionObserver(
    entries => {
      if (entries.some(e => e.isIntersecting) && !state.loadError) loadItems();
    },
    { rootMargin: "1200px 0px" },
  );
  if (sentinel.value) observer.observe(sentinel.value);
}

// pull-to-refresh for touch devices, only when the page is at the very top
let startY: number | null = null;
function onTouchStart(e: TouchEvent) {
  if (window.scrollY <= 0 && !state.reader.open && !state.menu && !state.sheet && e.touches.length === 1) {
    startY = e.touches[0].clientY;
  } else startY = null;
}
function onTouchMove(e: TouchEvent) {
  if (startY === null || pullRefreshing.value) return;
  const dy = e.touches[0].clientY - startY;
  pull.value = dy > 0 ? Math.min(110, dy * 0.5) : 0;
}
async function onTouchEnd() {
  if (startY === null) return;
  startY = null;
  if (pull.value >= PULL_TRIGGER && state.refreshAvailable) {
    pullRefreshing.value = true;
    pull.value = 56;
    await refreshFeeds();
    pullRefreshing.value = false;
  }
  pull.value = 0;
}

// (re)attach when the sentinel element appears or is replaced
watch(sentinel, () => observe());

// an observer only fires on changes; if the sentinel is still near the
// viewport after a page arrives (tall screens), keep loading
watch(
  () => state.loading,
  loading => {
    if (loading || !state.hasMore || state.loadError) return;
    nextTick(() => {
      const r = sentinel.value?.getBoundingClientRect();
      if (r && r.top < window.innerHeight + 1200) loadItems();
    });
  },
);

onMounted(() => {
  observe();
  window.addEventListener("touchstart", onTouchStart, { passive: true });
  window.addEventListener("touchmove", onTouchMove, { passive: true });
  window.addEventListener("touchend", onTouchEnd);
});
onUnmounted(() => {
  observer?.disconnect();
  window.removeEventListener("touchstart", onTouchStart);
  window.removeEventListener("touchmove", onTouchMove);
  window.removeEventListener("touchend", onTouchEnd);
});
</script>
