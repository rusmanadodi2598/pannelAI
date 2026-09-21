// App namespace declarations. Locals carry nothing today: the panel reads session state over
// /api/v1/auth/status rather than holding server-side session data.

declare global {
	namespace App {
		interface Error {
			code: string;
			message: string;
		}
		interface Locals {}
		interface PageData {}
		interface PageState {}
		interface Platform {}
	}
}

export {};
