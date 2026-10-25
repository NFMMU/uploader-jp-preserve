import { FormEvent, useEffect, useRef, useState } from "react";
import {
  Link,
  Navigate,
  Route,
  Routes,
  useParams,
  useSearchParams,
} from "react-router-dom";
import CopyEmail from "./CopyEmail";

type PublicFile = {
  id: string;
  original_filename: string;
  size_bytes: number;
  media_type?: string;
  created_at: string;
};
type PublicArtifact = {
  id: string;
  title: string;
  creator_name?: string;
  source_url: string;
  description?: string;
  created_at: string;
  files: PublicFile[];
};
type APIError = { error?: string; message?: string };

async function publicAPI<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...options,
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
  });
  const text = await response.text();
  let data: any = {};
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = { message: text };
  }
  if (!response.ok)
    throw new Error(
      (data as APIError).error ||
        (data as APIError).message ||
        `HTTP ${response.status}`,
    );
  return data as T;
}

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

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = size / 1024;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(1)} ${units[index]}`;
}

function PublicHeader() {
  return (
    <header className="public-header">
      <div className="public-header-inner">
        <Link className="public-brand" to="/">
          uploader.jp保全プロジェクト
        </Link>
        <nav aria-label="メインナビゲーション">
          <Link to="/">作品一覧</Link>
          <Link to="/preservation-requests">保全リクエスト</Link>
          <Link to="/redistribution-support">再配布の相談</Link>
          <Link to="/publisher-support">配布元向け代行</Link>
          <Link to="/requests">公開停止・削除</Link>
          <Link to="/relocation-info">移設先情報の提供</Link>
          <Link to="/participate">ストレージノード参加</Link>
          <Link to="/admin/login">管理者ログイン</Link>
        </nav>
      </div>
    </header>
  );
}

export default function PublicSite() {
  return (
    <div className="public-shell">
      <PublicHeader />
      <main className="public-main">
        <Routes>
          <Route path="/" element={<PublicCatalog />} />
          <Route path="/artifacts/:id" element={<PublicArtifactDetail />} />
          <Route path="/requests" element={<PublicRequest />} />
          <Route
            path="/preservation-requests"
            element={<PreservationRequest />}
          />
          <Route
            path="/redistribution-support"
            element={<RedistributionAssistancePage mode="redistribution" />}
          />
          <Route
            path="/publisher-support"
            element={<RedistributionAssistancePage mode="publisher" />}
          />
          <Route path="/relocation-info" element={<RelocationInfoRequest />} />
          <Route path="/participate" element={<NodeParticipationRequest />} />
          <Route path="/terms" element={<TermsPage />} />
          <Route path="/privacy" element={<PrivacyPage />} />
          <Route path="/disclaimer" element={<DisclaimerPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
      <footer className="public-footer">
        <div>
          <b>uploader.jp保全プロジェクト</b>
          <span>運営：全国もちもちうどん連盟</span>
          <span>© 2026 全国もちもちうどん連盟</span>
        </div>
        <div className="footer-right">
          <Link to="/terms">利用規約</Link>
          <Link to="/privacy">プライバシー・ポリシー</Link>
          <Link to="/disclaimer">免責事項</Link>
          <Link to="/participate">ストレージノード参加</Link>
          <a href="mailto:uploaderjp-archive@udonya.moe">お問い合わせ</a>
          <CopyEmail email="uploaderjp-archive@udonya.moe" />
        </div>
      </footer>
    </div>
  );
}

function LegalLayout({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <article className="public-legal">
      <h1>{title}</h1>
      <p className="legal-updated">制定・最終更新日：2026年10月25日</p>
      {children}
      <p className="legal-contact">
        本ページに関するお問い合わせ：
        <a href="mailto:uploaderjp-archive@udonya.moe">
          uploaderjp-archive@udonya.moe
        </a>{" "}
        <CopyEmail email="uploaderjp-archive@udonya.moe" />
      </p>
    </article>
  );
}

function TermsPage() {
  return (
    <LegalLayout title="利用規約">
      <p>
        この利用規約（以下「本規約」）は、全国もちもちうどん連盟（以下「運営者」）が運営する「uploader.jp保全プロジェクト」（以下「本サービス」）の利用条件を定めるものです。本サービスを利用する方は、本規約を確認のうえ利用してください。
      </p>
      <h2>第1条（目的・適用）</h2>
      <p>
        本サービスは、適法に保全する権限のあるファイルについて、作品情報・ファイル情報を管理し、複数の保存ノードに保全するための試験運用サービスです。本規約は、本サービスを閲覧、利用、運営または保存ノードへの参加をするすべての方に適用されます。
      </p>
      <h2>第2条（保全と公開の区別）</h2>
      <p>
        ファイルを保全することと、一般公開・ダウンロード提供・再配布することは別の判断です。一般公開は、権利および利用条件の確認後、運営者が別途有効化した場合に限ります。カタログに掲載されていることは、第三者の権利が存在しないことや、自由な再配布ができることを意味しません。
      </p>
      <h2>第3条（利用者の責任）</h2>
      <p>
        利用者は、ファイルを保全・提供するために必要な権利、許諾その他の正当な根拠を有することを確認してください。利用者は、登録情報を正確に保ち、自らのアカウントと認証情報を適切に管理するものとします。
      </p>
      <h2>第4条（禁止事項）</h2>
      <ul>
        <li>
          著作権、著作者人格権、商標権、プライバシーその他第三者の権利を侵害する行為
        </li>
        <li>法令、公序良俗または第三者の利用条件に違反する行為</li>
        <li>
          アクセス制御、パスワード、画像認証その他の技術的制限を回避してファイルを取得する行為
        </li>
        <li>
          本サービス、保存ノード、他の利用者に過度な負荷や損害を与える行為
        </li>
        <li>
          認証情報の不正利用、脆弱性の悪用、マルウェアの配布、運営を妨害する行為
        </li>
        <li>運営者が不適切と判断するその他の行為</li>
      </ul>
      <h2>第5条（再配布相談・配布代行）</h2>
      <p>
        利用者は、再配布相談・配布代行のためにファイルを送信できます。送信ファイルは権利関係の確認・問い合わせ・審査のために扱い、送信だけで一般公開・再配布を行うものではありません。運営者は権利者への問い合わせを試みる場合がありますが、連絡や許諾の取得、再配布の実施を保証しません。権利者の許諾や配布条件を確認できない場合、公開・再配布を行いません。申請が却下または終了となった場合、申請添付ファイルは削除します。ただし、障害や法令上の理由等により削除まで時間を要する場合があります。
      </p>
      <h2>第6条（公開停止・削除の申請）</h2>
      <p>
        権利者または正当な利害関係を有する方は、本サービスの申請フォームから公開停止・削除を申し出ることができます。運営者は、申請内容、対象の特定可能性、権利関係その他の事情を確認し、必要に応じて追加情報を求めます。申請の受付は、直ちに削除完了を意味しません。公開停止はダウンロードを停止するための措置であり、保存ノード上のデータ削除には各ノードの処理・接続状況に応じて時間を要する場合があります。
      </p>
      <h2>第7条（保存ノードへの参加）</h2>
      <p>
        保存ノードの提供者は、参加時に提示される設定・運用上の注意を確認し、保存領域、認証情報、OS・コンテナ環境を適切に管理するものとします。ノードはネットワーク障害、故障、容量不足その他の理由により停止することがあります。
      </p>
      <h2>第8条（サービスの変更・停止）</h2>
      <p>
        運営者は、保守、障害、セキュリティ上の必要、法令対応その他の理由により、本サービスの全部または一部を変更・中断・終了することがあります。可能な場合は事前に告知しますが、緊急時はこの限りではありません。
      </p>
      <h2>第9条（保証の否認）</h2>
      <p>
        本サービスは試験運用版として提供されます。運営者は、データの完全性、継続的な可用性、特定目的への適合性、エラーの不存在、第三者サイトの存続または取得元の内容について、明示・黙示を問わず保証しません。
      </p>
      <h2>第10条（規約の変更）</h2>
      <p>
        運営者は必要に応じて本規約を変更できます。重要な変更は本サービス上で告知し、変更後の規約は掲載時に定める効力発生日から適用されます。
      </p>
      <h2>第11条（準拠法）</h2>
      <p>
        本規約は日本法に準拠します。本サービスに関して紛争が生じた場合、適用法令に従い管轄権を有する裁判所を利用します。他の法令による制限を受けない場合、本サービスの合意管轄は、津地方裁判所です。
      </p>
    </LegalLayout>
  );
}

function PrivacyPage() {
  return (
    <LegalLayout title="プライバシー・ポリシー">
      <p>
        全国もちもちうどん連盟（以下「運営者」）は、uploader.jp保全プロジェクト（以下「本サービス」）において取り扱う情報について、以下のとおり定めます。
      </p>
      <h2>1. 取得する情報</h2>
      <ul>
        <li>管理アカウントのメールアドレス、表示名、権限、認証・利用履歴</li>
        <li>
          申請フォームに入力された氏名・表示名、連絡先、申請内容、対象URL、対象ファイル・作品の識別情報
        </li>
        <li>
          再配布相談・配布代行の申請で送信されたファイル、ファイル名、サイズ、ハッシュ値
        </li>
        <li>
          作品・ファイルのメタデータ、保全・公開設定、操作履歴、申請の審査履歴
        </li>
        <li>
          接続時刻、IPアドレス、ユーザーエージェント、エラー情報など、セキュリティ・障害対応に必要な技術情報
        </li>
        <li>
          管理画面の認証状態を維持するため、ブラウザーのローカルストレージに保存される認証トークン
        </li>
      </ul>
      <h2>2. 利用目的</h2>
      <ul>
        <li>本サービスの提供、運営、保全処理、カタログ管理</li>
        <li>
          保全・公開停止・削除・移設先情報・再配布相談・配布代行などの申請受付、本人確認、審査、連絡
        </li>
        <li>不正利用の防止、権限管理、監査、セキュリティ確保、障害調査</li>
        <li>法令・規約違反への対応、権利者からの申告への対応</li>
        <li>サービスの改善および運用状況の把握</li>
      </ul>
      <h2>3. 第三者への提供・保存ノードとの共有</h2>
      <p>
        運営者は、法令に基づく場合、生命・身体・財産の保護に必要な場合、本人の同意がある場合、またはサービス運用上必要な委託を行う場合を除き、個人情報を第三者に提供しません。保全対象ファイルは、保全処理のため参加保存ノードへ転送・複製されることがあります。申請に記載された連絡先などの情報は、対応に必要な権限を持つ担当者に限定して取り扱います。公開フォームの不正送信防止には
        Cloudflare Turnstile を利用します。認証判定に必要な IP
        アドレスやブラウザー・通信に関する情報等が Cloudflare に送信され、
        <a
          href="https://www.cloudflare.com/privacypolicy/"
          target="_blank"
          rel="noreferrer"
        >
          Cloudflare のプライバシーポリシー
        </a>
        に従って処理される場合があります。
      </p>
      <h2>4. 委託・国外での処理</h2>
      <p>
        ホスティング、通信、監視その他のインフラを利用する場合、当該サービス提供者が情報を処理することがあります。利用するインフラの所在地や設定によっては、日本国外で情報が処理・保存される可能性があります。運営者は、委託先・利用サービスの構成に応じて適切な管理に努めます。
        また、このサービスの運営にあたり権利に関する要請やその他お問い合わせに対応するためのボランティアを動員しています。ボランティアの方については秘密保持契約を交わし、適切な情報の管理に努めます。
      </p>
      <h2>5. 保存期間と削除</h2>
      <p>
        運営者は、利用目的の達成に必要な期間、または法令・セキュリティ・監査上必要な期間、情報を保存します。再配布相談で送信されたファイルは、権利確認・問い合わせ・審査のために一時保管し、一般公開はしません。保管期間や削除の相談はお問い合わせ窓口へご連絡ください。削除申請に基づくデータ削除では、ノードのオフライン、処理失敗、バックアップ等の事情により、すべての複製が直ちに消去されない場合があります。法令上または正当な業務上保存が必要な記録は、対象データの削除後も一定期間保持されることがあります。
      </p>
      <h2>6. 安全管理</h2>
      <p>
        運営者は、アクセス制御、権限管理、通信の保護、監査ログその他の合理的な安全管理措置を講じるよう努めます。ただし、インターネット通信や分散保存に絶対的な安全性を保証するものではありません。
      </p>
      <h2>7. Cookie・ブラウザー保存情報</h2>
      <p>
        本サービスの管理画面は、ログイン状態を維持するためローカルストレージを使用します。ブラウザーの設定から保存情報を削除できますが、その場合は再ログインが必要になります。現在、独自の広告配信を目的とするCookieの利用は予定していません。運用構成の変更により追加する場合は、本ページを更新します。
      </p>
      <h2>8. 開示・訂正・利用停止等</h2>
      <p>
        自身の情報に関する相談・請求は、下記の窓口へご連絡ください。本人確認や対象情報の特定に必要な情報をお願いする場合があります。法令上の義務、第三者の権利、セキュリティまたは監査上の必要により、請求に応じられない場合があります。
      </p>
      <h2>9. 本ポリシーの変更</h2>
      <p>
        法令やサービス内容の変更に応じて、本ポリシーを更新することがあります。重要な変更は本サービス上で告知します。
      </p>
    </LegalLayout>
  );
}

function DisclaimerPage() {
  return (
    <LegalLayout title="免責事項">
      <h2>1. 試験運用サービス</h2>
      <p>
        本サービスは試験運用版です。障害、保守、ノード停止、ネットワーク切断、容量不足、ソフトウェアの不具合その他の理由により、サービスの全部または一部を利用できない場合があります。
      </p>
      <h2>2. データ保全に関する注意</h2>
      <p>
        複数ノードへの保存を目指していますが、ファイルの永続的な保存、完全性、復元可能性、保存ノードの継続稼働を保証しません。本サービスを唯一のバックアップ先として使用せず、重要なデータは利用者自身でも保管してください。保存・検証状況はシステムが観測した時点の情報であり、将来の可用性を保証するものではありません。
      </p>
      <h2>3. 権利・利用許諾</h2>
      <p>
        本サービス上の作品名、作者名、ファイル名、URLその他の情報は、利用者または第三者が提供した情報を含みます。運営者は、個別コンテンツについて権利関係、正確性、合法性、完全性を常に確認できるとは限りません。利用者は、権利や許諾を確認せずにファイルを取得・公開・再配布しないでください。ソースコードのライセンスは、保全対象コンテンツに適用されるライセンスや権利を変更しません。
      </p>
      <h2>4. 外部サイト</h2>
      <p>
        本サービスから外部サイトへリンクする場合があります。外部サイトの内容、可用性、セキュリティ、利用条件、プライバシー慣行について、運営者は管理・保証しません。外部サイトの利用は、各サイトの規約・ポリシーに従い、利用者自身の判断と責任で行ってください。
      </p>
      <h2>5. 公開停止・削除</h2>
      <p>
        運営者は申請を確認し、必要な対応に努めますが、申請内容の不足、対象の特定困難、ノードのオフライン、削除タスクの失敗、既に第三者が取得した複製などにより、停止・削除が直ちに、または完全に完了することを保証できません。対応を急ぐ必要がある場合は、申請に加えて問い合わせ窓口へ連絡してください。
      </p>
      <h2>6. 損害に関する責任</h2>
      <p>
        運営者は、適用法令で認められる範囲において、本サービスの利用または利用不能、データの消失・破損、第三者との紛争、外部サイトの利用等により生じた損害について責任を負いません。ただし、適用法令により免責が認められない責任を排除するものではありません。
      </p>
      <h2>7. 情報の正確性</h2>
      <p>
        運営者は掲載情報の正確性・最新性の維持に努めますが、完全性を保証しません。誤りや権利侵害のおそれを見つけた場合は、対象URLと具体的な理由を添えてご連絡ください。
      </p>
      <h2>8. 免責事項の変更</h2>
      <p>
        本免責事項は必要に応じて変更されることがあります。最新の内容を本ページで確認してください。
      </p>
    </LegalLayout>
  );
}

function NodeParticipationRequest() {
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
              maxLength={120}
              placeholder="例：my-storage-node-01"
            />
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

function PublicCatalog() {
  const [query, setQuery] = useState("");
  const [submitted, setSubmitted] = useState("");
  const [items, setItems] = useState<PublicArtifact[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    setLoading(true);
    publicAPI<{ items: PublicArtifact[] }>(
      `/public/artifacts?q=${encodeURIComponent(submitted)}`,
    )
      .then((data) => {
        if (active) {
          setItems(data.items || []);
          setError("");
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [submitted]);
  function search(event: FormEvent) {
    event.preventDefault();
    setSubmitted(query.trim());
  }
  return (
    <>
      <h1>作品一覧</h1>
      <section
        className="project-description"
        aria-labelledby="project-description-title"
      >
        <h2 id="project-description-title">このプロジェクトについて</h2>
        <p>
          このプロジェクトはUTAU、MMD、YMMなどの制作に用いるアセットを保全するためのプロジェクトです。
        </p>
        <p>
          ゲームに関するアセットについては、受付はいたしますが対応の優先順位が低く対応されにくいことをご了承ください。
        </p>
      </section>
      <section className="public-service-cards" aria-label="再配布支援サービス">
        <Link to="/redistribution-support" className="public-service-card">
          <span className="public-service-label">ファイルを持っている方</span>
          <b>再配布の相談・権利者への問い合わせ代行</b>
          <span>ファイルを送って、権利確認を依頼する →</span>
        </Link>
        <Link to="/publisher-support" className="public-service-card">
          <span className="public-service-label">配布元・権利者の方</span>
          <b>配布・再配布の代行を相談する</b>
          <span>配布先探しや管理の手間を相談する →</span>
        </Link>
      </section>
      <p className="public-intro">公開が許可されたファイルを掲載しています。</p>
      <form className="public-search" onSubmit={search}>
        <input
          aria-label="作品を検索"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="作品・作者・ファイル名・uploader.jpのURL"
        />
        <button type="submit">検索</button>
      </form>
      {loading ? (
        <p className="public-muted">読み込み中…</p>
      ) : error ? (
        <p className="public-error">{error}</p>
      ) : items.length === 0 ? (
        <p className="public-muted">公開中の作品はありません。</p>
      ) : (
        <div className="public-list">
          {items.map((item) => (
            <article className="public-list-item" key={item.id}>
              <div>
                <h2>
                  <Link to={`/artifacts/${item.id}`}>{item.title}</Link>
                </h2>
                <p className="public-byline">
                  {item.creator_name || "作者不明"} · ファイル{" "}
                  {item.files.length} 件
                </p>
                {item.description && (
                  <p className="public-description">{item.description}</p>
                )}
              </div>
              <Link className="public-text-link" to={`/artifacts/${item.id}`}>
                詳細 →
              </Link>
            </article>
          ))}
        </div>
      )}
    </>
  );
}

function PublicArtifactDetail() {
  const { id = "" } = useParams();
  const [item, setItem] = useState<PublicArtifact | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    setLoading(true);
    publicAPI<{ item: PublicArtifact }>(
      `/public/artifacts/${encodeURIComponent(id)}`,
    )
      .then((data) => {
        if (active) {
          setItem(data.item);
          setError("");
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id]);
  if (loading) return <p className="public-muted">読み込み中…</p>;
  if (error || !item)
    return (
      <>
        <p className="public-error">{error || "作品が見つかりません。"}</p>
        <Link className="public-text-link" to="/">
          ← 作品一覧
        </Link>
      </>
    );
  return (
    <>
      <Link className="public-text-link" to="/">
        ← 作品一覧
      </Link>
      <h1 className="public-detail-title">{item.title}</h1>
      <dl className="public-metadata">
        <dt>作者</dt>
        <dd>{item.creator_name || "不明"}</dd>
        <dt>元ページ</dt>
        <dd>
          <a href={item.source_url} target="_blank" rel="noreferrer">
            元ページを開く ↗
          </a>
        </dd>
      </dl>
      {item.description && (
        <p className="public-detail-description">{item.description}</p>
      )}
      <h2 className="public-section-title">ファイル</h2>
      {item.files.length === 0 ? (
        <p className="public-muted">公開できるファイルはありません。</p>
      ) : (
        <div className="public-file-list">
          {item.files.map((file) => (
            <PublicFileRow key={file.id} file={file} artifactId={item.id} />
          ))}
        </div>
      )}
      <p className="public-footnote">
        ダウンロードできるのは、公開条件と保存状態を確認できたファイルだけです。
      </p>
    </>
  );
}

function PublicFileRow({
  file,
  artifactId,
}: {
  file: PublicFile;
  artifactId: string;
}) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  async function download() {
    setBusy(true);
    setMessage("準備中…");
    try {
      const created = await publicAPI<{
        job: { id: string };
        access_token: string;
      }>(`/public/files/${file.id}/downloads`, { method: "POST" });
      let ready = false;
      for (let attempt = 0; attempt < 90; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 2000));
        const status = await publicAPI<{ status: string; last_error?: string }>(
          `/public/downloads/${created.job.id}`,
          {
            headers: {
              "Download-Authorization": `Bearer ${created.access_token}`,
            },
          },
        );
        if (status.status === "ready") {
          ready = true;
          break;
        }
        if (status.status === "failed" || status.status === "expired")
          throw new Error(
            "ダウンロードを準備できませんでした。時間をおいて再度お試しください。",
          );
        setMessage("準備中…");
      }
      if (!ready)
        throw new Error(
          "準備に時間がかかっています。時間をおいて再度お試しください。",
        );
      const response = await fetch(
        `/api/v1/public/downloads/${created.job.id}/content`,
        {
          headers: {
            "Download-Authorization": `Bearer ${created.access_token}`,
          },
        },
      );
      if (!response.ok) {
        let detail = "";
        try {
          detail = (await response.json()).error || "";
        } catch {
          /* no body */
        }
        throw new Error(detail || "ダウンロードに失敗しました。");
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = file.original_filename;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setMessage("ダウンロードを開始しました");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "エラーが発生しました");
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="public-file-row">
      <div className="public-file-main">
        <b>{file.original_filename}</b>
        <span>{formatBytes(file.size_bytes)}</span>
        {message && (
          <small className="public-download-message">{message}</small>
        )}
      </div>
      <div className="public-file-actions">
        <button type="button" disabled={busy} onClick={download}>
          {busy ? "準備中…" : "ダウンロード"}
        </button>
        <Link
          to={`/requests?artifact_id=${encodeURIComponent(artifactId)}&file_id=${encodeURIComponent(file.id)}`}
        >
          公開停止等
        </Link>
        <Link
          to={`/relocation-info?artifact_id=${encodeURIComponent(artifactId)}&file_id=${encodeURIComponent(file.id)}`}
        >
          移設先情報
        </Link>
      </div>
    </div>
  );
}

function PublicRequest() {
  const [params] = useSearchParams();
  const artifactID = params.get("artifact_id") || "";
  const fileID = params.get("file_id") || "";
  const [type, setType] = useState("takedown");
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [details, setDetails] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [submittedID, setSubmittedID] = useState("");
  const [turnstileReset, setTurnstileReset] = useState(0);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const turnstileToken = String(
      new FormData(event.currentTarget).get("cf-turnstile-response") || "",
    );
    const body: Record<string, string> = {
      request_type: type,
      details: details.trim(),
      turnstile_token: turnstileToken,
    };
    if (artifactID) body.artifact_id = artifactID;
    if (fileID) body.file_object_id = fileID;
    if (name.trim()) body.submitter_name = name.trim();
    if (contact.trim()) body.contact = contact.trim();
    try {
      const data = await publicAPI<{ id: string }>("/public-requests", {
        method: "POST",
        body: JSON.stringify(body),
      });
      setSubmittedID(data.id);
      setDetails("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "送信できませんでした。");
    } finally {
      setBusy(false);
      setTurnstileReset((value) => value + 1);
    }
  }
  return (
    <>
      <h1>申請</h1>
      <p className="public-intro">
        公開停止や削除の申請はこちら。移設先URLの情報提供は専用ページから送信できます。
      </p>
      {submittedID ? (
        <div className="public-success">
          <p>申請を受け付けました。</p>
          <p className="public-muted">
            受付番号: <code>{submittedID}</code>
          </p>
          <button type="button" onClick={() => setSubmittedID("")}>
            続けて申請
          </button>
        </div>
      ) : (
        <form className="public-request-form" onSubmit={submit}>
          {artifactID && (
            <input type="hidden" name="artifact_id" value={artifactID} />
          )}
          {fileID && (
            <input type="hidden" name="file_object_id" value={fileID} />
          )}
          {(artifactID || fileID) && (
            <p className="public-target">
              対象: {fileID ? "指定したファイル" : "指定した作品"}
            </p>
          )}
          <label>
            申請の種類
            <select value={type} onChange={(e) => setType(e.target.value)}>
              <option value="takedown">公開停止</option>
              <option value="deletion">削除</option>
            </select>
          </label>
          <label>
            内容
            <textarea
              required
              minLength={5}
              maxLength={10000}
              rows={5}
              value={details}
              onChange={(e) => setDetails(e.target.value)}
              placeholder="申請の理由や補足"
            />
          </label>
          <label>
            お名前（任意）
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={255}
            />
          </label>
          <label>
            連絡先（任意）
            <input
              value={contact}
              onChange={(e) => setContact(e.target.value)}
              maxLength={500}
              placeholder="メールアドレスなど"
            />
          </label>
          <TurnstileField key={turnstileReset} />
          {error && <p className="public-error">{error}</p>}
          <button type="submit" disabled={busy}>
            {busy ? "送信中…" : "申請する"}
          </button>
        </form>
      )}
    </>
  );
}

function RelocationInfoRequest() {
  const [params] = useSearchParams();
  const artifactID = params.get("artifact_id") || "";
  const fileID = params.get("file_id") || "";
  const [url, setURL] = useState("");
  const [details, setDetails] = useState("");
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [submittedID, setSubmittedID] = useState("");
  const [turnstileReset, setTurnstileReset] = useState(0);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const turnstileToken = String(
      new FormData(event.currentTarget).get("cf-turnstile-response") || "",
    );
    const body: Record<string, string> = {
      request_type: "external_publication",
      external_url: url.trim(),
      details: details.trim() || "移設先情報を提供します。",
      turnstile_token: turnstileToken,
    };
    if (artifactID) body.artifact_id = artifactID;
    if (fileID) body.file_object_id = fileID;
    if (name.trim()) body.submitter_name = name.trim();
    if (contact.trim()) body.contact = contact.trim();
    try {
      const data = await publicAPI<{ id: string }>("/public-requests", {
        method: "POST",
        body: JSON.stringify(body),
      });
      setSubmittedID(data.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "送信できませんでした。");
    } finally {
      setBusy(false);
      setTurnstileReset((value) => value + 1);
    }
  }
  return (
    <>
      <h1>移設先情報の提供</h1>
      <p className="public-intro">
        保全対象のファイルが別の場所に移設・再公開されている場合、そのURLをお知らせください。情報は確認担当者が確認します。
      </p>
      {submittedID ? (
        <div className="public-success">
          <p>情報を受け付けました。</p>
          <p className="public-muted">
            受付番号: <code>{submittedID}</code>
          </p>
          <button
            type="button"
            onClick={() => {
              setSubmittedID("");
              setURL("");
              setDetails("");
            }}
          >
            続けて送信
          </button>
        </div>
      ) : (
        <form className="public-request-form" onSubmit={submit}>
          {(artifactID || fileID) && (
            <p className="public-target">
              対象: {fileID ? "指定したファイル" : "指定した作品"}
            </p>
          )}
          <label>
            移設先URL *
            <input
              required
              type="url"
              maxLength={4096}
              value={url}
              onChange={(e) => setURL(e.target.value)}
              placeholder="https://example.com/moved-file"
            />
          </label>
          <label>
            補足
            <textarea
              rows={4}
              maxLength={10000}
              value={details}
              onChange={(e) => setDetails(e.target.value)}
              placeholder="移設先の説明、確認できた日付など"
            />
          </label>
          <label>
            お名前（任意）
            <input
              maxLength={255}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            連絡先（任意）
            <input
              maxLength={500}
              value={contact}
              onChange={(e) => setContact(e.target.value)}
              placeholder="メールアドレスなど"
            />
          </label>
          <TurnstileField key={turnstileReset} />
          {error && <p className="public-error">{error}</p>}
          <button type="submit" disabled={busy}>
            {busy ? "送信中…" : "情報を送信する"}
          </button>
        </form>
      )}
    </>
  );
}

function RedistributionAssistancePage({
  mode,
}: {
  mode: "redistribution" | "publisher";
}) {
  const isPublisher = mode === "publisher";
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [submittedID, setSubmittedID] = useState("");
  const [submittedMessage, setSubmittedMessage] = useState("");
  const [turnstileReset, setTurnstileReset] = useState(0);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    form.set(
      "turnstile_token",
      String(form.get("cf-turnstile-response") || ""),
    );
    const file = form.get("file");
    if (file instanceof File && file.size > 100 * 1024 * 1024) {
      setError("ファイルは100 MiB以下にしてください。");
      setBusy(false);
      return;
    }
    try {
      const response = await fetch("/api/v1/public/redistribution-assistance", {
        method: "POST",
        body: form,
      });
      const text = await response.text();
      let data: any = {};
      try {
        data = text ? JSON.parse(text) : {};
      } catch {
        data = { message: text };
      }
      if (!response.ok)
        throw new Error(
          data.error || data.message || `HTTP ${response.status}`,
        );
      setSubmittedID(data.id || "");
      setSubmittedMessage(data.message || "申請を受け付けました。");
      formElement.reset();
    } catch (e) {
      setError(e instanceof Error ? e.message : "送信できませんでした。");
    } finally {
      setBusy(false);
      setTurnstileReset((value) => value + 1);
    }
  }
  return (
    <>
      <Link className="public-text-link" to="/">
        ← 作品一覧へ戻る
      </Link>
      <h1>
        {isPublisher
          ? "配布元・権利者向け：配布代行の相談"
          : "手元にあるファイルの再配布相談"}
      </h1>
      {isPublisher ? (
        <p className="public-intro">
          配布先を探している、ファイルの置き場所や再配布対応が負担になっている配布元・権利者の方へ。ファイルまたは配布元URLを送っていただければ、保全・配布方法を相談し、対応可能な範囲で代行します。
        </p>
      ) : (
        <p className="public-intro">
          uploader.jpにあったファイルを手元に持っていて、「良い作品なので再配布したいけれど、権利者への問い合わせが面倒」という方向けの窓口です。ファイルを送っていただければ、運営が権利者への問い合わせを試み、許諾が確認できた場合に限って再配布を検討します。
        </p>
      )}
      <section className="public-process">
        <h2>利用の流れ</h2>
        <ol>
          <li>フォームからファイルや作品情報を送る</li>
          <li>運営が内容を確認し、必要に応じて権利者・配布元へ問い合わせる</li>
          <li>
            許諾や配布条件を確認できた場合に、保全・再配布の可否を相談する
          </li>
        </ol>
        <p>
          送信しただけで公開・再配布されることはありません。権利者と連絡が取れない場合や許諾が得られない場合は、公開・再配布できないことがあります。
        </p>
      </section>
      {submittedID ? (
        <div className="public-success">
          <h2>相談を受け付けました</h2>
          <p>{submittedMessage}</p>
          <p className="public-muted">
            受付番号：<code>{submittedID}</code>
          </p>
          <p>運営が内容を確認し、必要に応じて入力された連絡先へ連絡します。</p>
          <button
            type="button"
            onClick={() => {
              setSubmittedID("");
              setSubmittedMessage("");
            }}
          >
            別の相談を送る
          </button>
        </div>
      ) : (
        <form
          className="public-request-form"
          onSubmit={submit}
          encType="multipart/form-data"
        >
          <input
            type="hidden"
            name="request_type"
            value={
              isPublisher ? "publisher_assistance" : "redistribution_assistance"
            }
          />
          <label>
            作品名・ファイルの名前 *
            <input
              name="title"
              required
              maxLength={500}
              placeholder="分かる範囲の作品名やファイル名"
            />
          </label>
          <label>
            作者・配布元
            <input
              name="creator_name"
              maxLength={255}
              placeholder="分かる場合"
            />
          </label>
          <label>
            {isPublisher
              ? "現在の配布ページURL（任意）"
              : "元の配布ページURL（任意）"}
            <input
              name="source_url"
              type="url"
              maxLength={4096}
              placeholder="https://..."
            />
            <small>
              {isPublisher
                ? "ファイルを添付しない場合は、配布元URLが必要です。"
                : "分かれば入力してください。"}
            </small>
          </label>
          <label>
            権利者への連絡先・関連情報（任意）
            <input
              name="rights_holder_contact"
              maxLength={1000}
              placeholder="公式サイト、連絡先、SNSなど"
            />
          </label>
          <label>
            {isPublisher ? "ファイルを添付（任意）" : "手元にあるファイル *"}
            <input name="file" type="file" required={!isPublisher} />{" "}
            <small>
              1ファイル100
              MiBまで。送信ファイルは権利確認のための一時保管であり、一般公開されません。
            </small>
          </label>
          <label>
            相談内容 *
            <textarea
              name="details"
              rows={4}
              required
              minLength={5}
              maxLength={10000}
              placeholder={
                isPublisher
                  ? "配布代行を希望する理由、希望する配布方法、条件など"
                  : "ファイルの由来、再配布したい理由、分かっている権利者情報など"
              }
            />
          </label>
          <label>
            お名前・活動名（任意）
            <input name="submitter_name" maxLength={255} />
            <small>
              権利者ではなく、ご連絡者様のお名前・活動名をご記入ください。
            </small>
          </label>
          <label>
            連絡先 *
            <input
              name="contact"
              required
              maxLength={500}
              placeholder="返信を受け取れるメールアドレスなど"
            />
          </label>
          <label className="check-label">
            <input type="checkbox" name="agree_to_terms" required />{" "}
            <span>
              送信ファイルを権利確認のために運営が一時保管・確認すること、権利者の許諾が確認できるまで公開・再配布しないことに同意します。
            </span>
          </label>
          <TurnstileField key={turnstileReset} />
          {error && <p className="public-error">{error}</p>}
          <button type="submit" disabled={busy}>
            {busy ? "送信中…" : "相談を送信する"}
          </button>
          <p className="public-footnote">
            第三者の権利を侵害する目的でファイルを送信しないでください。送信は再配布の許諾を意味せず、運営が再配布を保証するものでもありません。アップロード時の自動ウイルス検査は行いません。連絡先などの情報は、申請への対応に必要な範囲で取り扱います。
          </p>
        </form>
      )}
    </>
  );
}

function PreservationRequest() {
  const [title, setTitle] = useState("");
  const [creator, setCreator] = useState("");
  const [sourceURL, setSourceURL] = useState("");
  const [sourcePassword, setSourcePassword] = useState("");
  const [details, setDetails] = useState("");
  const [name, setName] = useState("");
  const [contact, setContact] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [submittedID, setSubmittedID] = useState("");
  const [submittedMessage, setSubmittedMessage] = useState("");
  const [turnstileReset, setTurnstileReset] = useState(0);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const turnstileToken = String(
      new FormData(event.currentTarget).get("cf-turnstile-response") || "",
    );
    try {
      const data = await publicAPI<{
        id?: string;
        duplicate?: boolean;
        message?: string;
      }>("/public-requests", {
        method: "POST",
        body: JSON.stringify({
          turnstile_token: turnstileToken,
          request_type: "preservation",
          title: title.trim(),
          creator_name: creator.trim(),
          source_url: sourceURL.trim(),
          source_password: sourcePassword,
          details: details.trim() || "詳細なし",
          submitter_name: name.trim(),
          contact: contact.trim(),
        }),
      });
      setSubmittedID(data.id || (data.duplicate ? "duplicate" : ""));
      setSubmittedMessage(
        data.message ||
          (data.duplicate
            ? "このURLはすでに登録・申請済みのため、重複リクエストを破棄しました。"
            : "リクエストを受け付けました。"),
      );
      setSourcePassword("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "送信できませんでした。");
    } finally {
      setBusy(false);
      setTurnstileReset((value) => value + 1);
    }
  }
  return (
    <>
      <h1>保全リクエスト</h1>
      <p className="public-intro">
        残してほしいファイル、またはアップローダーのURLを送ってください。
      </p>
      {submittedID ? (
        <div className="public-success">
          <p>{submittedMessage}</p>
          {submittedID !== "duplicate" && (
            <p className="public-muted">
              受付番号: <code>{submittedID}</code>
            </p>
          )}
          <p className="public-muted">内容を確認してから保全を検討します。</p>
          <button
            type="button"
            onClick={() => {
              setSubmittedID("");
              setSubmittedMessage("");
              setTitle("");
              setCreator("");
              setSourceURL("");
              setSourcePassword("");
              setDetails("");
            }}
          >
            続けて送信
          </button>
        </div>
      ) : (
        <form className="public-request-form" onSubmit={submit}>
          <label>
            作品名 *
            <input
              required
              maxLength={500}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="作品名"
            />
          </label>
          <label>
            作者・配布者
            <input
              maxLength={255}
              value={creator}
              onChange={(e) => setCreator(e.target.value)}
              placeholder="分かる場合"
            />
          </label>
          <label>
            uploader.jp のURL *
            <input
              required
              type="url"
              maxLength={4096}
              value={sourceURL}
              onChange={(e) => setSourceURL(e.target.value)}
              placeholder="https://uu.getuploader.com/名前/ または /download/123"
            />
            <small>
              個別ファイルのURLでも、アップローダーの一覧ページURLでも送れます。
            </small>
          </label>
          <label>
            ダウンロードパスワード（任意）
            <input
              type="password"
              maxLength={1024}
              autoComplete="off"
              value={sourcePassword}
              onChange={(e) => setSourcePassword(e.target.value)}
              placeholder="パスワードが設定されている場合"
            />
            <small>
              暗号化して保管し、保全時の正規のパスワード入力に使用します。取得担当のノード運営者がファイル内容を扱える点にご注意ください。CAPTCHA等の追加認証は自動処理しません。
            </small>
          </label>
          <label>
            補足
            <textarea
              rows={4}
              maxLength={10000}
              value={details}
              onChange={(e) => setDetails(e.target.value)}
              placeholder="作品情報や保全を希望する理由"
            />
          </label>
          <label>
            お名前（任意）
            <input
              maxLength={255}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            連絡先（任意）
            <input
              maxLength={500}
              value={contact}
              onChange={(e) => setContact(e.target.value)}
              placeholder="メールアドレスなど"
            />
          </label>
          <p className="public-footnote">
            一覧URLの場合は、管理者が掲載ファイルを確認してから順番に保全します。送信だけで自動取得や一般公開は始まりません。
          </p>
          <TurnstileField key={turnstileReset} />
          {error && <p className="public-error">{error}</p>}
          <button type="submit" disabled={busy}>
            {busy ? "送信中…" : "保全を依頼する"}
          </button>
        </form>
      )}
    </>
  );
}
