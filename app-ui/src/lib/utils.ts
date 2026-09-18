// Shared class-name helper for the primitive layer.
//
// `clsx` resolves conditional class values and `tailwind-merge` collapses the conflicting Tailwind
// utilities that result, which is what lets a call site override a component default without a
// specificity fight. Kept in this file so the primitive layer has one import.
//
// The shadcn-svelte registry writes `export { cn } from "cn"` here. That package is a re-export of the
// same two helpers as a single dependency; the helpers themselves are used directly so the panel's
// dependency list stays explicit about what it depends on.

import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]): string {
	return twMerge(clsx(inputs));
}

// The primitive layer's prop helpers. Kept from the registry version because the generated components
// import them by name.

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChild<T> = T extends { child?: any } ? Omit<T, 'child'> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, 'children'> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };
