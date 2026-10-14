<template>
  <main class="login">
    <form @submit.prevent="login">
      <div class="brand"><img :src="'./static/icons/icon-192.png'" alt="" />Yarrow</div>
      <div class="field">
        <label for="username">Username</label>
        <input id="username" name="username" class="input" autocomplete="username" autocapitalize="off" required autofocus />
      </div>
      <div class="field">
        <label for="password">Password</label>
        <input id="password" name="password" class="input" type="password" autocomplete="current-password" required />
      </div>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button class="btn" type="submit" :disabled="busy">Sign in</button>
    </form>
  </main>
</template>

<script setup lang="ts">
import { ref } from "vue";

const props = defineProps<{ onLogin: () => void }>();
const error = ref("");
const busy = ref(false);

async function login(e: Event) {
  busy.value = true;
  error.value = "";
  try {
    const res = await fetch("./login", { method: "POST", body: new FormData(e.target as HTMLFormElement) });
    if (res.ok) props.onLogin();
    else error.value = "Wrong username or password.";
  } catch {
    error.value = "Can't reach the server.";
  } finally {
    busy.value = false;
  }
}
</script>
