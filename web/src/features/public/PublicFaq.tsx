import type React from "react";

export type PublicFaqItem = {
	question: string;
	answer: React.ReactNode;
};

export function PublicFaq({ items }: { items: PublicFaqItem[] }) {
	return (
		<dl className="public-faq">
			{items.map((item) => (
				<div className="public-faq-item" key={item.question}>
					<dt className="public-faq-question">
						<span className="public-faq-badge">Q</span>
						<span>{item.question}</span>
					</dt>
					<dd className="public-faq-answer">
						<span className="public-faq-badge public-faq-badge-answer">A</span>
						<span>{item.answer}</span>
					</dd>
				</div>
			))}
		</dl>
	);
}
