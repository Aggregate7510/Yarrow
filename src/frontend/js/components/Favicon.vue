<template>
  <img v-if="feed?.icon && !failed" class="favicon" :src="feed.icon" alt="" @error="failed = true" />
  <span v-else class="favicon is-letter" :style="{ background: color }" aria-hidden="true">{{ letter }}</span>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { Feed } from "../api-types";

const props = defineProps<{ feed?: Feed }>();
const failed = ref(false);
const letter = computed(() => (props.feed?.title || "?").trim().charAt(0).toUpperCase() || "?");
// stable muted hue per feed so letter avatars are distinguishable
const color = computed(() => {
  const s = props.feed?.title || "";
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360;
  return `hsl(${h} 32% 46%)`;
});
</script>
