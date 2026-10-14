import { createApp, defineComponent, h, ref } from "vue";
import App from "./pages/App.vue";
import Login from "./pages/Login.vue";
import api from "./api";
import { applyTheme, watchSystemTheme } from "./theme";
import { state } from "./store";

// configuration is embedded by the server as JSON (no inline script, so the
// page can run under a strict Content-Security-Policy)
const configEl = document.getElementById("app-config");
window.app = configEl ? JSON.parse(configEl.textContent || "{}") : {};
Object.assign(state.settings, window.app.settings || {});
state.requiresAuth = !!window.app.requiresAuth;
state.version = window.app.version || "";

applyTheme(state.settings.theme_name);
watchSystemTheme(() => state.settings.theme_name);

const ready = ref(!!window.app.authenticated);

const Root = defineComponent({
  setup() {
    const onLogin = async () => {
      const settings = await api.settings.get();
      Object.assign(state.settings, settings);
      applyTheme(settings.theme_name);
      ready.value = true;
    };
    return () => (ready.value ? h(App) : h(Login, { onLogin }));
  },
});

createApp(Root).mount("#app");

if ("serviceWorker" in navigator && window.isSecureContext) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("./sw.js", { scope: "./" }).catch(() => {});
  });
}
