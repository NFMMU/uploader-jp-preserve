import type React from "react";
export function Heading({
  eyebrow,
  title,
  desc,
  actions,
}: {
  eyebrow: string;
  title: string | React.ReactNode;
  desc: string;
  actions?: React.ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <div className="eyebrow">{eyebrow}</div>
        <h1>{title}</h1>
        <p>{desc}</p>
      </div>
      {actions && <div className="heading-actions">{actions}</div>}
    </div>
  );
}
