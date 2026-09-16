// Panel rendering mode: client-side, per docs/SPEC-UI/001-SPEC-UI.md §4.
//
// The session lives in an HttpOnly cookie owned by app-serv, so there is nothing meaningful to
// render server-side for an authenticated view, and rendering one would duplicate that truth.

export const ssr = false;
export const prerender = false;
