import { useEffect, useRef, useState } from "react";
import type { Obj } from "@/types/common";

type Api = (path: string, opts?: RequestInit) => Promise<Obj>;

export type ReconcileCounts = {
	files_scanned?: number;
	replicas_scheduled?: number;
	rebalance_moves?: number;
	deletes_queued?: number;
	moves_abandoned?: number;
	errors?: number;
	reasons?: Record<string, number>;
};

export type ReconcileStatus = {
	status: "idle" | "running" | "completed" | "failed";
	run_id?: string;
	trigger?: string;
	started_at?: string;
	finished_at?: string;
	elapsed_seconds?: number;
	counts?: ReconcileCounts;
	reasons?: Record<string, number>;
	error?: string;
};

const reasonLabels: Record<string, string> = {
	deletion_pending: "削除依頼済み",
	active_tasks: "処理中のタスクあり",
	replicas_satisfied: "複製数は足りている",
	no_healthy_source: "健全なコピー元がない",
	no_target_capacity: "空き容量のあるノードがない",
	rebalance_not_exactly_two: "複製が2つでない",
	rebalance_unstable_copies: "コピーが安定していない",
	rebalance_no_hot_source: "逼迫したノードがない",
	rebalance_no_target_capacity: "移動先の空き容量がない",
	rebalance_cooldown: "直前に移動済み",
};

function topReason(reasons?: Record<string, number>): [string, number] | null {
	let best: [string, number] | null = null;
	for (const [code, count] of Object.entries(reasons || {})) {
		if (!best || count > best[1]) best = [code, count];
	}
	return best;
}

export function summarizeReconcile(status: ReconcileStatus | null): string {
	if (!status) return "";
	if (status.status === "running") {
		return `保存先を調整中…（${Math.trunc(status.elapsed_seconds || 0)} 秒）`;
	}
	if (status.status === "failed") {
		return `保存先の調整に失敗しました: ${status.error || "不明なエラー"}`;
	}
	const counts = status.counts || {};
	const parts: string[] = [];
	if (counts.rebalance_moves) parts.push(`移動 ${counts.rebalance_moves}件`);
	if (counts.deletes_queued) parts.push(`削除待ち ${counts.deletes_queued}件`);
	if (counts.replicas_scheduled)
		parts.push(`複製 ${counts.replicas_scheduled}件`);
	if (counts.moves_abandoned)
		parts.push(`移動中止 ${counts.moves_abandoned}件`);
	const top = topReason(counts.reasons || status.reasons);
	const detail = top
		? `（主な見送り理由: ${reasonLabels[top[0]] || top[0]} ×${top[1]}）`
		: "";
	if (parts.length === 0) {
		return `保存先の変更はありません${detail}`;
	}
	return `調整しました: ${parts.join(" / ")}${detail}`;
}

// useReconcile starts an asynchronous placement reconciliation and polls its
// status until it finishes. onFinished is called once per run so callers can
// refetch their data.
export function useReconcile(
	api: Api,
	notify: (message: string) => void,
	onFinished: () => void,
) {
	const [running, setRunning] = useState(false);
	const [status, setStatus] = useState<ReconcileStatus | null>(null);
	const timer = useRef<number | undefined>(undefined);
	const alive = useRef(true);

	useEffect(() => {
		alive.current = true;
		return () => {
			alive.current = false;
			window.clearTimeout(timer.current);
		};
	}, []);

	const poll = async (deadline: number) => {
		try {
			const next = (await api(
				"/admin/placements/reconcile",
			)) as ReconcileStatus;
			if (!alive.current) return;
			setStatus(next);
			if (
				next.status === "completed" ||
				next.status === "failed" ||
				next.status === "idle"
			) {
				setRunning(false);
				notify(summarizeReconcile(next));
				onFinished();
				return;
			}
		} catch (e) {
			if (!alive.current) return;
			setRunning(false);
			notify(e instanceof Error ? e.message : "状態を取得できませんでした");
			return;
		}
		if (Date.now() > deadline) {
			setRunning(false);
			notify("調整の完了を確認できませんでした。再読み込みしてください。");
			return;
		}
		timer.current = window.setTimeout(() => poll(deadline), 1000);
	};

	const start = async () => {
		if (running) return;
		setRunning(true);
		setStatus(null);
		try {
			const response = await api("/admin/placements/reconcile", {
				method: "POST",
			});
			if (response.status === "already_running") {
				notify("保存先の調整は実行中です（完了を待機します）");
			} else {
				notify("保存先の調整を開始しました");
			}
			poll(Date.now() + 10 * 60 * 1000);
		} catch (e) {
			setRunning(false);
			notify(e instanceof Error ? e.message : "調整を開始できませんでした");
		}
	};

	return { start, running, status, summary: summarizeReconcile(status) };
}
