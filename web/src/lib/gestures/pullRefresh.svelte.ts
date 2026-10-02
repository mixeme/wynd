import {
	PTR,
	pullEnd,
	pullFinished,
	pullIdle,
	pullMove,
	pullSettled,
	pullStart,
	type PullState
} from './pullToRefresh';

/**
 * «Потянуть, чтобы обновить» на экране (план 47, 2.8): касания списка,
 * пауза доигрыша и вызов обновления. Был скопирован в улочку и ленту —
 * вместе с таймером, который надо снимать при уходе с экрана (GUI-5).
 * Полосу со знаком рисует `$ui/data/PullRefresh` по `state`.
 */
export class PullRefresh {
	state = $state<PullState>(pullIdle());
	#timer: ReturnType<typeof setTimeout> | null = null;
	#scrollTop: () => number;
	#refresh: () => Promise<unknown>;

	/**
	 * @param scrollTop прокрутка списка: жест начинается только от верха.
	 * @param refresh перечитать данные; полоса держится, пока он не закончится.
	 */
	constructor(scrollTop: () => number, refresh: () => Promise<unknown>) {
		this.#scrollTop = scrollTop;
		this.#refresh = refresh;
	}

	start = (e: TouchEvent) => {
		const t = e.touches[0];
		this.state = pullStart(this.state, t?.clientY ?? 0, this.#scrollTop(), t?.clientX ?? 0);
	};

	move = (e: TouchEvent) => {
		const t = e.touches[0];
		this.state = pullMove(this.state, t?.clientY ?? 0, this.#scrollTop(), t?.clientX ?? 0);
	};

	end = () => {
		this.state = pullEnd(this.state);
		if (this.state.phase !== 'settling') return;
		this.#timer = setTimeout(() => {
			this.#timer = null;
			this.state = pullSettled(this.state);
			void this.#refresh().finally(() => (this.state = pullFinished()));
		}, PTR.settleMs);
	};

	/** Уход с экрана: таймер не должен доживать до размонтированного экрана. */
	destroy() {
		if (this.#timer !== null) clearTimeout(this.#timer);
		this.#timer = null;
	}
}
