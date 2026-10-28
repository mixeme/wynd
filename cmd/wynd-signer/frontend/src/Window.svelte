<script lang="ts">
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import App from './App.svelte';

	// Своя рамка, как у Мастерской: окно без системной рамки (Frameless).
	interface Runtime {
		WindowMinimise(): void;
		WindowToggleMaximise(): void;
		WindowIsMaximised(): Promise<boolean>;
		Quit(): void;
	}
	const rt = (window as unknown as { runtime?: Runtime }).runtime;
	let maximised = $state(false);
	async function syncMaximised() {
		maximised = (await rt?.WindowIsMaximised().catch(() => false)) ?? false;
	}
	void syncMaximised();
</script>

<svelte:window onresize={() => void syncMaximised()} />

<div class="ins-win" class:maximised>
	<TitleBar
		title="Wynd Подпись"
		{maximised}
		onminimise={() => rt?.WindowMinimise()}
		ontoggle={() => rt?.WindowToggleMaximise()}
		onclose={() => rt?.Quit()}
	/>
	<App />
</div>
