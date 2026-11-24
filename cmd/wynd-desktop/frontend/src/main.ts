import { mount } from 'svelte';
import '$lib/styles/tokens.css';
import '$lib/styles/ui.css';
import './app.css';
import App from './App.svelte';
import TrayMenu from './TrayMenu.svelte';

const trayMenu = new URLSearchParams(window.location.search).get('tray-menu') === '1';
mount(trayMenu ? TrayMenu : App, { target: document.getElementById('app')! });
