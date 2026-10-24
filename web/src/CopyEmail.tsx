import { useState } from 'react';
import { Check, Copy } from 'lucide-react';

export default function CopyEmail({ email }: { email: string }) {
  const [copied, setCopied] = useState(false);
  async function copyEmail() {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(email);
      } else {
        const input = document.createElement('textarea');
        input.value = email;
        input.setAttribute('readonly', '');
        input.style.position = 'fixed';
        input.style.opacity = '0';
        document.body.appendChild(input);
        input.select();
        const succeeded = document.execCommand('copy');
        document.body.removeChild(input);
        if (!succeeded) throw new Error('copy failed');
      }
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
      window.prompt('メールアドレスを手動でコピーしてください', email);
    }
  }
  return <button type="button" className="copy-email-btn" onClick={copyEmail} aria-label="メールアドレスをコピー" title="メールアドレスをコピー">{copied ? <Check size={13} /> : <Copy size={13} />}{copied ? 'コピー済み' : 'コピー'}</button>;
}
