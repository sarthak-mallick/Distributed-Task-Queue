import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// defineConfig centralizes local dev/build settings for the UI app.
export default defineConfig({
  plugins: [react()],
});
