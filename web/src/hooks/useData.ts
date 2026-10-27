import { useEffect, useState } from "react";
import type { Obj } from "../types/common";

export function useData(
	api: (p: string, o?: RequestInit) => Promise<Obj>,
	path: string,
	refresh: number,
) {
	const [data, setData] = useState<Obj>({ items: [] });
	const [error, setError] = useState("");
	const [loading, setLoading] = useState(true);
	useEffect(() => {
		let live = true;
		setLoading(true);
		api(path)
			.then((d) => {
				if (live) {
					setData(d);
					setError("");
				}
			})
			.catch((e) => {
				if (live) setError(e.message);
			})
			.finally(() => live && setLoading(false));
		return () => {
			live = false;
		};
	}, [path, refresh]);
	return { data, error, loading };
}
