import { KeyRound, Link2, Plus } from "lucide-react";
import type React from "react";
import { useEffect, useState } from "react";
import { Heading } from "@/components/common/Heading";
import { Modal } from "@/components/common/Modal";
import { useData } from "@/hooks/useData";
import type { Obj } from "@/types/common";

type CurrentUser = {
	id: string;
	email: string;
	display_name: string;
	role: "admin" | "reviewer" | "viewer";
	disabled: boolean;
};

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

	const baseURL = `${window.location.protocol}//${window.location.host}`;

	// 基本情報
	const [profile, setProfile] = useState({
		display_name: "",
	});

	// パスワード
	const [password, setPassword] = useState({
		current_password: "",
		new_password: "",
		confirm_password: "",
	});

	const [profileSaving, setProfileSaving] = useState(false);
	const [passwordSaving, setPasswordSaving] = useState(false);

	// 外部アカウント連携
	const [modal, setModal] = useState(false);
	const [provider, setProvider] = useState<string>("");

	const me = useData(api, "/me", refresh);

	const user =
		!me.loading && !me.error ? (me.data as CurrentUser | undefined) : undefined;

	useEffect(() => {
		if (!user) return;

		setProfile({
			display_name: user.display_name,
		});
	}, [user?.id, user?.display_name]);

	const identities = useData(
		api,
		user?.id ? `/users/${encodeURIComponent(user.id)}/oidc_identities` : "",
		refresh + localRefresh,
	);

	// 基本情報更新
	async function updateProfile(e: React.FormEvent) {
		e.preventDefault();

		if (!user || profileSaving) return;

		const displayName = profile.display_name.trim();

		if (!displayName) {
			notify("表示名を入力してください。");
			return;
		}

		if (displayName.length > 120) {
			notify("表示名は120文字以内で入力してください。");
			return;
		}

		setProfileSaving(true);

		try {
			await api(`/admin/users/${encodeURIComponent(user.id)}`, {
				method: "PUT",
				body: JSON.stringify({
					display_name: displayName,
				}),
			});

			setLocalRefresh((v) => v + 1);

			setProfile({
				display_name: displayName,
			});

			notify("基本情報を更新しました。");
		} catch (error) {
			notify(
				error instanceof Error
					? error.message
					: "基本情報の更新に失敗しました。",
			);
		} finally {
			setProfileSaving(false);
		}
	}

	// パスワード変更
	async function updatePassword(e: React.FormEvent) {
		e.preventDefault();

		if (!user || passwordSaving) return;

		const { current_password, new_password, confirm_password } = password;

		if (Array.from(new_password).length < 12) {
			notify("新しいパスワードは12文字以上にしてください。");
			return;
		}

		if (new TextEncoder().encode(new_password).length > 72) {
			notify("新しいパスワードは72バイト以内にしてください。");
			return;
		}

		if (new_password !== confirm_password) {
			notify("新しいパスワードと確認用パスワードが一致しません。");
			return;
		}

		if (current_password === new_password) {
			notify("現在のパスワードとは異なるパスワードを設定してください。");
			return;
		}

		setPasswordSaving(true);

		try {
			await api(`/admin/users/${encodeURIComponent(user.id)}/password`, {
				method: "PUT",
				body: JSON.stringify({
					current_password,
					new_password,
					confirm_password,
				}),
			});

			setPassword({
				current_password: "",
				new_password: "",
				confirm_password: "",
			});

			notify("パスワードを変更しました。");
		} catch (error) {
			notify(
				error instanceof Error
					? error.message
					: "パスワードの変更に失敗しました。",
			);
		} finally {
			setPasswordSaving(false);
		}
	}

	// 外部アカウント連携
	async function linkAccount(e: React.FormEvent) {
		e.preventDefault();

		if (!provider) {
			notify("連携先を選択してください。");
			return;
		}

		window.location.href = `${baseURL}/api/v1/auth/oidc/${encodeURIComponent(provider)}?mode=link`;
	}

	async function unlinkAccount(identityId: string) {
		try {
			await api(
				`/users/${encodeURIComponent(user!.id)}/oidc_identities/${encodeURIComponent(identityId)}`,
				{
					method: "DELETE",
				},
			);

			notify("外部アカウントの連携を解除しました。");
			setLocalRefresh((v) => v + 1);
		} catch (error) {
			notify(
				error instanceof Error ? error.message : "連携解除に失敗しました。",
			);
		}
	}

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

				{me.loading ? (
					<p>ユーザー情報を読み込んでいます...</p>
				) : me.error ? (
					<p>ユーザー情報を取得できませんでした。</p>
				) : user ? (
					<form className="form-stack" onSubmit={updateProfile}>
						<label>
							表示名
							<input
								required
								maxLength={120}
								value={profile.display_name}
								onChange={(e) =>
									setProfile({
										...profile,
										display_name: e.target.value,
									})
								}
								placeholder="表示名を入力"
							/>
						</label>

						<label>
							メールアドレス
							<input type="email" value={user.email} readOnly disabled />
							<small>メールアドレスの変更には対応していません。</small>
						</label>

						<div className="modal-actions">
							<button
								type="submit"
								className="primary"
								disabled={profileSaving}
							>
								{profileSaving ? "保存中..." : "基本情報を保存"}
							</button>
						</div>
					</form>
				) : (
					<p>ユーザー情報を取得できませんでした。</p>
				)}
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
								minLength={12}
								value={password.new_password}
								onChange={(e) =>
									setPassword({
										...password,
										new_password: e.target.value,
									})
								}
							/>
							<small>12文字以上、72バイト以内で入力してください。</small>
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
							<button
								type="submit"
								className="primary"
								disabled={passwordSaving || !user}
							>
								{passwordSaving ? "変更中..." : "パスワードを変更"}
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
						{identities.loading ? (
							<p>読み込み中...</p>
						) : identities.error ? (
							<p>連携アカウントの取得に失敗しました。</p>
						) : identities.data?.items?.length ? (
							identities.data.items.map(
								(identity: {
									id: string;
									provider_id: string;
									email: string;
									preferred_username?: string;
									picture?: string;
								}) => (
									<div key={identity.id} className="preference-provider">
										<div>
											<strong>
												{identity.preferred_username || identity.email}
											</strong>
											<p>{identity.email}</p>
										</div>

										<button
											type="button"
											className="secondary"
											onClick={() => unlinkAccount(identity.id)}
										>
											連携解除
										</button>
									</div>
								),
							)
						) : (
							<div className="preference-empty">
								<p>連携されている外部アカウントはありません。</p>
							</div>
						)}
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
									value={provider}
									onChange={(e) => setProvider(e.target.value)}
								>
									<option value="" disabled>
										連携先を選択
									</option>
									{(
										(d.data?.data ?? []) as {
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
							<button
								type="submit"
								className="primary"
								disabled={!provider || d.loading || !!d.error}
							>
								連携する
							</button>
						</div>
					</form>
				</Modal>
			)}
		</>
	);
}
