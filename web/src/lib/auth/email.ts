export const INVALID_EMAIL_HINT = 'Укажите почту вида имя@домен';

/** Mirrors server ParseParticipantEmail: local and domain non-empty, domain has a dot. */
export function isValidParticipantEmail(raw: string): boolean {
	const trimmed = raw.trim();
	if (!trimmed) return false;
	const angle = trimmed.match(/<([^>]+)>\s*$/);
	const addr = (angle ? angle[1] : trimmed).trim().toLowerCase();
	const at = addr.indexOf('@');
	if (at <= 0 || at === addr.length - 1) return false;
	const local = addr.slice(0, at);
	const domain = addr.slice(at + 1);
	return local.length > 0 && domain.length > 0 && domain.includes('.');
}
