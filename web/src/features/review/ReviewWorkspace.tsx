import { Catalog } from "./Catalog";
import { Requests } from "./Requests";
import { ClipboardList, Files } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";

export function ReviewWorkspace({
  api,
  refresh,
  notify,
  role,
  token,
  bumpRefresh,
  initialTab = "files",
}: any) {
  const loc = useLocation();
  const nav = useNavigate();
  const queryTab = new URLSearchParams(loc.search).get("tab");
  const tab =
    queryTab === "requests" || (!queryTab && initialTab === "requests")
      ? "requests"
      : "files";
  function choose(next: "files" | "requests") {
    nav(`/admin/review?tab=${next}`);
  }
  return (
    <>
      <div
        className="review-workspace-tabs"
        role="tablist"
        aria-label="保全レビュー"
      >
        <button
          type="button"
          role="tab"
          aria-selected={tab === "files"}
          className={tab === "files" ? "active" : ""}
          onClick={() => choose("files")}
        >
          <Files size={16} /> 作品・ファイル
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "requests"}
          className={tab === "requests" ? "active" : ""}
          onClick={() => choose("requests")}
        >
          <ClipboardList size={16} /> 申請・保全レビュー
        </button>
      </div>
      {tab === "files" ? (
        <Catalog
          api={api}
          refresh={refresh}
          notify={notify}
          role={role}
          token={token}
        />
      ) : (
        <Requests
          api={api}
          refresh={refresh}
          notify={notify}
          role={role}
          bumpRefresh={bumpRefresh}
        />
      )}
    </>
  );
}
