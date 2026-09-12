import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { "@lucide/vue-next": "lucide-vue-next" } },
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    port: 5173,
    proxy: {
      "/auth": "http://127.0.0.1:2485",
      "/api": "http://127.0.0.1:2485",
      "/certificate": "http://127.0.0.1:2485",
      "/fast": "http://127.0.0.1:2485",
      "/health": "http://127.0.0.1:2485",
    },
  },
});
