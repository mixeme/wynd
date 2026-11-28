import type { Backend, RemoteDirectoryFolder, RemoteDirectoryList } from '../api';

type DirectoryList = Extract<RemoteDirectoryList, { ok: true }>;
type DirectoryRow = { path: string; name: string; level: number; expandable: boolean; canSelect: boolean };

const childPath = (parent: string, name: string) => `${parent === '/' ? '' : parent}/${name}`;
const errorText = (error: unknown) => error instanceof Error ? error.message : String(error).replace(/^RuntimeError:\s*/, '');

export class InstallDirectories {
	private lists = $state<Record<string, DirectoryList>>({});
	expanded = $state<string[]>([]);
	selected = $state<{ path: string; canSelect: boolean } | null>(null);
	opening = $state(false);
	loadingBranch = $state('');
	creating = $state(false);
	saving = $state(false);
	name = $state('');
	error = $state('');
	private revision = 0;

	rows = $derived.by(() => {
		const rows: DirectoryRow[] = [];
		const append = (path: string, name: string, level: number, hasFolders: boolean, canSelect: boolean) => {
			const list = this.lists[path];
			const expandable = list ? Boolean(list.folders.length || list.truncated) : hasFolders;
			rows.push({ path, name, level, expandable, canSelect });
			if (list && this.expanded.includes(path)) {
				for (const child of list.folders) {
					append(childPath(path, child.name), child.name, level + 1, child.has_folders, child.can_select);
				}
			}
		};
		if (this.lists['/']) append('/', '/', 0, true, false);
		return rows;
	});
	truncated = $derived(this.rows.some(row => this.expanded.includes(row.path) && this.lists[row.path]?.truncated));

	constructor(private readonly api: Backend) {}

	private async browse(path: string, allowParent = false): Promise<DirectoryList> {
		const result = await this.api.BrowseInstallDirectories(path);
		if (!result.ok) throw new Error(result.message);
		if (!allowParent && result.path !== path) {
			throw new Error('Путь к папке изменился. Откройте выбор каталога заново.');
		}
		return result;
	}

	private addFolder(list: DirectoryList, folder: RemoteDirectoryFolder): DirectoryList {
		if (list.folders.some(item => item.name === folder.name)) return list;
		const folders = [...list.folders, folder].sort((a, b) => a.name.localeCompare(b.name));
		return { ...list, folders };
	}

	async open(path: string) {
		if (this.opening || this.selected) return;
		const revision = ++this.revision;
		this.opening = true;
		this.error = '';
		try {
			const selected = await this.browse(path.trim(), true);
			const lists = { [selected.path]: selected };
			let child = selected;
			while (child.parent && revision === this.revision) {
				const parent = await this.browse(child.parent);
				lists[parent.path] = this.addFolder(parent, {
					name: child.path.slice(child.path.lastIndexOf('/') + 1),
					has_folders: Boolean(child.folders.length || child.truncated),
					can_select: child.can_select
				});
				child = parent;
			}
			if (revision !== this.revision) return;
			this.lists = lists;
			this.expanded = Object.keys(lists);
			this.selected = { path: selected.path, canSelect: selected.can_select };
		} catch (error) {
			if (revision === this.revision) this.error = errorText(error) || 'Не удалось открыть каталог на сервере';
		} finally {
			if (revision === this.revision) this.opening = false;
		}
	}

	close() {
		if (this.saving) return;
		this.revision++;
		this.cancelCreation();
		this.opening = false;
		this.loadingBranch = '';
		this.selected = null;
		this.lists = {};
		this.expanded = [];
	}

	select(row: DirectoryRow) {
		this.error = '';
		this.selected = { path: row.path, canSelect: row.canSelect };
	}

	async toggle(path: string) {
		if (this.loadingBranch) return;
		this.error = '';
		if (this.expanded.includes(path)) {
			this.expanded = this.expanded.filter(dir => dir !== path);
			return;
		}
		const revision = this.revision;
		this.loadingBranch = path;
		try {
			const list = this.lists[path] ?? await this.browse(path);
			if (revision !== this.revision) return;
			this.lists[path] = list;
			this.expanded = [...this.expanded, path];
		} catch (error) {
			if (revision === this.revision) this.error = errorText(error) || 'Не удалось прочитать список папок';
		} finally {
			if (revision === this.revision) this.loadingBranch = '';
		}
	}

	beginCreation() {
		this.error = '';
		this.creating = true;
	}

	cancelCreation() {
		this.creating = false;
		this.name = '';
		this.error = '';
	}

	async create() {
		if (!this.selected || !this.name.trim() || this.saving || this.loadingBranch) return;
		const parent = this.selected.path;
		const name = this.name;
		this.saving = true;
		this.error = '';
		try {
			const list = this.lists[parent] ?? await this.browse(parent);
			const result = await this.api.CreateInstallDirectory(parent, name);
			if (!result.ok) throw new Error(result.message);
			this.lists[parent] = this.addFolder(list, { name, has_folders: false, can_select: true });
			const expanded = new Set(this.expanded);
			for (let path = parent; path; path = this.lists[path].parent ?? '') expanded.add(path);
			this.expanded = [...expanded];
			this.selected = { path: result.path, canSelect: true };
			this.cancelCreation();
		} catch (error) {
			this.error = errorText(error) || 'Не удалось создать папку на сервере';
		} finally {
			this.saving = false;
		}
	}
}
