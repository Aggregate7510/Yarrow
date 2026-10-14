<template>
  <time :datetime="val" :title="full">{{ text }}</time>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { dateRepr, dateTimeString, relRepaintDelay } from "../utils";

const props = defineProps<{ val: string }>();
const text = ref("");
let timer: number | undefined;
const date = computed(() => new Date(props.val));
const full = computed(() => dateTimeString(date.value, navigator.language));

function repaint() {
  window.clearTimeout(timer);
  text.value = dateRepr(date.value, navigator.language);
  const delay = relRepaintDelay(date.value);
  if (delay !== null) timer = window.setTimeout(repaint, delay);
}
repaint();
watch(() => props.val, repaint);
onUnmounted(() => window.clearTimeout(timer));
</script>
