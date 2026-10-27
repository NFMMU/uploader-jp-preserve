export function Status({ value }: { value: string }) {
  const map: Record<string, [string, string]> = {
    verified: ["検証済み", "green"],
    stored: ["保存済み", "green"],
    queued: ["待機中", "blue"],
    running: ["実行中", "blue"],
    succeeded: ["成功", "green"],
    completed: ["完了", "green"],
    failed: ["失敗", "red"],
    degraded: ["劣化", "amber"],
    missing: ["実体不明", "red"],
    deleted: ["削除済み", "gray"],
    error: ["エラー", "red"],
    online: ["オンライン", "green"],
    offline: ["オフライン", "red"],
    pending: ["未確認", "amber"],
    in_review: ["確認中", "blue"],
    allowed: ["再配布可", "green"],
    restricted: ["制限あり", "red"],
    unknown: ["不明", "amber"],
    in_progress: ["確認中", "blue"],
    uncollected: ["未取得", "gray"],
    accepted: ["承認", "green"],
    approved: ["承認済み", "green"],
    enrolled: ["登録済み", "green"],
    rejected: ["却下", "red"],
    closed: ["終了", "gray"],
  };
  const [label, tone] = map[value] || [value || "—", "gray"];
  return (
    <span className={"badge " + tone}>
      <i />
      {label}
    </span>
  );
}
