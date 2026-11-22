import { Link } from "react-router-dom";
import { PageMeta } from "@/components/common/PageMeta";

export function PreservationFlowPage() {
	return (
		<>
			<PageMeta
				title="当プロジェクトにおける保全の流れ"
				description="保全リクエストの受付から、確認・保存・再配布条件の確認・公開までの流れを説明します。"
			/>
			<article className="public-article">
				<Link className="public-text-link" to="/">
					← 作品一覧へ戻る
				</Link>
				<h1>当プロジェクトにおける保全の流れ</h1>
				<p className="public-intro">
					保全リクエストを送っていただいてからファイルが公開されるまでの流れをまとめました。すべてのファイルが公開されるわけではなく、権利者の許諾を確認できたものだけを公開します。
				</p>

				<section className="public-process">
					<h2>かんたんに言うと</h2>
					<ol>
						<li>保全してほしいアップローダーのURLを送る</li>
						<li>運営が内容を確認し、ダウンロードして保存する</li>
						<li>READMEや配布元の条件を確認する</li>
						<li>再配布が許諾されているファイルだけを公開する</li>
						<li>著作権者には管理ダッシュボードの認証情報をお送りする</li>
					</ol>
					<p>
						保全（保存）と公開は別の判断です。保存しただけでは公開されません。
					</p>
				</section>

				<h2>保全の流れ（詳細）</h2>
				<ol className="public-flow-steps">
					<li className="public-flow-step">
						<h2>
							<span className="public-flow-step-number">1</span>
							保全してほしいアップローダーのリクエスト
						</h2>
						<p>
							保全してほしいuploader.jpのURLを添えて、リクエストを送信してください。個別ファイルのURLでも、アップローダーの一覧ページURLでも構いません。ダウンロードパスワードが設定されている場合は、フォームからお知らせください。
						</p>
						<p>
							リクエストを送信しただけでは、自動でダウンロードや公開が始まることはありません。
						</p>
						<Link className="public-text-link" to="/preservation-requests">
							保全リクエストを送る →
						</Link>
					</li>

					<li className="public-flow-step">
						<h2>
							<span className="public-flow-step-number">2</span>
							運営による内容の確認
						</h2>
						<p>
							対象が保全の対象か、すでに登録済みでないか、取得元の利用条件や取得可否に問題がないかを確認します。取得が難しい場合は、その理由を記録したうえで対応を見送ることがあります。
						</p>
					</li>

					<li className="public-flow-step">
						<h2>
							<span className="public-flow-step-number">3</span>
							ダウンロードと保存
						</h2>
						<p>
							運営がファイルをダウンロードし、複数の保存設備に保存します。保存後はハッシュ値を照合して、データが欠けたり壊れたりしていないかを確認します。この時点では、ファイルは一般公開されません。
						</p>
					</li>

					<li className="public-flow-step">
						<h2>
							<span className="public-flow-step-number">4</span>
							README・再配布条件の確認
						</h2>
						<p>
							保存したファイルのREADMEや配布元の説明を確認し、再配布してよいかを調べます。
						</p>
						<ul>
							<li>
								再配布が認められている場合（READMEに明記されている、ライセンスで許諾されている等）は、そのまま公開の手続きに進みます。
							</li>
							<li>
								再配布が制限されている場合、または条件が不明な場合は、メール等で著作権者に問い合わせます。
							</li>
						</ul>
					</li>

					<li className="public-flow-step">
						<h2>
							<span className="public-flow-step-number">5</span>
							公開
						</h2>
						<p>
							著作権者から許諾をいただいたファイル、または再配布が認められているファイルのみを公開します。作者名・取得元・利用条件などの情報もあわせて掲載します。許諾を確認できないファイルは公開しません。
						</p>
					</li>

					<li className="public-flow-step">
						<h2>
							<span className="public-flow-step-number">6</span>
							著作権者向けダッシュボードのご案内
						</h2>
						<p>
							公開後、許諾をいただいた著作権者には、著作権者向けダッシュボードの認証情報をお送りします。ダッシュボードでは、ファイルの差し替え、削除・非公開化、作品やファイルの説明の変更ができます。
						</p>
						<Link className="public-text-link" to="/copyright-holders">
							著作権者の方へ →
						</Link>
					</li>
				</ol>

				<h2>公開されない場合</h2>
				<p>
					次のような場合は、ファイルを公開しません。回答がないことを許諾とみなすことはありません。
				</p>
				<ul>
					<li>著作権者と連絡が取れない場合</li>
					<li>著作権者が公開・再配布を断っている場合</li>
					<li>再配布の条件が不明確な場合</li>
					<li>第三者の権利を侵害するおそれがある場合</li>
				</ul>
				<p>
					保全したデータの取り扱いは、取得元の利用条件や法令に従って個別に判断します。詳しくは
					<Link to="/preservation-policy">保全・再配布審査ポリシー</Link>
					をご覧ください。
				</p>

				<h2>公開後の変更・停止</h2>
				<p>
					著作権者の方はダッシュボードから、ファイルの非公開化や差し替えができます。どなたでも
					<Link to="/requests">公開停止・削除の申請</Link>
					から申し出ることもできます。削除は、保存ノードやバックアップを含むすべての保存先で確認してから完了となります。ノードがオフラインの場合など、完了まで時間がかかることがあります。
				</p>

				<h2>よくある質問</h2>
				<h3>リクエストすれば必ず公開されますか？</h3>
				<p>
					いいえ。リクエストは保全の依頼であり、公開を約束するものではありません。権利者の許諾または再配布を認めるライセンスを確認できた場合に限り公開します。
				</p>
				<h3>手元にあるファイルを送ることはできますか？</h3>
				<p>
					できます。配布元・権利者の方は
					<Link to="/publisher-support">配布・再配布の相談フォーム</Link>
					からファイルを添付して送信できます。
				</p>
				<h3>どれくらいで公開されますか？</h3>
				<p>
					リクエストが多く、著作権者との連絡にも時間がかかるため、目安をお伝えできません。許諾が確認できたものから順次公開します。
				</p>
				<h3>公開をやめてほしい場合は？</h3>
				<p>
					<Link to="/requests">公開停止・削除の申請</Link>
					またはメールでご連絡ください。著作権者の方はダッシュボードから非公開化することもできます。
				</p>
				<h3>保全されればデータは安心ですか？</h3>
				<p>
					複数の保存設備への保存を目指していますが、永続的な保存や復元可能性を保証するものではありません。重要なデータは、ご自身でも保管してください。
				</p>

				<h2>関連ページ</h2>
				<ul>
					<li>
						<Link to="/preservation-requests">保全リクエスト</Link>
					</li>
					<li>
						<Link to="/copyright-holders">著作権者の方へ</Link>
					</li>
					<li>
						<Link to="/preservation-policy">保全・再配布審査ポリシー</Link>
					</li>
					<li>
						<Link to="/requests">公開停止・削除の申請</Link>
					</li>
				</ul>
			</article>
		</>
	);
}
