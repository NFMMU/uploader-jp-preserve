import { Heading } from "../../components/common/Heading";
import { Status } from "../../components/common/Status";
import { Table } from "../../components/common/Table";
import { useData } from "../../hooks/useData";
import { date } from "../../lib/format";
import type { Obj } from "../../types/common";

export function Audit({ api, refresh }: any) {
  const d = useData(api, "/admin/audit-logs", refresh);
  return (
    <>
      <Heading eyebrow="" title="操作履歴" desc="管理操作の履歴" />
      <section className="panel table-panel">
        <Table headers={["日時", "実行主体", "対象", "操作", "結果", "理由"]}>
          {d.loading ? (
            <tr>
              <td colSpan={6} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : d.error ? (
            <tr>
              <td colSpan={6} className="empty">
                {d.error}
              </td>
            </tr>
          ) : (
            (d.data.items || []).map((x: Obj) => (
              <tr key={x.id}>
                <td>{date(x.created_at)}</td>
                <td>
                  {x.actor_display_name ||
                    (x.actor_label === "anonymous"
                      ? "system"
                      : x.actor_label) ||
                    x.actor_email ||
                    x.actor_id ||
                    "system"}
                </td>
                <td>
                  {x.target_type}{" "}
                  <code>{String(x.target_id || "").slice(0, 8)}</code>
                </td>
                <td>{x.action}</td>
                <td>
                  <Status value={x.result || x.status || "success"} />
                </td>
                <td>{x.reason || x.details || "—"}</td>
              </tr>
            ))
          )}
        </Table>
      </section>
    </>
  );
}
