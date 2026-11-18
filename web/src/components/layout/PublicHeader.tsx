import { Link } from "react-router-dom";

export function PublicHeader() {
	return (
		<header className="public-header">
			<div className="public-header-inner">
				<Link className="public-brand" to="/">
					uploader.jp保全プロジェクト
				</Link>
				<nav aria-label="メインナビゲーション">
					<Link to="/">作品一覧</Link>
					<Link to="/preservation-requests">保全リクエスト</Link>
					<Link to="/publisher-support">配布元向け代行</Link>
					<Link to="/requests">公開停止・削除</Link>
					<Link to="/relocation-info">移設先情報の提供</Link>
					<Link to="/notices">お知らせ</Link>
					<Link to="/admin/login" prefetch={"none"}>
						ログイン
					</Link>
				</nav>
			</div>
		</header>
	);
}
