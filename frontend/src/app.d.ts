/// <reference types="@sveltejs/kit" />
/// <reference types="@vite-pwa/sveltekit" />

declare module 'virtual:pwa-register' {
	export function registerSW(options?: {
		immediate?: boolean;
		onNeedRefresh?: () => void;
		onOfflineReady?: () => void;
	}): (reloadPage?: boolean) => Promise<void>;
}
