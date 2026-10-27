import type React from "react";
export function Stat({
	label,
	value,
	sub,
	icon: Icon,
	tone = "blue",
}: {
	label: string;
	value: React.ReactNode;
	sub: string;
	icon: any;
	tone?: string;
}) {
	return (
		<div className="stat-card">
			<div className={"stat-icon " + tone}>
				<Icon size={19} />
			</div>
			<div className="stat-label">{label}</div>
			<div className="stat-value">{value}</div>
			<div className="stat-sub">{sub}</div>
		</div>
	);
}
