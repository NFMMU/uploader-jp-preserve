import { date } from "../../lib/format";
import type { Obj } from "../../types/common";
import { Status } from "./Status";
import { Table } from "./Table";

export function TaskTable({
  items,
  loading,
  error,
}: {
  items: Obj[];
  loading: boolean;
  error: string;
}) {
  return (
    <Table headers={["タスクID", "種別", "状態", "優先度", "更新日時"]}>
      {loading ? (
        <tr>
          <td colSpan={5} className="empty">
            読み込み中…
          </td>
        </tr>
      ) : error ? (
        <tr>
          <td colSpan={5} className="empty">
            {error}
          </td>
        </tr>
      ) : items.length ? (
        items.map((x, i) => (
          <tr key={x.id || i}>
            <td>
              <code>{String(x.id || "").slice(0, 8) || "—"}</code>
            </td>
            <td>{x.task_type || "—"}</td>
            <td>
              <Status value={x.status} />
            </td>
            <td>
              <span className="priority">{x.priority ?? "—"}</span>
            </td>
            <td>{date(x.updated_at || x.created_at)}</td>
          </tr>
        ))
      ) : (
        <tr>
          <td colSpan={5} className="empty">
            タスクがありません
          </td>
        </tr>
      )}
    </Table>
  );
}
