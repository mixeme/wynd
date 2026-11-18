<script lang="ts">
	import { Application, Window as ThisWindow } from '@wailsio/runtime';
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import App from './App.svelte';

	// Своя рамка, как у Мастерской: окно без системной рамки (Frameless).
	interface Runtime {
		WindowMinimise(): void;
		WindowToggleMaximise(): void;
		WindowIsMaximised(): Promise<boolean>;
		Quit(): void;
	}
	// mock-стенд задаёт window.runtime в виде v2; настоящее окно берёт v3.
	const rt: Runtime = (window as unknown as { runtime?: Runtime }).runtime ?? {
		WindowMinimise: () => void ThisWindow.Minimise(),
		WindowToggleMaximise: () => void ThisWindow.ToggleMaximise(),
		WindowIsMaximised: () => ThisWindow.IsMaximised(),
		Quit: () => void Application.Quit(),
	};
	let maximised = $state(false);
	async function syncMaximised() {
		maximised = (await rt.WindowIsMaximised().catch(() => false)) ?? false;
	}
	void syncMaximised();
</script>

<svelte:window onresize={() => void syncMaximised()} />

<div class="ins-win" class:maximised>
	<TitleBar
		title="Wynd Подпись"
		{maximised}
		onminimise={() => rt.WindowMinimise()}
		ontoggle={() => rt.WindowToggleMaximise()}
		onclose={() => rt.Quit()}
	/>
	<App />
</div>
