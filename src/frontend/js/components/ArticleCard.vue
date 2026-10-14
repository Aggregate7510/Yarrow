<template>
  <article
    class="card"
    :class="{
      'is-read': item.status !== 'unread',
      'has-media': showImage,
      'is-focused': focused,
      'is-selected': selected,
    }"
    :data-id="item.id"
    :aria-current="selected || undefined"
    @click="onOpen"
    @contextmenu="onContext">
    <img
      v-if="showImage"
      class="card-media"
      :src="item.image"
      alt=""
      loading="lazy"
      decoding="async"
      referrerpolicy="no-referrer"
      @error="imageFailed = true" />
    <div class="card-body">
      <div class="card-meta">
        <span class="card-source">
          <v-favicon :feed="feed" />
          <b>{{ feed?.title || "Unknown feed" }}</b>
        </span>
        <span v-if="item.status === 'unread'" class="unread-dot" title="Unread"></span>
        <v-icon v-else-if="item.status === 'starred'" class="saved-mark" name="bookmark-full" />
        <v-reltime :val="item.date" />
        <!-- quick actions appear on hover with a mouse or trackpad -->
        <button
          v-if="item.status !== 'starred'"
          class="icon-btn card-quick"
          type="button"
          :title="item.status === 'unread' ? 'Mark as read (m)' : 'Mark as unread (m)'"
          :aria-label="item.status === 'unread' ? 'Mark as read' : 'Mark as unread'"
          @click.stop="toggleRead(item)">
          <v-icon small :name="item.status === 'unread' ? 'check-circle' : 'circle'" />
        </button>
        <button
          class="icon-btn card-quick"
          type="button"
          :title="item.status === 'starred' ? 'Remove from Saved (s)' : 'Save for later (s)'"
          :aria-label="item.status === 'starred' ? 'Remove from Saved' : 'Save for later'"
          @click.stop="toggleSaved(item)">
          <v-icon small :name="item.status === 'starred' ? 'bookmark-full' : 'bookmark'" />
        </button>
        <button
          class="icon-btn card-more"
          type="button"
          :aria-label="'Actions for ' + item.title"
          @click.stop="onMenu">
          <v-icon name="more-horizontal" />
        </button>
      </div>
      <h2 class="card-title">
        <a :href="item.link || '#'" @click.prevent>{{ item.title || "Untitled" }}</a>
      </h2>
      <p v-if="item.snippet" class="card-snippet">{{ item.snippet }}</p>
      <div v-if="item.has_audio || item.has_video" class="media-flag">
        <v-icon small :name="item.has_video ? 'play-circle' : 'headphones'" />
        {{ item.has_video ? "Video" : "Audio" }}
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { CardItem } from "../api-types";
import { state, feedsById, isSplit, openReader, openMenu, openMenuAt, articleMenu, toggleRead, toggleSaved } from "../store";
import VIcon from "./Icon.vue";
import VReltime from "./RelTime.vue";
import VFavicon from "./Favicon.vue";

const props = defineProps<{ item: CardItem; focused?: boolean }>();
const imageFailed = ref(false);
const feed = computed(() => feedsById.value[props.item.feed_id]);
const showImage = computed(() => !!props.item.image && !imageFailed.value);
const selected = computed(() => isSplit.value && state.reader.open && state.reader.id === props.item.id);

function onContext(e: MouseEvent) {
  // keep the browser's own menu for links when a modifier is held
  if (e.altKey || e.shiftKey) return;
  e.preventDefault();
  openMenuAt(e.clientX, e.clientY, articleMenu(props.item), props.item.title);
}

function onOpen(e: MouseEvent) {
  // let modified clicks on the headline open the original in a new tab
  if ((e.metaKey || e.ctrlKey) && props.item.link) {
    window.open(props.item.link, "_blank", "noopener,noreferrer");
    return;
  }
  openReader(props.item.id);
}
function onMenu(e: Event) {
  openMenu(e, articleMenu(props.item), props.item.title);
}
</script>
