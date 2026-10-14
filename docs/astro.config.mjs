// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://www.lanovo.link',
	integrations: [
		starlight({
			title: 'LANovo',
			description: 'Turn a Lenovo Smart Display into a local Home Assistant device.',
			logo: {
				light: './src/assets/logo-light.png',
				dark: './src/assets/logo-dark.png',
				alt: 'LANovo',
				replacesTitle: true,
			},
			customCss: ['./src/styles/custom.css'],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/ygelfand/LANovo' }],
			sidebar: [
				{
					label: 'Get started',
					items: [
						{ label: 'Supported devices', slug: 'start/devices' },
						{ label: 'Before you install', slug: 'start/before' },
						{ label: 'Install LANovo', slug: 'start/install' },
						{ label: 'Add it to Home Assistant', slug: 'start/home-assistant' },
					],
				},
				{
					label: 'Using it',
					items: [
						{ label: 'Getting around', slug: 'use/getting-around' },
						{ label: 'Dashboards', slug: 'use/dashboards' },
						{ label: 'Styles and themes', slug: 'use/styles' },
						{ label: 'The screen saver', slug: 'use/screen-saver' },
						{ label: 'Visuals', slug: 'use/visuals' },
						{ label: 'Configuration', slug: 'use/configuration' },
						{ label: 'Settings', slug: 'use/settings' },
					],
				},
				{
					label: 'Features',
					items: [
						{ label: 'Music and casting', slug: 'features/music' },
						{ label: 'Voice assistants', slug: 'features/voice' },
						{ label: 'Intercom', slug: 'features/intercom' },
						{ label: 'Messages and timers', slug: 'features/messages' },
						{ label: 'Apps', slug: 'features/apps' },
						{ label: 'Camera', slug: 'features/camera' },
						{ label: 'Privacy switches', slug: 'features/privacy' },
						{ label: 'Updates', slug: 'features/updates' },
					],
				},
			],
		}),
	],
});
