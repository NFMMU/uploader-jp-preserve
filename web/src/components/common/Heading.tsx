import type React from "react";
export function Heading({
	eyebrow,
	title,
	desc,
	actions,
	level = "h1",
}: {
	eyebrow?: string;
	title: string | React.ReactNode;
	desc: string;
	actions?: React.ReactNode;
	level?: "h1" | "h2" | "h3";
}) {
	let titleBlock: React.ReactNode;
	switch (level) {
		case "h1":
			titleBlock = <h1>{title}</h1>;
			break;
		case "h2":
			titleBlock = <h2>{title}</h2>;
			break;
		case "h3":
			titleBlock = <h3>{title}</h3>;
			break;
		default:
			titleBlock = <h1>{title}</h1>;
			break;
	}
	return (
		<div className="page-heading">
			<div>
				<div className="eyebrow">{eyebrow}</div>
				{titleBlock}
				<p>{desc}</p>
			</div>
			{actions && <div className="heading-actions">{actions}</div>}
		</div>
	);
}
