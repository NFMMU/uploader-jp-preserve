import CopyEmail from "components/common/CopyEmail";
import { Link, Navigate, Route, Routes } from "react-router-dom";
import { PublicHeader } from "@/components/layout/PublicHeader";
import {
	DisclaimerPage,
	PreservationPolicyPage,
	PrivacyPage,
	TermsPage,
} from "./legal/LegalPages";
import { CopyrightHoldersPage } from "./pages/CopyrightHoldersPage";
import { NoticeDetail } from "./pages/NoticeDetail";
import { PreservationFlowPage } from "./pages/PreservationFlowPage";
import { PreservationRequest } from "./pages/PreservationRequest";
import { PublicArtifactDetail } from "./pages/PublicArtifactDetail";
import { PublicCatalog } from "./pages/PublicCatalog";
import { PublicFileDetail } from "./pages/PublicFileDetail";
import { PublicNotices } from "./pages/PublicNotices";
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
					<Route path="/notices" element={<PublicNotices />} />
					<Route path="/notices/:id" element={<NoticeDetail />} />
					<Route path="/notice" element={<Navigate to="/notices" replace />} />
					<Route path="/artifacts/:id" element={<PublicArtifactDetail />} />
					<Route path="/files/:id" element={<PublicFileDetail />} />
					<Route path="/requests" element={<PublicRequest />} />
					<Route
						path="/preservation-requests"
						element={<PreservationRequest />}
					/>
					<Route path="/preservation-flow" element={<PreservationFlowPage />} />
					<Route path="/copyright-holders" element={<CopyrightHoldersPage />} />
					<Route
						path="/publisher-support"
						element={<RedistributionAssistancePage mode="publisher" />}
					/>
					<Route path="/relocation-info" element={<RelocationInfoRequest />} />
					<Route path="/terms" element={<TermsPage />} />
					<Route path="/disclaimer" element={<DisclaimerPage />} />
					<Route path="/privacy" element={<PrivacyPage />} />
					<Route
						path="/preservation-policy"
						element={<PreservationPolicyPage />}
					/>
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
					<Link to="/notices">お知らせ</Link>
					<Link to="/preservation-flow">保全の流れ</Link>
					<Link to="/copyright-holders">著作権者の方へ</Link>
					<Link to="/terms">利用規約</Link>
					<Link to="/privacy">プライバシー・ポリシー</Link>
					<Link to="/disclaimer">免責事項</Link>
					<Link to="/preservation-policy">保全・再配布審査ポリシー</Link>
					<a href="mailto:uploaderjp-archive@udonya.moe">お問い合わせ</a>
					<CopyEmail email="uploaderjp-archive@udonya.moe" />
				</div>
			</footer>
		</div>
	);
}

export default PublicSite;
