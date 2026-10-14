<template>
  <!-- Navigation shared by the desktop sidebar and the mobile folder sheet.
       folder-only mode lists folders; expanded mode adds their feeds. -->
  <div>
    <button class="nav-item" type="button" :aria-current="is('all')" @click="go({ type: 'all', id: null })">
      <v-icon name="inbox" /><span class="label">All Feeds</span>
      <span class="count" v-if="count({ type: 'all', id: null })">{{ count({ type: "all", id: null }) }}</span>
    </button>
    <button class="nav-item" type="button" :aria-current="is('saved')" @click="go({ type: 'saved', id: null })">
      <v-icon name="bookmark" /><span class="label">Saved</span>
      <span class="count" v-if="starredTotal">{{ starredTotal }}</span>
    </button>

    <template v-if="navMode !== 'all'">
      <div class="nav-group" v-if="sortedFolders.length">Folders</div>
      <template v-for="folder in sortedFolders" :key="folder.id">
        <div class="nav-item" :aria-current="is('folder', folder.id)" role="presentation">
          <button
            v-if="navMode === 'expanded'"
            class="nav-toggle"
            type="button"
            :aria-expanded="folder.is_expanded"
            :aria-label="(folder.is_expanded ? 'Collapse ' : 'Expand ') + folder.title"
            @click="setFolderExpanded(folder, !folder.is_expanded)">
            <v-icon name="chevron-right" />
          </button>
          <v-icon v-else name="folder" />
          <button class="label" type="button" @click="go({ type: 'folder', id: folder.id })">{{ folder.title }}</button>
          <span class="count" v-if="count({ type: 'folder', id: folder.id })">
            {{ count({ type: "folder", id: folder.id }) }}
          </span>
        </div>
        <template v-if="navMode === 'expanded' && folder.is_expanded">
          <feed-row v-for="feed in feedsInFolder(folder.id)" :key="feed.id" :feed="feed" />
        </template>
      </template>
      <template v-if="navMode === 'expanded' && unfiled.length">
        <div class="nav-group">Not in a folder</div>
        <feed-row v-for="feed in unfiled" :key="feed.id" :feed="feed" />
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h } from "vue";
import type { Feed } from "../api-types";
import {
  state,
  navMode,
  sortedFolders,
  starredTotal,
  feedsInFolder,
  unreadFor,
  selectView,
  type View,
} from "../store";
import { setFolderExpanded } from "../manage";
import VIcon from "./Icon.vue";
import VFavicon from "./Favicon.vue";

const emit = defineEmits<{ navigate: [] }>();
const unfiled = computed(() => feedsInFolder(null));
const count = (v: View) => unreadFor(v);
const is = (type: string, id: number | null = null) => state.view.type === type && state.view.id === id;

function go(v: View) {
  selectView(v);
  emit("navigate");
}

const FeedRow = defineComponent({
  props: { feed: { type: Object as () => Feed, required: true } },
  setup(props) {
    return () =>
      h(
        "button",
        {
          class: "nav-item is-feed",
          type: "button",
          "aria-current": is("feed", props.feed.id),
          onClick: () => go({ type: "feed", id: props.feed.id }),
        },
        [
          h(VFavicon, { feed: props.feed }),
          h("span", { class: "label" }, props.feed.title),
          state.feedErrors[props.feed.id] ? h("span", { class: "error", title: state.feedErrors[props.feed.id] }, "!") : null,
          unreadFor({ type: "feed", id: props.feed.id })
            ? h("span", { class: "count" }, String(unreadFor({ type: "feed", id: props.feed.id })))
            : null,
        ],
      );
  },
});
</script>
