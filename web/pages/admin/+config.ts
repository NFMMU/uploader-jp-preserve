import type { Config } from "vike/types";

export default {
	// The authenticated admin console depends on browser localStorage.
	// Keep it client-rendered while public pages get SSR metadata.
	ssr: false,
} satisfies Config;
