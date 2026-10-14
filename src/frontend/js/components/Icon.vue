<template>
  <span class="icon" :class="{ 'is-small': small }" aria-hidden="true" v-html="svg"></span>
</template>

<script setup lang="ts">
import { computed } from "vue";
import icons from "../icons";

const props = defineProps<{ name: string; small?: boolean }>();
const svg = computed(() => (icons as Record<string, string>)[props.name] ?? "");
</script>
