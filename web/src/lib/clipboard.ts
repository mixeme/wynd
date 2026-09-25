/**
 * Копирует текст в буфер обмена. `navigator.clipboard` есть только в secure
 * context; локальный запуск с телефона по `http://192.168.…` его не имеет,
 * и там копирует скрытое поле через `execCommand('copy')`.
 */
export async function copyText(text: string): Promise<void> {
	if (navigator.clipboard) {
		await navigator.clipboard.writeText(text);
		return;
	}
	const area = document.createElement('textarea');
	area.value = text;
	area.setAttribute('readonly', '');
	area.style.position = 'fixed';
	area.style.opacity = '0';
	document.body.append(area);
	area.select();
	try {
		if (!document.execCommand('copy')) throw new Error('copy failed');
	} finally {
		area.remove();
	}
}
