import { Heading } from "../../components/common/Heading";
import { Status } from "../../components/common/Status";
import { Table } from "../../components/common/Table";
import { useData } from "../../hooks/useData";
import { date } from "../../lib/format";
import type { Obj } from "../../types/common";
import { Clock3, RefreshCw } from "lucide-react";
import { useState } from "react";

export function Tasks({ api, refresh, notify, role }: any) {
  const d = useData(api, "/tasks", refresh);
  const [filter, setFilter] = useState("all");
  const items = (d.data.items || []).filter(
    (x: Obj) => filter === "all" || x.status === filter,
  );
  async function retry(id: string) {
    try {
      await api(`/tasks/${id}/retry`, { method: "POST" });
      notify("再試行を開始しました");
    } catch (e) {
      notify((e as Error).message);
    }
  }
  return (
    <>
      <Heading
        eyebrow=""
        title="タスク"
        desc="処理状況と失敗理由"
        actions={
          <button className="secondary" onClick={() => location.reload()}>
            <RefreshCw size={16} /> 更新
          </button>
        }
      />
      <div className="filter-bar">
        <div className="filter-tabs">
          {[
            ["all", "すべて"],
            ["queued", "待機中"],
            ["running", "実行中"],
            ["failed", "失敗"],
            ["completed", "成功"],
          ].map(([v, l]) => (
            <button
              key={v}
              className={filter === v ? "active" : ""}
              onClick={() => setFilter(v)}
            >
              {l}
            </button>
          ))}
        </div>
        <span className="result-count">{items.length} 件</span>
      </div>
      <section className="panel table-panel">
        <Table
          headers={[
            "タスクID",
            "タスク種別",
            "状態",
            "優先度",
            "試行回数",
            "エラー / 結果",
            "ノード",
            "更新日時",
            "操作",
          ]}
        >
          {d.loading ? (
            <tr>
              <td colSpan={8} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={8} className="empty">
                {d.error}
              </td>
            </tr>
          ) : items.length ? (
            items.map((x: Obj) => (
              <tr key={x.id}>
                <td>
                  <code>{String(x.id).slice(0, 8)}</code>
                </td>
                <td>
                  {x.task_type === "source_fetch"
                    ? "元ファイル取得（agent）"
                    : x.task_type === "store"
                      ? "保存"
                      : x.task_type === "replicate"
                        ? "複製"
                        : x.task_type === "delete"
                          ? "削除"
                          : x.task_type}
                </td>
                <td>
                  <Status value={x.status} />
                </td>
                <td>
                  <span className="priority">{x.priority ?? "—"}</span>
                </td>
                <td>{x.attempt_count ?? 0} / 5</td>
                <td className="error-cell">{x.last_error || "—"}</td>
                <td>{x.target_node ? x.source_node : "-"}</td>
                <td>{date(x.updated_at)}</td>
                <td>
                  {role === "admin" && x.status === "failed" && (
                    <button className="small-btn" onClick={() => retry(x.id)}>
                      再試行
                    </button>
                  )}
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={8} className="empty">
                タスクがありません
              </td>
            </tr>
          )}
        </Table>
      </section>
      <div className="info-card">
        <Clock3 size={18} />
        <div>
          <b>再試行</b>
          <p>
            失敗タスクは最大5回まで自動で再試行します。管理者は手動でも再試行できます。
          </p>
        </div>
      </div>
    </>
  );
}
