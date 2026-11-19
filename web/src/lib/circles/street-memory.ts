/**
 * Улочка этого захода приложения — для мгновенного возврата из круга (C97).
 * Отдельный модуль без зависимостей: память снимает и чтение кругов, и выход
 * из учётной записи, а тянуть их друг в друга нельзя (цикл импортов).
 */
import type { Street } from './circles';

let street: Street | undefined;

export function rememberStreet(next: Street): void {
	street = next;
}

export function peekStreetMemory(): Street | undefined {
	return street;
}

/** Выход или отзыв сессии: чужие названия кругов не должны мигнуть новому. */
export function forgetStreetMemory(): void {
	street = undefined;
}
