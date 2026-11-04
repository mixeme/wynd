import type { Progress } from './api';

/** Request identity outlives individual promises and delivery of queued events. */
export class ReleaseRequests {
	private checkRequest = '';
	private signRequest = '';
	private createId: () => string;

	constructor(createId: () => string) {
		this.createId = createId;
	}

	beginCheck(): string {
		this.signRequest = '';
		return (this.checkRequest = this.createId());
	}
	beginSign(): string {
		return (this.signRequest = this.createId());
	}
	clear(): void {
		this.checkRequest = this.signRequest = '';
	}
	isCurrentCheck(id: string): boolean {
		return id !== '' && id === this.checkRequest;
	}
	isCurrentSign(checkRequest: string, signRequest: string): boolean {
		return (
			this.isCurrentCheck(checkRequest) && signRequest !== '' && signRequest === this.signRequest
		);
	}
	acceptsProgress(p: Progress, tag?: string, checkId?: string): boolean {
		if (!tag || p.tag !== tag) return false;
		if (p.phase === 'check') return this.isCurrentCheck(p.request_id);
		return (
			p.phase === 'sign' &&
			!!checkId &&
			p.check_id === checkId &&
			this.isCurrentSign(this.checkRequest, p.request_id)
		);
	}
}
