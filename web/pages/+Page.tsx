import { BrowserRouter, StaticRouter } from "react-router-dom";
import { usePageContext } from "vike-react/usePageContext";
import App from "../src/App";
import "../src/style.css";

export default function Page() {
	const pageContext = usePageContext();
	// Vike renders this component on the server and hydrates it in the browser.
	// StaticRouter avoids touching browser APIs during SSR; BrowserRouter keeps
	// the existing React Router navigation behavior after hydration.
	if (typeof window === "undefined") {
		return (
			<StaticRouter location={pageContext.urlOriginal}>
				<App />
			</StaticRouter>
		);
	}

	return (
		<BrowserRouter>
			<App />
		</BrowserRouter>
	);
}
