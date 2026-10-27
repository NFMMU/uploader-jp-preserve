import CopyEmail from "../../CopyEmail";
import { Heading } from "../../components/common/Heading";
import { Stat } from "../../components/common/Stat";
import { Status } from "../../components/common/Status";
import { Table } from "../../components/common/Table";
import { useData } from "../../hooks/useData";
import { bytes, date } from "../../lib/format";
import type { Obj } from "../../types/common";
import { NodeEnrollmentRequests } from "./NodeEnrollmentRequests";
import { AlertTriangle, Archive, CheckCircle2, HardDrive, RefreshCw } from "lucide-react";
import { useState } from "react";

export function Nodes({ api, refresh, notify, role }: any) {
  const [localRefresh, setLocalRefresh] = useState(0);
  const d = useData(api, "/nodes", refresh + localRefresh);
  const [deletingRows, setDeletingRows] = useState<string[]>([]);
  const deleteNode = async (id: string, name: string) => {
    if (!window.confirm(`本当に「${name}」を削除しますか？`)) return;
    setDeletingRows((rows) => [...rows, id]);
    try {
      await api(`/nodes/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
    } catch (e) {
      notify(e instanceof Error ? e.message : "エラーが発生しました");
    } finally {
      setDeletingRows((rows) => rows.filter((row) => row !== id));
      setLocalRefresh((x) => x + 1);
    }
  };
  return (
    <>
      <Heading
        eyebrow=""
        title="保存ノード"
        desc="稼働状況と容量"
        actions={
          <button
            className="secondary"
            onClick={() =>
              api("/admin/placements/reconcile", { method: "POST" })
                .then(() => notify("再調整を実行しました"))
                .catch((e: Error) => notify(e.message))
            }
          >
            <RefreshCw size={16} /> 保存先を調整
          </button>
        }
      />
      <div className="stat-grid">
        <Stat
          label="登録ノード"
          value={(d.data.items || []).length}
          sub="参加ノード数"
          icon={HardDrive}
        />
        <Stat
          label="オンライン"
          value={
            (d.data.items || []).filter((x: Obj) => x.status === "online")
              .length
          }
          sub="稼働状況"
          icon={CheckCircle2}
          tone="green"
        />
        <Stat
          label="申告容量"
          value={bytes(
            (d.data.items || []).reduce(
              (s: number, x: Obj) =>
                s +
                (x.requested_capacity_bytes === 0
                  ? x.capacity_bytes || 0
                  : x.requested_capacity_bytes || 0),
              0,
            ),
          )}
          sub="参加者の設定値の合計"
          icon={Archive}
          tone="violet"
        />
        <Stat
          label="要確認"
          value={
            (d.data.items || []).filter((x: Obj) => x.status !== "online")
              .length
          }
          sub="停止・劣化ノード"
          icon={AlertTriangle}
          tone="amber"
        />
      </div>
      <section className="panel table-panel">
        <div className="panel-head">
          <div>
            <h2>ノード一覧</h2>
            <p>実容量と申告上限を表示</p>
          </div>
        </div>
        <Table
          headers={[
            "ノード",
            "種別",
            "状態",
            "連絡先",
            "実容量 / 空き",
            "申告上限",
            "使用量",
            "最終応答",
            "エージェント",
            "操作",
          ]}
        >
          {d.loading ? (
            <tr>
              <td colSpan={10} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={10} className="empty">
                {d.error}
              </td>
            </tr>
          ) : (
            (d.data.items || []).map((x: Obj) => (
              <tr key={x.id}>
                <td>
                  <div className="file-cell">
                    <div className="file-icon">
                      <HardDrive size={17} />
                    </div>
                    <div>
                      <b>{x.name || x.id}</b>
                      <small>
                        <code>{String(x.id).slice(0, 8)}</code>
                      </small>
                    </div>
                  </div>
                </td>
                <td>
                  <span className="type-pill">{x.node_type || "—"}</span>
                </td>
                <td>
                  <Status value={x.status} />
                </td>
                <td>
                  {x.contact_email ? (
                    <>
                      <a href={`mailto:${x.contact_email}`}>
                        {x.contact_email}
                      </a>{" "}
                      <CopyEmail email={x.contact_email} />
                    </>
                  ) : (
                    "—"
                  )}
                </td>
                <td>
                  <b>
                    {bytes(
                      x.total_bytes ??
                        x.actual_capacity_bytes ??
                        x.capacity_bytes,
                    )}
                  </b>
                  <small className="cell-sub">
                    空き {bytes(x.available_bytes ?? x.free_bytes)}
                  </small>
                </td>
                <td>
                  {x.requested_capacity_bytes === 0
                    ? "無制限"
                    : bytes(x.requested_capacity_bytes ?? x.capacity_bytes)}
                </td>
                <td>
                  {x.used_bytes
                    ? bytes(x.used_bytes) +
                      `(${Math.trunc(
                        x.requested_capacity_bytes === 0
                          ? (x.used_bytes /
                              (x.total_bytes ??
                                x.actual_capacity_bytes ??
                                x.capacity_bytes)) *
                              100
                          : (x.used_bytes /
                              (x.requested_capacity_bytes ??
                                x.capacity_bytes)) *
                              100,
                      )}%)`
                    : "不明"}
                </td>
                <td>{date(x.last_seen_at)}</td>
                <td>{x.agent_version || "—"}</td>
                <td>
                  <div className="row-actions">
                    <button
                      className="small-btn"
                      disabled={
                        role === "viewer" || deletingRows.includes(x.id)
                      }
                      onClick={() => {
                        deleteNode(x.id, x.name || x.id);
                      }}
                    >
                      {deletingRows.includes(x.id) ? "処理中..." : "削除"}
                    </button>
                  </div>
                </td>
              </tr>
            ))
          )}
        </Table>
      </section>
      {role === "admin" && (
        <NodeEnrollmentRequests
          api={api}
          refresh={refresh + localRefresh}
          notify={notify}
        />
      )}
      <div className="info-card">
        <AlertTriangle size={18} />
        <div>
          <b>運用上の注意</b>
          <p>
            開発用ノードは同じホスト上です。本番では別ホストに配置してください。参加申請を承認すると72時間有効・一回限りの登録トークンが発行されます。
          </p>
        </div>
      </div>
    </>
  );
}
