/**
 * Какие разделы круга на широком экране живут в левой колонке настроек круга,
 * а не в списке кругов (план 76): сами настройки и их подразделы — квота,
 * архив, отделение, перенос.
 */
export function isCircleSettingsPath(pathname: string, id: string): boolean {
	if (!id) return false;
	const path = pathname.replace(/\/+$/, '') || '/';
	const root = `/circles/${id}`;
	const under = (base: string) => path === base || path.startsWith(`${base}/`);
	return (
		under(`${root}/settings`) ||
		under(`${root}/quota`) ||
		path === `${root}/archive` ||
		path === `${root}/split` ||
		path === `${root}/move`
	);
}
