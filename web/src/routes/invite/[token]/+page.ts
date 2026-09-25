export const prerender = false;

export function load({ params }) {
	return { token: params.token };
}
