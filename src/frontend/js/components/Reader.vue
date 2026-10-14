<template>
  <div class="reader" ref="scroller" role="dialog" aria-modal="true" :aria-label="item?.title || 'Article'" tabindex="-1">
    <div class="reader-bar">
      <button class="reader-back" type="button" @click="closeReader()">
        <v-icon name="chevron-left" /> Back
      </button>
      <div class="grow"></div>
      <template v-if="item">
        <button
          class="icon-btn"
          type="button"
          :aria-pressed="item.status === 'starred'"
          :aria-label="item.status === 'starred' ? 'Remove from Saved' : 'Save for later'"
          :title="item.status === 'starred' ? 'Remove from Saved' : 'Save for later'"
          @click="toggleSaved(item)">
          <v-icon :name="item.status === 'starred' ? 'bookmark-full' : 'bookmark'" />
        </button>
        <button
          v-if="item.status !== 'starred'"
          class="icon-btn"
          type="button"
          :aria-label="item.status === 'unread' ? 'Mark as read' : 'Mark as unread'"
          :title="item.status === 'unread' ? 'Mark as read' : 'Mark as unread'"
          @click="toggleRead(item)">
          <v-icon :name="item.status === 'unread' ? 'circle-full' : 'circle'" />
        </button>
        <button class="icon-btn" type="button" aria-label="Text settings" title="Text settings" @click="textMenu">
          <v-icon name="type" />
        </button>
        <button class="icon-btn" type="button" aria-label="More actions" @click="moreMenu">
          <v-icon name="more-horizontal" />
        </button>
        <button class="icon-btn desktop-only" type="button" aria-label="Previous article" title="Previous (k)" :disabled="!prev" @click="openNeighbor(-1)">
          <v-icon name="chevron-up" />
        </button>
        <button class="icon-btn desktop-only" type="button" aria-label="Next article" title="Next (j)" :disabled="!next" @click="openNeighbor(1)">
          <v-icon name="chevron-down" />
        </button>
      </template>
    </div>

    <article class="article">
      <div v-if="state.reader.loading" class="timeline-state"><span class="spinner"></span></div>
      <div v-else-if="state.reader.error" class="timeline-state" role="alert">
        <h2>Couldn't open this article</h2>
        <p>{{ state.reader.error }}</p>
        <button class="btn" type="button" @click="openReader(state.reader.id!, { push: false })">Try again</button>
      </div>
      <template v-else-if="item">
        <div class="article-source">
          <v-favicon :feed="feed" />
          <b>{{ feed?.title }}</b>
          <time :datetime="item.date">{{ fullDate }}</time>
        </div>
        <h1>{{ item.title || "Untitled" }}</h1>
        <div class="article-actions">
          <a v-if="item.link" class="btn is-quiet" :href="item.link" target="_blank" rel="noopener noreferrer">
            <v-icon small name="external-link" /> Open original
          </a>
          <button v-if="item.link" class="btn is-quiet" type="button" :disabled="state.reader.fullTextLoading" @click="loadFullText">
            <span v-if="state.reader.fullTextLoading" class="spinner"></span>
            <v-icon v-else small name="book-open" />
            {{ state.reader.fullText !== null ? "Show feed version" : "Load full article" }}
          </button>
        </div>

        <template v-if="state.reader.fullText === null">
          <figure v-for="m in images" :key="m.url">
            <img class="article-lead" :src="m.url" alt="" loading="lazy" referrerpolicy="no-referrer" />
            <figcaption v-if="m.description" class="content" v-html="m.description"></figcaption>
          </figure>
          <audio v-for="m in audios" :key="m.url" class="w-full" controls preload="none" :src="m.url"></audio>
          <video v-for="m in videos" :key="m.url" class="article-lead" controls preload="none" :src="m.url"></video>
        </template>

        <!-- content is sanitized on the server (allow-list sanitizer) and the
             page runs under a script-src 'self' CSP -->
        <div class="content" :class="fontClass" :style="{ '--reader-scale': state.settings.theme_size || 1 }" v-html="body"></div>

        <nav class="reader-nav" aria-label="More articles">
          <button v-if="prev" type="button" @click="openNeighbor(-1)">Previous<span>{{ prev.title }}</span></button>
          <button v-if="next" class="next" type="button" @click="openNeighbor(1)">Next<span>{{ next.title }}</span></button>
        </nav>
      </template>
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue";
import {
  state,
  feedsById,
  closeReader,
  openReader,
  openNeighbor,
  readerNeighbor,
  toggleRead,
  toggleSaved,
  loadFullText,
  openMenu,
  articleMenu,
  saveSettings,
  type MenuEntry,
} from "../store";
import { dateTimeString } from "../utils";
import VIcon from "./Icon.vue";
import VFavicon from "./Favicon.vue";

const scroller = ref<HTMLElement>();
const item = computed(() => state.reader.item);
const feed = computed(() => (item.value ? feedsById.value[item.value.feed_id] : undefined));
const prev = computed(() => readerNeighbor(-1));
const next = computed(() => readerNeighbor(1));
const fullDate = computed(() => (item.value ? dateTimeString(new Date(item.value.date), navigator.language) : ""));
const fontClass = computed(() => ({
  "font-serif": state.settings.theme_font === "serif",
  "font-monospace": state.settings.theme_font === "monospace",
}));

const body = computed(() => state.reader.fullText ?? item.value?.content ?? "");
const media = computed(() => item.value?.media_links ?? []);
// show feed-provided images only when the article body doesn't already contain them
const images = computed(() =>
  media.value.filter(m => m.type === "image" && !(item.value?.content || "").includes(m.url)).slice(0, 1),
);
const audios = computed(() => media.value.filter(m => m.type === "audio"));
const videos = computed(() => media.value.filter(m => m.type === "video"));

watch(
  () => state.reader.id,
  () => nextTick(() => scroller.value?.scrollTo({ top: 0 })),
);
onMounted(() => scroller.value?.focus({ preventScroll: true }));

function textMenu(e: Event) {
  const size = state.settings.theme_size || 1;
  const step = (d: number) => saveSettings({ theme_size: Math.round(Math.min(1.6, Math.max(0.8, size + d)) * 10) / 10 });
  const entries: MenuEntry[] = [
    { header: "Font" },
    { label: "Sans serif" + (!state.settings.theme_font ? "  ✓" : ""), action: () => saveSettings({ theme_font: "" }) },
    { label: "Serif" + (state.settings.theme_font === "serif" ? "  ✓" : ""), action: () => saveSettings({ theme_font: "serif" }) },
    { label: "Monospace" + (state.settings.theme_font === "monospace" ? "  ✓" : ""), action: () => saveSettings({ theme_font: "monospace" }) },
    { header: `Size ${Math.round(size * 100)}%` },
    { label: "Larger text", icon: "plus", action: () => step(0.1) },
    { label: "Smaller text", icon: "chevron-down", action: () => step(-0.1) },
  ];
  openMenu(e, entries, "Text");
}

function moreMenu(e: Event) {
  if (!item.value) return;
  openMenu(e, articleMenu(item.value), item.value.title);
}
</script>
