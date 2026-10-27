import { downloadExport } from "../../api/admin";
import { Heading } from "../../components/common/Heading";
import { Stat } from "../../components/common/Stat";
import { TaskTable } from "../../components/common/TaskTable";
import { useData } from "../../hooks/useData";
import type { Obj } from "../../types/common";
import { AlertTriangle, Archive, ClipboardList, Download, Files, HardDrive, Plus, RefreshCw, ShieldCheck, Workflow } from "lucide-react";
import { NavLink } from "react-router-dom";

export function Dashboard({ api, refresh, notify, token }: any) {
  const a = useData(api, "/artifacts", refresh),
    f = useData(api, "/files", refresh),
    n = useData(api, "/nodes", refresh),
    t = useData(api, "/tasks", refresh),
    r = useData(api, "/public-requests?status=pending", refresh);
  const files = f.data.items || [],
    nodes = n.data.items || [],
    tasks = t.data.items || [];
  const degraded = files.filter((x: Obj) =>
    ["degraded", "missing", "error"].includes(x.preservation_status),
  ).length;
  const failed = tasks.filter((x: Obj) => x.status === "failed").length;
  const errors = [a.error, f.error, n.error, r.error].filter(Boolean);
  const filesUnavailable = f.loading || !!f.error;
  return (
    <>
      <Heading
        eyebrow=""
        title="保全状況"
        desc="保存状況と要対応項目"
        actions={
          <>
            <button
              className="secondary"
              onClick={() => downloadExport("json", token, notify)}
            >
              <Download size={16} /> 一覧を出力
            </button>
            <button
              className="primary"
              onClick={() =>
                api("/admin/placements/reconcile", { method: "POST" })
                  .then(() => notify("配置の再調整を実行しました"))
                  .catch((e: Error) => notify(e.message))
              }
            >
              <RefreshCw size={16} /> 保存先を調整
            </button>
          </>
        }
      />
      <div className="notice">
        <div className="notice-icon">
          <ShieldCheck size={19} />
        </div>
        <div>
          <b>保存と公開は別です</b>
          <p>権利未確認のファイルは、保存しても公開しません。</p>
        </div>
        <span className="notice-tag"></span>
      </div>
      {errors.length > 0 && (
        <div className="policy-warning" role="alert">
          <AlertTriangle size={16} /> {errors.join(" / ")}
        </div>
      )}
      <div className="stat-grid">
        <Stat
          label="作品数"
          value={
            a.loading || a.error
              ? "—"
              : a.data.total ?? (a.data.items || []).length
          }
          sub="登録済み"
          icon={Files}
        />
        <Stat
          label="保存ファイル"
          value={filesUnavailable ? "—" : files.length}
          sub="登録ファイル数"
          icon={Archive}
          tone="violet"
        />
        <Stat
          label="稼働ノード"
          value={
            n.loading || n.error
              ? "—"
              : `${nodes.filter((x: Obj) => x.status === "online").length} / ${nodes.length}`
          }
          sub="稼働状況"
          icon={HardDrive}
          tone="green"
        />
        <Stat
          label="要対応"
          value={t.loading || t.error ? "—" : failed}
          sub="失敗したタスク"
          icon={Workflow}
          tone={failed ? "red" : "amber"}
        />
      </div>
      <div className="content-grid">
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>保存状態</h2>
              <p>ファイルの状態</p>
            </div>
            <span className="live-chip">
              <span className="pulse" />{" "}
            </span>
          </div>
          <div className="health-list">
            {[
              ["verified", "検証済み", "green"],
              ["queued", "保存待ち", "blue"],
              ["degraded", "劣化・保存先不足", "amber"],
              ["missing", "実体不明", "red"],
              ["error", "エラー", "red"],
            ].map(([key, label, tone]) => {
              const count = files.filter(
                (x: Obj) => x.preservation_status === key,
              ).length;
              return (
                <div className="health-row" key={key}>
                  <span className={"health-dot " + tone} />
                  <span>{label}</span>
                  <b>{filesUnavailable ? "—" : count}</b>
                  <div className="mini-bar">
                    <i
                      className={tone}
                      style={{
                        width: `${!filesUnavailable && files.length ? (count / files.length) * 100 : 0}%`,
                      }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
          <div className="panel-foot">
            <span>
              要対応: <b>{filesUnavailable ? "—" : degraded}</b>
            </span>
            <NavLink to="/admin/review?tab=files">一覧を見る →</NavLink>
          </div>
        </section>
        <section className="panel">
          <div className="panel-head">
            <div>
              <h2>操作</h2>
              <p>よく使う機能</p>
            </div>
          </div>
          <div className="quick-actions">
            <NavLink to="/admin/review?tab=files" className="quick-card">
              <div className="quick-icon blue">
                <Plus />
              </div>
              <div>
                <b>作品を登録</b>
                <small>情報を登録して保存</small>
              </div>
              <span>→</span>
            </NavLink>
            <NavLink to="/admin/nodes" className="quick-card">
              <div className="quick-icon green">
                <HardDrive />
              </div>
              <div>
                <b>ノードを確認</b>
                <small>容量と稼働状況</small>
              </div>
              <span>→</span>
            </NavLink>
            <NavLink to="/admin/review?tab=requests" className="quick-card">
              <div className="quick-icon amber">
                <ClipboardList />
              </div>
              <div>
                <b>申請を確認</b>
                <small>
                  未処理 {r.loading || r.error
                    ? "—"
                    : r.data.total ?? (r.data.items || []).length} 件
                </small>
              </div>
              <span>→</span>
            </NavLink>
            <NavLink to="/admin/tasks" className="quick-card">
              <div className="quick-icon violet">
                <Workflow />
              </div>
              <div>
                <b>タスク</b>
                <small>保存・検証</small>
              </div>
              <span>→</span>
            </NavLink>
          </div>
        </section>
      </div>
      <section className="panel table-panel">
        <div className="panel-head">
          <div>
            <h2>最近のタスク</h2>
            <p>処理状況</p>
          </div>
          <NavLink to="/admin/tasks" className="text-link">
            一覧 →
          </NavLink>
        </div>
        <TaskTable
          items={tasks.slice(0, 6)}
          loading={t.loading}
          error={t.error}
        />
      </section>
    </>
  );
}
