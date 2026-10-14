<template>
  <form class="add-feed" @submit.prevent="submit">
    <div class="field">
      <label for="feed-url">Website or feed URL</label>
      <input
        id="feed-url"
        class="input"
        type="url"
        inputmode="url"
        autocomplete="off"
        autocapitalize="off"
        required
        placeholder="https://example.com"
        v-model="url"
        :readonly="choices.length > 0"
        autofocus />
    </div>
    <div class="field">
      <label for="feed-folder">Folder</label>
      <select id="feed-folder" class="input" v-model="folderId">
        <option :value="null">No folder</option>
        <option v-for="f in sortedFolders" :key="f.id" :value="f.id">{{ f.title }}</option>
      </select>
      <button class="link-btn" type="button" @click="newFolder">New folder…</button>
    </div>

    <div v-if="choices.length" class="field">
      <label>Several feeds were found. Pick one:</label>
      <div class="list-group">
        <label v-for="c in choices" :key="c.url" class="list-row">
          <input type="radio" name="choice" :value="c.url" v-model="choice" />
          <span class="label">
            <div>{{ c.title || c.url }}</div>
            <div class="sub" v-if="c.title">{{ c.url }}</div>
          </span>
        </label>
      </div>
      <button class="link-btn" type="button" @click="reset">Use a different URL</button>
    </div>

    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <button class="btn" type="submit" :disabled="busy">
      <span v-if="busy" class="spinner"></span>
      <span v-else>Subscribe</span>
    </button>
  </form>
</template>

<script setup lang="ts">
import { ref } from "vue";
import api from "../api";
import type { FeedLink } from "../api-types";
import { state, sortedFolders, toast, describeError, loadSubscriptions, loadStatus, loadItems, selectView } from "../store";
import { createFolder } from "../manage";

const emit = defineEmits<{ done: [] }>();
const url = ref("");
const folderId = ref<number | null>(state.view.type === "folder" ? state.view.id : null);
const choices = ref<FeedLink[]>([]);
const choice = ref("");
const busy = ref(false);
const error = ref("");

function reset() {
  choices.value = [];
  choice.value = "";
  error.value = "";
}

async function newFolder() {
  const f = await createFolder();
  if (f) folderId.value = f.id;
}

async function submit() {
  error.value = "";
  let target = choice.value || url.value.trim();
  if (!/^https?:\/\//i.test(target)) target = "https://" + target;
  const picked = choices.value.find(c => c.url === choice.value);
  busy.value = true;
  try {
    const res = await api.feeds.create({
      url: target,
      folder_id: folderId.value,
      title_override: picked?.title_override,
    } as any);
    if (res.status === "success") {
      toast(`Subscribed to ${res.feed.title}`);
      await loadSubscriptions();
      await loadStatus();
      emit("done");
      selectView({ type: "feed", id: res.feed.id });
      loadItems(true);
    } else if (res.status === "multiple") {
      choices.value = res.choice;
      choice.value = res.choice[0]?.url ?? "";
    } else {
      const reason = (res as { error?: string }).error;
      error.value = reason?.includes("blocked: destination")
        ? "That address is on a private network, which is blocked for safety. See YARR_ALLOWED_NETWORKS in the docs."
        : "No feed was found at that address. Check the URL, or paste the feed's own URL.";
    }
  } catch (err) {
    error.value = "Couldn't add the feed. " + describeError(err);
  } finally {
    busy.value = false;
  }
}
</script>
