import { KeyRound, Link2, Plus } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { Heading } from "@/components/common/Heading";
import { Modal } from "@/components/common/Modal";
import { useData } from "@/hooks/useData";
import type { Obj } from "@/types/common";

export function PreferencePage({
	api,
	refresh,
	notify,
	role,
}: {
	api: (path: string, opts?: RequestInit) => Promise<Obj>;
	refresh: number;
	notify: (s: string) => void;
	role: any;
}) {
	const [localRefresh, setLocalRefresh] = useState(0);
	const d = useData(api, "/auth/oidc/providers", refresh + localRefresh);

	const baseURL = `${window.location.protocol}://${window.location.host}`;

	// 基本情報
	const [profile, setProfile] = useState({
		display_name: "",
		email: "",
	});

	// パスワード
	const [password, setPassword] = useState({
		current_password: "",
		new_password: "",
		confirm_password: "",
	});

	// 外部アカウント連携
	const [modal, setModal] = useState(false);
	const [provider, setProvider] = useState<string | undefined | null>();

	async function updateProfile(e: React.FormEvent) {
		e.preventDefault();
	}

	async function updatePassword(e: React.FormEvent) {
		e.preventDefault();
	}

	async function linkAccount(e: React.FormEvent) {
		e.preventDefault();
		window.location.href = `${baseURL}/api/v1/auth/oidc/${provider}?mode=link`;
	}

	async function unlinkAccount(id: string) {}

	return (
		<>
			<Heading eyebrow="" title="個人設定" desc="アカウントの各種設定。" />

			{/* 基本情報 */}
			<section>
				<Heading
					title="基本設定"
					desc="アカウントの基本情報を管理します。"
					level="h2"
				/>

				<form className="form-stack" onSubmit={updateProfile}>
					<label>
						表示名
						<input
							required
							value={profile.display_name}
							onChange={(e) =>
								setProfile({ ...profile, display_name: e.target.value })
							}
							placeholder="表示名を入力"
						/>
					</label>

					<label>
						メールアドレス
						<input
							required
							type="email"
							value={profile.email}
							onChange={(e) =>
								setProfile({ ...profile, email: e.target.value })
							}
							placeholder="example@example.com"
						/>
					</label>

					<div className="modal-actions">
						<button type="submit" className="primary">
							基本情報を保存
						</button>
					</div>
				</form>
			</section>

			{/* セキュリティ設定 */}
			<section>
				<Heading
					title="セキュリティ設定"
					desc="パスワードや外部アカウントの連携を管理します。"
					level="h2"
				/>

				{/* パスワード変更 */}
				<div className="preference-section">
					<div>
						<KeyRound size={20} />
						<div>
							<h3>パスワード変更</h3>
							<p>アカウントのパスワードを変更します。</p>
						</div>
					</div>

					<form className="form-stack" onSubmit={updatePassword}>
						<label>
							現在のパスワード
							<input
								required
								type="password"
								autoComplete="current-password"
								value={password.current_password}
								onChange={(e) =>
									setPassword({
										...password,
										current_password: e.target.value,
									})
								}
							/>
						</label>

						<label>
							新しいパスワード
							<input
								required
								type="password"
								autoComplete="new-password"
								value={password.new_password}
								onChange={(e) =>
									setPassword({
										...password,
										new_password: e.target.value,
									})
								}
							/>
						</label>

						<label>
							新しいパスワード（確認）
							<input
								required
								type="password"
								autoComplete="new-password"
								value={password.confirm_password}
								onChange={(e) =>
									setPassword({
										...password,
										confirm_password: e.target.value,
									})
								}
							/>
						</label>

						<div className="modal-actions">
							<button type="submit" className="primary">
								パスワードを変更
							</button>
						</div>
					</form>
				</div>

				{/* 外部アカウント連携 */}
				<div className="preference-section">
					<div>
						<Link2 size={20} />
						<div>
							<h3>外部アカウント連携</h3>
							<p>外部の認証プロバイダーとアカウントを連携します。</p>
						</div>
					</div>

					<div className="preference-provider-list">
						{/* TODO: 連携済みアカウントを表示 */}

						<div className="preference-empty">
							<p>連携されている外部アカウントはありません。</p>
						</div>
					</div>

					<div className="modal-actions">
						<button
							type="button"
							className="primary"
							onClick={() => setModal(true)}
						>
							<Plus size={16} />
							外部アカウントを追加
						</button>
					</div>
				</div>
			</section>

			{/* 外部アカウント連携モーダル */}
			{modal && (
				<Modal title="外部アカウントを連携" close={() => setModal(false)}>
					<form className="form-stack" onSubmit={linkAccount}>
						<label>
							連携先
							{d.loading ? (
								<div>ロード中...</div>
							) : d.error ? (
								<div>エラーが発生しました。</div>
							) : (
								<select
									required
									defaultValue=""
									onChange={(e) => {
										setProvider(e.target.value);
									}}
								>
									<option value="" disabled>
										連携先を選択
									</option>
									{(
										d.data.data as {
											id: string;
											service_icon?: string;
											display_name: string;
										}[]
									).map((p) => (
										<option key={p.id} value={p.id}>
											{p.display_name}
										</option>
									))}
								</select>
							)}
						</label>

						<div className="modal-actions">
							<button
								type="button"
								className="secondary"
								onClick={() => setModal(false)}
							>
								キャンセル
							</button>
							<button type="submit" className="primary">
								連携する
							</button>
						</div>
					</form>
				</Modal>
			)}
		</>
	);
}
