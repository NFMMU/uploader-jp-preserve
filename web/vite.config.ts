import react from "@vitejs/plugin-react";
import vike from "vike/plugin";
import { defineConfig } from "vite";
import { publicNoticesPlugin } from "./plugins/publicNotices";

const allowedHosts = ["uploaderjp.udonya.moe"];

export default defineConfig({
	plugins: [publicNoticesPlugin(), react(), vike()],
	server: {
		host: "0.0.0.0",
		port: 5173,
		allowedHosts,
		proxy: {
			"/api": "http://api:8080",
			"/healthz": "http://api:8080",
		},
	},
	preview: {
		host: "0.0.0.0",
		port: 3000,
		allowedHosts,
	},
});
