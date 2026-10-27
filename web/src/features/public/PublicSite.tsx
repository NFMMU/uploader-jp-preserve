import { Link, Navigate, Route, Routes } from "react-router-dom";
import CopyEmail from "../../CopyEmail";
import { PublicHeader } from "../../components/layout/PublicHeader";
import { DisclaimerPage, PrivacyPage, TermsPage } from "./legal/LegalPages";
import { NodeParticipationRequest } from "./pages/NodeParticipationRequest";
import { PreservationRequest } from "./pages/PreservationRequest";
import { PublicArtifactDetail } from "./pages/PublicArtifactDetail";
import { PublicCatalog } from "./pages/PublicCatalog";
import { PublicRequest } from "./pages/PublicRequest";
import { RedistributionAssistancePage } from "./pages/RedistributionAssistancePage";
import { RelocationInfoRequest } from "./pages/RelocationInfoRequest";

function PublicSite() {
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

export default PublicSite;
