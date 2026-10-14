<template>
  <template v-if="state.menu">
    <div class="scrim" :class="{ 'is-clear': wide }" @click="closeMenu()"></div>
    <div ref="el" class="menu" role="menu" :aria-label="state.menu.title || 'Actions'" :style="pos" @keydown="onKey">
      <div v-if="state.menu.title" class="menu-title">{{ state.menu.title }}</div>
      <template v-for="(e, i) in state.menu.entries" :key="i">
        <div v-if="e.divider" class="menu-divider" role="separator"></div>
        <div v-else-if="e.header" class="menu-header" role="presentation">{{ e.header }}</div>
        <button
          v-else
          class="menu-item"
          :class="{ 'is-danger': e.danger }"
          role="menuitem"
          type="button"
          @click="choose(e)">
          <v-icon v-if="e.icon" :name="e.icon" />
          <span>{{ e.label }}</span>
        </button>
      </template>
    </div>
  </template>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { state, closeMenu, type MenuEntry } from "../store";
import VIcon from "./Icon.vue";

const el = ref<HTMLElement>();
const wide = ref(window.matchMedia("(min-width: 640px)").matches);
const size = ref({ w: 260, h: 300 });
let returnFocus: Element | null = null;

// anchored popover on wider screens, bottom sheet (pure CSS) on phones
const pos = computed(() => {
  const a = state.menu?.anchor;
  if (!a || !wide.value) return {};
  const margin = 8;
  // pointer anchors (context menu) open to the right of the cursor
  const preferred = a.width === 0 ? a.left : a.right - size.value.w;
  let left = Math.min(preferred, window.innerWidth - size.value.w - margin);
  left = Math.max(margin, left);
  let top = a.width === 0 ? a.top : a.bottom + 6;
  if (top + size.value.h > window.innerHeight - margin) top = Math.max(margin, a.top - size.value.h - 6);
  return { left: left + "px", top: top + "px" };
});

watch(
  () => state.menu,
  m => {
    if (!m) {
      (returnFocus as HTMLElement | null)?.focus?.();
      return;
    }
    returnFocus = document.activeElement;
    wide.value = window.matchMedia("(min-width: 640px)").matches;
    nextTick(() => {
      if (!el.value) return;
      size.value = { w: el.value.offsetWidth, h: el.value.offsetHeight };
      (el.value.querySelector(".menu-item") as HTMLElement | null)?.focus({ preventScroll: true });
    });
  },
);

function choose(e: MenuEntry) {
  closeMenu();
  e.action?.();
}

function onKey(ev: KeyboardEvent) {
  const items = Array.from(el.value?.querySelectorAll<HTMLElement>(".menu-item") ?? []);
  const i = items.indexOf(document.activeElement as HTMLElement);
  if (ev.key === "ArrowDown") items[(i + 1) % items.length]?.focus();
  else if (ev.key === "ArrowUp") items[(i - 1 + items.length) % items.length]?.focus();
  else if (ev.key === "Escape" || ev.key === "Tab") closeMenu();
  else return;
  ev.preventDefault();
  ev.stopPropagation();
}
</script>
