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
    turnstileScriptPromise = new Promise((resolve, reject) => {
      const existing = document.querySelector<HTMLScriptElement>(
        "script[data-cloudflare-turnstile]",
      );
      if (existing) {
        existing.addEventListener("load", () => resolve(), { once: true });
        existing.addEventListener(
          "error",
          () => reject(new Error("Turnstile script failed to load")),
          { once: true },
        );
        return;
      }
      const script = document.createElement("script");
      script.src =
        "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit&language=ja";
      script.async = true;
      script.defer = true;
      script.dataset.cloudflareTurnstile = "true";
      script.onload = () => resolve();
      script.onerror = () =>
        reject(new Error("Turnstile script failed to load"));
      document.head.appendChild(script);
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
