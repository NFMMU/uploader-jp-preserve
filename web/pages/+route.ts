import type { PageContext } from "vike/types";

// Existing React Router remains responsible for client-side page routing.
// Vike handles the initial request for every URL so metadata can be SSR'd.
export default function route(_pageContext: PageContext): boolean {
	return true;
}
