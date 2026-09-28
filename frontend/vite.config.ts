import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [tailwindcss(), reactRouter(), tsconfigPaths()],
  server: {
    proxy: {
      "/levels": "http://127.0.0.1:8080",
      "/nodes": "http://127.0.0.1:8080",
      "/integrity": "http://127.0.0.1:8080",
      "/relationships": "http://127.0.0.1:8080",
      "/assignment": "http://127.0.0.1:8080",
      "/layout": "http://127.0.0.1:8080",
      "/proposals": "http://127.0.0.1:8080",
      "/events": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
});
