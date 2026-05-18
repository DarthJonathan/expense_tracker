import { sveltekit } from '@sveltejs/kit/vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		sveltekit(),
		SvelteKitPWA({
			registerType: 'autoUpdate',
			includeAssets: ['favicon.svg', 'icon.svg'],
			manifest: {
				name: 'Shared Expense Tracker',
				short_name: 'Expenses',
				description: 'Offline-first group expense tracking with backend API sync.',
				theme_color: '#0f766e',
				background_color: '#f8fafc',
				display: 'standalone',
				scope: '/',
				start_url: '/',
				icons: [
					{
						src: '/icon.svg',
						sizes: 'any',
						type: 'image/svg+xml',
						purpose: 'any maskable'
					}
				]
			},
			workbox: {
				globPatterns: ['client/**/*.{js,css,ico,png,svg,webp,woff2}'],
				navigateFallback: '/'
			},
			devOptions: {
				enabled: true
			}
		})
	]
});
