import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import vike from "vike/plugin";
export default defineConfig({
	plugins: [react(), vike()],
	server: {
		port: 5173,
		proxy: {
			"/api": "http://localhost:8080",
			"/healthz": "http://localhost:8080",
		},
	},
	 preview: {
    allowedHosts: ["uploaderjp.udonya.moe"],
  },
});
