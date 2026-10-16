import { mount } from 'svelte';
import '$lib/styles/tokens.css';
import '$lib/styles/ui.css';
import './app.css';
import Window from './Window.svelte';

mount(Window, { target: document.getElementById('app')! });
