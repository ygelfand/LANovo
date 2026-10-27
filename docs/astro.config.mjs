// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: "https://www.lanovo.link",
	integrations: [
		starlight({
			title: "LANovo",
			description: "Turn a Lenovo Smart Display into a local Home Assistant device.",
			logo: {
				light: './src/assets/logo-light.png',
				dark: './src/assets/logo-dark.png',
				alt: "LANovo",
				replacesTitle: true,
			},
			customCss: ['./src/styles/custom.css', './src/styles/accent.css'],
			head: [{ tag: 'script', attrs: { src: '/zoom.js', defer: true } }],
			social: [{ icon: 'github', label: 'GitHub', href: "https://github.com/ygelfand/LANovo" }],
			sidebar: [
				{
					"label": "Get started",
					"items": [
						{
							"label": "Supported devices",
							"slug": "start/devices"
						},
						{
							"label": "Before you install",
							"slug": "start/before"
						},
						{
							"label": "Install LANovo",
							"slug": "start/install"
						},
						{
							"label": "Add it to Home Assistant",
							"slug": "start/home-assistant"
						},
						{
							"label": "The EchoLocal integration",
							"slug": "start/integration"
						}
					]
				},
				{
					"label": "Screen",
					"items": [
						{
							"label": "Getting around",
							"slug": "use/getting-around"
						},
						{
							"label": "Dashboards",
							"slug": "use/dashboards"
						},
						{
							"label": "Styles and themes",
							"slug": "use/styles"
						},
						{
							"label": "Screen saver",
							"slug": "use/screen-saver"
						},
						{
							"label": "Visuals",
							"slug": "use/visuals"
						}
					]
				},
				{
					"label": "Settings",
					"items": [
						{
							"label": "Where to change settings",
							"slug": "use/configuration"
						},
						{
							"label": "On the screen",
							"slug": "use/settings"
						},
						{
							"label": "In Home Assistant",
							"slug": "use/entities"
						},
						{
							"label": "In a browser",
							"slug": "use/web"
						},
						{
							"label": "All settings and defaults",
							"slug": "use/reference"
						}
					]
				},
				{
					"label": "Features",
					"items": [
						{
							"label": "Music and casting",
							"slug": "features/music"
						},
						{
							"label": "Voice assistants",
							"slug": "features/voice"
						},
						{
							"label": "Microphones",
							"slug": "features/microphones"
						},
						{
							"label": "Intercom",
							"slug": "features/intercom"
						},
						{
							"label": "Messages, timers and alarms",
							"slug": "features/messages"
						},
						{
							"label": "Custom tones",
							"slug": "features/tones"
						},
						{
							"label": "Apps",
							"slug": "features/apps"
						},
						{
							"label": "Photos and wallpaper",
							"slug": "features/photos"
						},
						{
							"label": "Doorbell",
							"slug": "features/doorbell"
						},
						{
							"label": "Home alarm",
							"slug": "features/home-alarm"
						},
						{
							"label": "Camera",
							"slug": "features/camera"
						},
						{
							"label": "Privacy switches",
							"slug": "features/privacy"
						},
						{
							"label": "Updates",
							"slug": "features/updates"
						}
					]
				},
				{
					"label": "Help",
					"items": [
						{
							"label": "Questions and answers",
							"slug": "use/faq"
						}
					]
				}
			],
		}),
	],
});
