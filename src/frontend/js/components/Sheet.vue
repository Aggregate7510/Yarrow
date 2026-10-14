<template>
  <div class="scrim" @click="$emit('close')"></div>
  <div class="sheet" role="dialog" aria-modal="true" :aria-label="title" ref="el" @keydown.esc.stop="$emit('close')">
    <div class="sheet-grip" aria-hidden="true"></div>
    <div class="sheet-head">
      <h2>{{ title }}</h2>
      <button class="icon-btn" type="button" aria-label="Close" @click="$emit('close')"><v-icon name="x" /></button>
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import VIcon from "./Icon.vue";

defineProps<{ title: string }>();
defineEmits<{ close: [] }>();
const el = ref<HTMLElement>();
onMounted(() => {
  const target = el.value?.querySelector<HTMLElement>("[autofocus], input, button:not([aria-label='Close'])");
  target?.focus({ preventScroll: true });
});
</script>
