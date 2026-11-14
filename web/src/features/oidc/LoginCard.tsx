import { useData } from "@/hooks/useData";
import type { Obj } from "@/types/common";

export default function OidcLoginCard({
	api,
	notify,
	refresh,
	busy,
	setBusy,
}: {
	api: (path: string, opts?: RequestInit) => Promise<Obj>;
	notify: (s: string) => void;
	refresh: number;
	busy: boolean;
	setBusy: React.Dispatch<React.SetStateAction<boolean>>;
}) {
	const d = useData(api, "/auth/oidc/providers", refresh);
	const onLoginClick = (id: string) => {
		setBusy(true);
		const baseURL = `${window.location.protocol}://${window.location.host}`;
		window.location.href = `${baseURL}/api/v1/auth/oidc/${id}?mode=authenticate`;
	};
	return (
		<div>
			{d.loading && <span>ロード中...</span>}
			{d.error && <span>SSOの処理中にエラーが発生しました。</span>}
			{d.data && (
				<>
					{(
						d.data as {
							id: string;
							display_name: string;
							service_icon: string;
						}[]
					).map((p) => (
						<button
							className="secondary wide"
							disabled={busy}
							onClick={() => onLoginClick(p.id)}
						>
							{p.service_icon ?? (
								<img src={p.service_icon} alt={`${p.display_name}のアイコン`} />
							)}
							{p.display_name}でログイン
						</button>
					))}
				</>
			)}
		</div>
	);
}
