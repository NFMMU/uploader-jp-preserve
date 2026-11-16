import { Catalog } from "@/features/review/Catalog";

export function AuthorArtifacts({ api, refresh, notify }: any) {
	return <Catalog api={api} refresh={refresh} notify={notify} role="author" />;
}
