type PaginationProps = {
	page: number;
	limit: number;
	total: number;
	loading?: boolean;
	onPageChange: (page: number) => void;
	onLimitChange: (limit: number) => void;
};

export function Pagination({
	page,
	limit,
	total,
	loading = false,
	onPageChange,
	onLimitChange,
}: PaginationProps) {
	const totalPages = Math.ceil(total / limit);
	const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
	const endItem = Math.min(page * limit, total);

	const pages: (number | "...")[] = [];
	if (totalPages > 0) {
		for (let p = 1; p <= totalPages; p++) {
			if (p === 1 || p === totalPages || Math.abs(p - page) <= 2) {
				const previous = pages[pages.length - 1];
				if (typeof previous === "number" && p - previous > 1) {
					pages.push("...");
				}
				pages.push(p);
			}
		}
	}

	return (
		<nav className="pagination" aria-label="ページネーション">
			<div className="pagination-info">
				<span>
					全 {total.toLocaleString()} 件中 {startItem.toLocaleString()}～
					{endItem.toLocaleString()} 件
				</span>

				<label className="pagination-limit">
					表示件数
					<select
						value={limit}
						disabled={loading}
						onChange={(e) => onLimitChange(Number(e.target.value))}
					>
						<option value={10}>10件</option>
						<option value={20}>20件</option>
						<option value={50}>50件</option>
						<option value={100}>100件</option>
					</select>
				</label>
			</div>

			<div className="pagination-controls">
				<button
					type="button"
					className="secondary"
					disabled={loading || page <= 1 || totalPages === 0}
					onClick={() => onPageChange(page - 1)}
				>
					前へ
				</button>

				{pages.map((p, index) =>
					p === "..." ? (
						<span
							key={`ellipsis-${index}`}
							className="pagination-ellipsis"
							aria-hidden="true"
						>
							…
						</span>
					) : (
						<button
							key={p}
							type="button"
							className={p === page ? "primary" : "secondary"}
							aria-current={p === page ? "page" : undefined}
							disabled={loading}
							onClick={() => onPageChange(p)}
						>
							{p}
						</button>
					),
				)}

				<button
					type="button"
					className="secondary"
					disabled={loading || totalPages === 0 || page >= totalPages}
					onClick={() => onPageChange(page + 1)}
				>
					次へ
				</button>
			</div>
		</nav>
	);
}
