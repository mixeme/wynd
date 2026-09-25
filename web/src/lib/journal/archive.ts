/** Запись до отсечки активного цикла — без новых комментариев и реакций. */
export function isPostArchiveLocked(
	archiveActive: boolean,
	cutoffDate: string | undefined,
	createdAt: string
): boolean {
	if (!archiveActive || !cutoffDate) return false;
	const cutoff = new Date(`${cutoffDate}T00:00:00.000Z`);
	return new Date(createdAt) < cutoff;
}
