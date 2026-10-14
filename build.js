import esbuild from "esbuild";
import vuePlugin from "esbuild-plugin-vue3";
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";

const args = process.argv.slice(2);
const isWatch = args.includes("--watch");
const isMinify = args.includes("--minify") || process.env.NODE_ENV === "production";

const options = {
  entryPoints: ["src/frontend/js/main.ts", "src/frontend/css/app.css"],
  bundle: true,
  outdir: "src/assets/static",
  entryNames: "bundle",
  // runtime-only build: templates are precompiled, so the app works under a
  // strict Content-Security-Policy without 'unsafe-eval'
  alias: { vue: "vue/dist/vue.runtime.esm-bundler.js" },
  define: {
    __VUE_OPTIONS_API__: "true",
    __VUE_PROD_DEVTOOLS__: "false",
    __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: "false",
  },
  plugins: [vuePlugin()],
  target: ["es2020", "safari15"],
  sourcemap: !isMinify,
  minify: isMinify,
  logLevel: "info",
};

// The service worker is built separately (it is not a module of the app) and
// stamped with a hash of the app bundle so a new build always invalidates the
// offline cache.
async function buildServiceWorker() {
  let stamp = "dev";
  try {
    const h = createHash("sha256");
    h.update(readFileSync("src/assets/static/bundle.js"));
    h.update(readFileSync("src/assets/static/bundle.css"));
    stamp = h.digest("hex").slice(0, 12);
  } catch {}
  await esbuild.build({
    entryPoints: ["src/frontend/sw/sw.ts"],
    bundle: true,
    outfile: "src/assets/static/sw.js",
    target: ["es2020", "safari15"],
    minify: isMinify,
    define: { __CACHE_VERSION__: JSON.stringify(stamp) },
    logLevel: "info",
  });
}

if (isWatch) {
  const ctx = await esbuild.context({
    ...options,
    plugins: [...options.plugins, { name: "sw", setup(b) { b.onEnd(buildServiceWorker); } }],
  });
  await ctx.watch();
  console.log("Watching for changes...");
} else {
  await esbuild.build(options);
  await buildServiceWorker();
}
