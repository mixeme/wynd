import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { requestDeviceHere } from './map-locate';

// Место для «Карты»: один запрос при открытии экрана (план 46, A4).
// Сторож C163: первый запрос не должен жить 8 секунд — меньше, чем проходят
// объяснение «Место на карте» и системный запрос; на планшете 28.11 первый
// вызов уходил в ошибку по таймауту, и карта после «Разрешить» оставалась
// на точках круга. Повторное открытие центрировалось — разрешение уже было
// выдано и координаты приходили мгновенно.

type FakeGeo = Geolocation & {
	callback: ((pos: GeolocationPosition) => void) | undefined;
	onerror: ((err: GeolocationPositionError) => void) | undefined;
	options: PositionOptions | undefined;
};

function fakeGeolocation(): FakeGeo {
	const geo = {
		callback: undefined,
		onerror: undefined,
		options: undefined,
		getCurrentPosition(
			success: PositionCallback,
			error: PositionErrorCallback | undefined,
			options: PositionOptions | undefined
		) {
			geo.callback = success;
			geo.onerror = error ?? undefined;
			geo.options = options;
		},
		clearWatch() {},
		watchPosition() {
			return 0;
		}
	} as FakeGeo;
	return geo;
}

function grant(geo: FakeGeo, lat: number, lng: number): void {
	geo.callback?.({
		coords: {
			latitude: lat,
			longitude: lng,
			accuracy: 19,
			altitude: null,
			altitudeAccuracy: null,
			heading: null,
			speed: null
		},
		timestamp: Date.now()
	} as GeolocationPosition);
}

beforeEach(() => {
	// jsdom считает http://localhost незащищённым — запрос места в тестах разрешаем.
	vi.stubGlobal('isSecureContext', true);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('запрос места для карты', () => {
	it('C163: запрос не ограничен восемью секундами под диалогами разрешения', () => {
		const geo = fakeGeolocation();
		requestDeviceHere(() => {}, geo);
		expect(geo.options?.timeout).toBe(Infinity);
	});

	it('координаты попадают в onHere, пока пользователь не двигал карту', () => {
		const geo = fakeGeolocation();
		const onHere = vi.fn();
		requestDeviceHere(onHere, geo);
		expect(onHere).not.toHaveBeenCalled();
		grant(geo, 59.9481742, 30.4817029);
		expect(onHere).toHaveBeenCalledWith([59.9481742, 30.4817029]);
	});

	it('отказ или выключенное «Местоположение» — onHere не зовётся', () => {
		const geo = fakeGeolocation();
		const onHere = vi.fn();
		requestDeviceHere(onHere, geo);
		geo.onerror?.({ code: 1, message: 'denied' } as GeolocationPositionError);
		expect(onHere).not.toHaveBeenCalled();
	});

	it('без защищённого контекста запрос не делается', () => {
		vi.stubGlobal('isSecureContext', false);
		const geo = fakeGeolocation();
		requestDeviceHere(() => {}, geo);
		expect(geo.callback).toBeUndefined();
	});

	it('без геолокации (старый WebView) запрос не делается', () => {
		const onHere = vi.fn();
		requestDeviceHere(onHere, undefined);
		expect(onHere).not.toHaveBeenCalled();
	});
});
