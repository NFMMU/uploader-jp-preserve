import { publicAPI } from "../../../api/public";
import { TurnstileField } from "../../../components/TurnstileField";
import { FormEvent, useState } from "react";
import { Link } from "react-router-dom";

export function NodeParticipationRequest() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [capacity, setCapacity] = useState("10737418240");
  const [turnstileReset, setTurnstileReset] = useState(0);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setSuccess("");
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      const result = await publicAPI<{ message: string }>(
        "/public/node-enrollment-requests",
        {
          method: "POST",
          body: JSON.stringify({
            turnstile_token: String(form.get("cf-turnstile-response") || ""),
            name: String(form.get("name") || "").trim(),
            node_type: String(form.get("node_type") || "B"),
            requested_capacity_bytes: Number(capacity),
            contact_email: String(form.get("contact_email") || "").trim(),
            message: String(form.get("message") || "").trim(),
            agree_to_terms: form.get("agree_to_terms") === "on",
          }),
        },
      );
      setSuccess(result.message || "参加申請を受け付けました。");
      formElement.reset();
      setCapacity("10737418240");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
      setTurnstileReset((value) => value + 1);
    }
  }
  return (
    <>
      <h1>ストレージノード参加申請</h1>
      <p className="public-intro">
        ご自身のサーバーや PC
        のストレージを、ファイル保全ネットワークに提供するための申請です。申請は運営が確認し、承認された場合に限り登録用の一回限りトークンを発行します。
      </p>
      {success ? (
        <div className="public-success">
          <h2>申請を受け付けました</h2>
          <p>{success}</p>
          <p>運営が申請内容を確認し、入力された連絡先へ別途連絡します。</p>
          <button type="button" onClick={() => setSuccess("")}>
            別の申請を行う
          </button>
        </div>
      ) : (
        <form className="public-request-form" onSubmit={submit}>
          <label>
            ノード名 *
            <input
              name="name"
              required
              minLength={1}
              maxLength={16}
              pattern="[a-z0-9][a-z0-9_\-]{0,15}"
              placeholder="例：my-storage-node-01"
            />
            <small>
              英数-_小文字が利用可能です。-_が先頭に来る文字列は利用できません。
            </small>
          </label>
          <label>
            ノード種別 *
            <select name="node_type" defaultValue="B">
              <option value="A">A</option>
              <option value="B">B</option>
            </select>
            <small>運営者からの指定がない限りBを選択してください。</small>
          </label>
          <label>
            提供容量（バイト） *
            <input
              type="number"
              min="0"
              step="1"
              max={Number.MAX_SAFE_INTEGER}
              required
              value={capacity}
              onChange={(e) => setCapacity(e.target.value)}
            />
            <small>
              0 は参加者側で容量上限を設定しない指定です。例：10 GiB =
              10737418240
            </small>
            <small>
              容量の計算にはこちらのサイトも便利です。
              <a
                href="https://keisan.site/ja/exec/user/1355670543"
                target="_blank"
              >
                バイト換算
              </a>
            </small>
          </label>
          <label>
            連絡先メールアドレス *
            <input
              name="contact_email"
              type="email"
              required
              maxLength={320}
              autoComplete="email"
              placeholder="you@example.com"
            />
          </label>
          <label>
            補足（任意）
            <textarea
              name="message"
              rows={4}
              maxLength={1000}
              placeholder="稼働予定、回線・ストレージ構成など"
            />
          </label>
          <label className="check-label">
            <input type="checkbox" name="agree_to_terms" required />{" "}
            <span>
              <Link to="/terms" target="_blank">
                利用規約
              </Link>
              、
              <Link to="/privacy" target="_blank">
                プライバシー・ポリシー
              </Link>
              および参加ノード運用上の注意を確認し、ノード参加申請に必要な情報を送信することに同意します。
            </span>
          </label>
          <TurnstileField key={turnstileReset} />
          {error && <p className="public-error">{error}</p>}
          <button type="submit" disabled={busy}>
            {busy ? "申請中…" : "参加申請を送信"}
          </button>
          <p className="public-footnote">
            承認前にノードを接続しないでください。登録トークンは申請ごとに発行され、有効期限は72時間、登録成功後は再利用できません。ノードの保存データは第三者の権利物を含む場合があります。参加条件を確認し、不明点は{" "}
            <a href="mailto:uploaderjp-archive@udonya.moe">運営窓口</a>{" "}
            にお問い合わせください。
          </p>
        </form>
      )}
    </>
  );
}
