import { mount } from 'svelte';
import '$lib/styles/tokens.css';
import '$lib/styles/ui.css';
import '$lib/styles/workshop.css';
// Раскладка окна — та же, что у Мастерской (контейнер .ins, поля, кнопки).
import '../../../wynd-installer/frontend/src/app.css';
import Window from './Window.svelte';

mount(Window, { target: document.getElementById('app')! });
