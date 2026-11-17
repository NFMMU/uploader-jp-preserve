/// <reference types="vite/client" />
declare module "virtual:public-notices" {
	const notices: Array<{ path: string; source: string }>;
	export default notices;
}
