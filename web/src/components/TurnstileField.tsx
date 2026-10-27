import { useEffect, useRef, useState } from "react";
import { publicAPI } from "../api/public";

type TurnstileOptions = {
	sitekey: string;
	theme?: "light" | "dark" | "auto";
	size?: "normal" | "compact" | "flexible";
	"response-field"?: boolean;
	"response-field-name"?: string;
	"error-callback"?: () => void;
};

declare global {
	interface Window {
		turnstile?: {
			render: (container: HTMLElement, options: TurnstileOptions) => string;
			remove: (widgetID: string) => void;
		};
	}
}

let turnstileScriptPromise: Promise<void> | null = null;

function loadTurnstileScript(): Promise<void> {
	if (window.turnstile) return Promise.resolve();
	if (!turnstileScriptPromise) {
		// With no API or pending promise, an existing script may have already fired
		// its load/error event. Replace it rather than waiting for a past event.
		document.querySelector("script[data-cloudflare-turnstile]")?.remove();
		const script = document.createElement("script");
		script.src =
			"https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit&language=ja";
		script.async = true;
		script.defer = true;
		script.dataset.cloudflareTurnstile = "true";
		turnstileScriptPromise = new Promise<void>((resolve, reject) => {
			const cleanup = () => {
				script.removeEventListener("load", onLoad);
				script.removeEventListener("error", onError);
			};
			const onLoad = () => {
				cleanup();
				if (window.turnstile) resolve();
				else reject(new Error("Turnstile API unavailable after script load"));
			};
			const onError = () => {
				cleanup();
				reject(new Error("Turnstile script failed to load"));
			};
			script.addEventListener("load", onLoad, { once: true });
			script.addEventListener("error", onError, { once: true });
			document.head.appendChild(script);
		}).catch((error) => {
			turnstileScriptPromise = null;
			script.remove();
			throw error;
		});
	}
	return turnstileScriptPromise;
}

function TurnstileField() {
	const containerRef = useRef<HTMLDivElement>(null);
	const [siteKey, setSiteKey] = useState("");
	const [status, setStatus] = useState<
		"loading" | "disabled" | "ready" | "error"
	>("loading");

	useEffect(() => {
		let active = true;
		publicAPI<{ turnstile_enabled: boolean; turnstile_site_key?: string }>(
			"/public/config",
		)
			.then((config) => {
				if (!active) return;
				if (!config.turnstile_enabled || !config.turnstile_site_key) {
					setStatus("disabled");
					return;
				}
				setSiteKey(config.turnstile_site_key);
			})
			.catch(() => {
				if (active) setStatus("error");
			});
		return () => {
			active = false;
		};
	}, []);

	useEffect(() => {
		if (!siteKey || !containerRef.current) return;
		let active = true;
		let widgetID = "";
		loadTurnstileScript()
			.then(() => {
				if (!active || !window.turnstile || !containerRef.current) return;
				widgetID = window.turnstile.render(containerRef.current, {
					sitekey: siteKey,
					theme: "auto",
					size: "normal",
					"response-field": true,
					"response-field-name": "cf-turnstile-response",
					"error-callback": () => {
						if (active) setStatus("error");
					},
				});
				setStatus("ready");
			})
			.catch(() => {
				if (active) setStatus("error");
			});
		return () => {
			active = false;
			if (widgetID && window.turnstile) window.turnstile.remove(widgetID);
		};
	}, [siteKey]);

	return (
		<div className="turnstile-field" aria-label="Cloudflare Turnstile 認証">
			<div ref={containerRef} />
			{status === "loading" && (
				<p className="turnstile-note">認証を準備しています…</p>
			)}
			{status === "disabled" && (
				<p className="turnstile-note">
					開発環境では Cloudflare Turnstile を無効にしています。
				</p>
			)}
			{status === "error" && (
				<p className="public-error">
					Cloudflare Turnstile
					を読み込めません。通信設定を確認して、ページを再読み込みしてください。
				</p>
			)}
		</div>
	);
}

export { TurnstileField };
