import { useState } from "react";

type CopyEmailProps = { email: string; label?: string };

export default function CopyEmail({ email, label }: CopyEmailProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(email);
      } else {
        const input = document.createElement("textarea");
        input.value = email;
        input.style.position = "fixed";
        input.style.opacity = "0";
        document.body.appendChild(input);
        input.select();
        const ok = document.execCommand("copy");
        input.remove();
        if (!ok) throw new Error("copy failed");
      }
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      window.prompt("メールアドレスを手動でコピーしてください。", email);
    }
  }

  return (
    <button type="button" className="copy-email" onClick={copy} title={email}>
      <span>{label ?? email}</span>
      <small>{copied ? "コピーしました" : "コピー"}</small>
    </button>
  );
}
