// Монтируется тот же принятый исходник a1d1e3ee; серверные маршруты в APK не входят.
import { mount } from 'svelte';
import Startup from '../routes/dev/smoke/android-startup/+page.svelte';
import '$lib/styles/tokens.css';
import '$lib/styles/ui.css';
import 'virtual:startup-frame.css';
import { initTheme, isDark } from '$lib/session/session.svelte';
import { initViewportHeight } from '$lib/session/viewport';
import { postAndroidAppearance } from '$lib/android/appearance';

initTheme();
initViewportHeight();
$effect.root(() => {
	$effect(() => {
		const dark = isDark();
		document.body.classList.toggle('dark', dark);
		postAndroidAppearance(dark ? '#211E1C' : '#F4F0E9', dark ? '#211E1C' : '#F4F0E9');
	});
});
mount(Startup, { target: document.getElementById('startup')! });
