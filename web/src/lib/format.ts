/** Date and size formatting shared by the admin and public interfaces. */
export function date(v: string) {
	return v
		? new Date(v).toLocaleString("ja-JP", {
				dateStyle: "medium",
				timeStyle: "short",
			})
		: "—";
}

export function bytes(v: number) {
	if (v == null) return "—";
	const u = ["B", "KB", "MB", "GB", "TB", "PB"];
	let n = v,
		i = 0;
	while (n >= 1024 && i < u.length - 1) {
		n /= 1024;
		i++;
	}
	return `${n.toFixed(i ? 1 : 0)} ${u[i]}`;
}

export function formatBytes(size: number) {
	if (size < 1024) return `${size} B`;
	const units = ["KB", "MB", "GB", "TB"];
	let value = size / 1024;
	let index = 0;
	while (value >= 1024 && index < units.length - 1) {
		value /= 1024;
		index += 1;
	}
	return `${value.toFixed(1)} ${units[index]}`;
}
