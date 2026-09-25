import { formatMonthYear } from '$lib/format/time';

export function groupByMonth<T extends { entryDate: string }>(
	items: T[]
): Array<{ month: string; label: string; items: T[] }> {
	const groups = new Map<string, T[]>();
	for (const item of items) {
		const month = item.entryDate.slice(0, 7);
		const list = groups.get(month) ?? [];
		list.push(item);
		groups.set(month, list);
	}
	return [...groups.entries()]
		.sort(([a], [b]) => b.localeCompare(a))
		.map(([month, groupItems]) => ({
			month,
			label: formatMonthYear(`${month}-01`),
			items: groupItems
		}));
}
