import { AlertTriangle, ListChecks, Mail } from "lucide-react";
import type { Obj } from "@/types/common";

type StepState = "done" | "current" | "todo" | "skipped";

function stepClass(state: StepState) {
	return `assist-step ${state}`;
}

export function ReviewAssist({
	request,
	files,
	filesLoading,
	artifactID,
	navigate,
}: {
	request: Obj;
	files: Obj[];
	filesLoading: boolean;
	artifactID: string;
	navigate: (path: string) => void;
}) {
	const verified = files.filter(
		(file) => file.preservation_status === "verified",
	);
	const unfinished = files.filter(
		(file) =>
			!file.preservation_status || file.preservation_status !== "verified",
	);
	const unchecked = files.filter(
		(file) => (file.redistribution_policy || "unchecked") === "unchecked",
	);
	const allowed = files.filter(
		(file) => file.redistribution_policy === "allowed",
	);
	const restricted = files.filter(
		(file) => file.redistribution_policy === "restricted",
	);
	const unknown = files.filter(
		(file) => file.redistribution_policy === "unknown",
	);

	const started = files.length > 0;
	const preservationComplete = started && unfinished.length === 0;
	const reviewComplete = preservationComplete && unchecked.length === 0;
	const hasAllowed = allowed.length > 0;
	const hasRestricted = restricted.length > 0;

	const mailTemplate =
		hasAllowed && hasRestricted
			? "報告と許諾確認および移設のご提案"
			: hasAllowed
				? "再掲載の報告"
				: hasRestricted
					? "許諾確認および移設のご提案"
					: "";

	let currentStep = 1;
	if (request.status === "accepted") currentStep = 4;
	else if (request.status === "rejected") currentStep = 5;
	else if (request.status === "closed") currentStep = 0;
	else if (reviewComplete && request.status === "in_review") currentStep = 3;
	else if (preservationComplete) currentStep = 2;

	const stateFor = (step: number): StepState => {
		if (currentStep === 0) return "skipped";
		if (step < currentStep) return "done";
		if (step === currentStep) return "current";
		return "todo";
	};

	if (request.status === "closed") return null;

	const openFiles = () => {
		if (!artifactID) return;
		navigate(
			`/admin/review?tab=files&artifact_id=${encodeURIComponent(artifactID)}`,
		);
	};

	return (
		<section className="panel review-assist">
			<div className="panel-head">
				<div>
					<h2>
						<ListChecks size={16} /> レビューアシスト
					</h2>
					<p>
						申請の状態から、次に行う操作を案内します。メール送信は LibreDesk で{" "}
						<kbd>Ctrl</kbd>+<kbd>M</kbd>{" "}
						を押し、案内されたテンプレートを選択してください。最初の連絡は
						uploader.jp のメールフォームを使っても構いません。
					</p>
				</div>
				{artifactID && (
					<button type="button" className="secondary" onClick={openFiles}>
						ファイル一覧を開く
					</button>
				)}
			</div>

			<div className="assist-placeholder-warning" role="alert">
				<AlertTriangle size={16} />
				<div>
					<b>
						「[このようにカッコで囲まれている文字列]は必ず置き換えてください」
					</b>
					<span>
						テンプレート内の [お名前] や [作品名]
						などは、送信前に実際の内容へ置き換えます。カッコごと残さないでください。
					</span>
				</div>
			</div>

			<ol className="assist-steps">
				<li className={stepClass(stateFor(1))}>
					<div className="assist-step-number">1</div>
					<div className="assist-step-body">
						<b>保全処理（ダウンロード）を開始する</b>
						<p>
							右の「保全を実行」で利用規約を確認し、
							<strong>「URLを調査・取得タスク登録」</strong>
							を押します。一覧ページの場合は、候補を選んで取得タスクを登録します。
						</p>
						{started && (
							<small>
								検証済み {verified.length} 件 / 保全未完了 {unfinished.length}{" "}
								件{filesLoading ? "（読み込み中）" : ""}
							</small>
						)}
					</div>
				</li>

				<li className={stepClass(stateFor(2))}>
					<div className="assist-step-number">2</div>
					<div className="assist-step-body">
						<b>ダウンロードしたファイルを確認する</b>
						<p>
							申請を「確認中」にしたうえで、関連ファイルを一つずつダウンロードし、
							README などから再配布条件を確認します。確認後、「ファイル操作」で
							再配布ポリシーを<strong>「制限付き」「再配布可」「不明」</strong>
							のいずれかに設定します。「未確認」が残っている間は次の手順に進みません。
						</p>
						<div className="assist-counts">
							<span>未確認 {unchecked.length}</span>
							<span>再配布可 {allowed.length}</span>
							<span>制限付き {restricted.length}</span>
							<span>不明 {unknown.length}</span>
						</div>
					</div>
				</li>

				<li className={stepClass(stateFor(3))}>
					<div className="assist-step-number">3</div>
					<div className="assist-step-body">
						<b>メールを送り、公開と承認を判断する</b>
						{!preservationComplete ? (
							<p>すべてのファイルの保全が完了すると、この手順に進みます。</p>
						) : unchecked.length > 0 ? (
							<p>
								未確認のファイルが {unchecked.length}{" "}
								件残っています。先に手順2を完了してください。
							</p>
						) : mailTemplate ? (
							<>
								<p>
									LibreDesk で <kbd>Ctrl</kbd>+<kbd>M</kbd> を押し、テンプレート
									<strong>「{mailTemplate}」</strong>を選択して送信します。
								</p>
								<ul>
									{hasAllowed && (
										<li>
											再配布可のファイルは、「ファイル操作」→「公開設定」で
											<strong>一般公開をON</strong>
											にします（承認前に公開しないでください）。
										</li>
									)}
									{hasRestricted && (
										<li>
											制限付きのファイルは、返信で許諾が取れた場合のみ公開フラグをONにし、
											申請を<strong>「承認」</strong>
											にします。許諾が取れなければ
											<strong>「却下」</strong>にします。
										</li>
									)}
									{!hasRestricted && (
										<li>
											返信を待たずに申請を<strong>「承認」</strong>にします。
										</li>
									)}
								</ul>
							</>
						) : (
							<p>
								再配布可・制限付きのファイルがありません。公開できるファイルがないため、
								却下または保留を検討してください。
							</p>
						)}
					</div>
				</li>

				{(currentStep === 4 || request.status === "accepted") && (
					<li className={stepClass(stateFor(4))}>
						<div className="assist-step-number">4</div>
						<div className="assist-step-body">
							<b>許諾へのお礼と認証情報を送る</b>
							{hasRestricted ? (
								<>
									<p>
										LibreDesk で <kbd>Ctrl</kbd>+<kbd>M</kbd>{" "}
										を押し、テンプレート
										<strong>「許諾のお礼と認証情報通知」</strong>
										を選択して、アカウント情報を送付します。
									</p>
									<p className="muted">
										初期パスワードはこの画面で一度だけ表示されます。送信前に控えてください。
									</p>
								</>
							) : (
								<p>
									README
									などで再配布可が確認できているため、許諾メールは不要です。
									「再掲載の報告」を送ったか確認してください。
								</p>
							)}
						</div>
					</li>
				)}

				{(currentStep === 5 || request.status === "rejected") && (
					<li className={stepClass(stateFor(5))}>
						<div className="assist-step-number">5</div>
						<div className="assist-step-body">
							<b>許諾が取れなかった場合の連絡</b>
							<p>
								LibreDesk で <kbd>Ctrl</kbd>+<kbd>M</kbd> を押し、テンプレート
								<strong>「許諾が取れなかった時」</strong>
								を選択して、対応へのお礼メールを送ります。
							</p>
						</div>
					</li>
				)}
			</ol>

			<div className="assist-footer">
				<Mail size={14} />
				<span>
					メールの宛先・文面は LibreDesk
					のテンプレートで確認してください。公開フラグは
					「再配布可」のファイルだけに設定します。
				</span>
			</div>
		</section>
	);
}
