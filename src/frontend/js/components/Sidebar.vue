<template>
  <aside class="sidebar" aria-label="Navigation" :inert="state.sidebarCollapsed || undefined">
    <div class="sidebar-head">
      <div class="brand"><img :src="'./static/icons/icon-192.png'" alt="" />Yarrow</div>
    </div>
    <div class="sidebar-scroll">
      <v-nav-list />
    </div>
    <div class="sidebar-foot">
      <div v-if="state.refreshing > 0" class="refresh-status" role="status">
        <span class="spinner"></span> Refreshing {{ state.refreshing }} feed{{ state.refreshing === 1 ? "" : "s" }}…
      </div>
      <button class="nav-item" type="button" @click="state.sheet = 'add-feed'">
        <v-icon name="plus" /><span class="label">Add feed</span>
      </button>
      <button class="nav-item" type="button" :aria-current="state.page === 'settings'" @click="state.page = 'settings'">
        <v-icon name="settings" /><span class="label">Settings</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { state } from "../store";
import VIcon from "./Icon.vue";
import VNavList from "./NavList.vue";
</script>
