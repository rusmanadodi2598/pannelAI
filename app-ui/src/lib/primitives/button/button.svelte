<script lang="ts" module>
	// The panel's one source of button state (DESIGN.md §3.5). A control takes its rest, hover, pressed,
	// and disabled colour from these variants rather than from a class written at the call site, and the
	// values are the tokens in `src/lib/states.css`.
	//
	// One text colour per state: two `text-*` utilities in the same state resolve by stylesheet order
	// rather than by intent. No elevation (DESIGN.md §5). No focus ring either, because the panel has one
	// focus treatment, the `:focus-visible` rule in `src/app.css`.
	//
	// A disabled filled control says one of two things. Gated by a precondition it has not met, so the
	// action is not the operator's yet: it steps off the accent onto a neutral surface. In flight with a
	// call the operator just pressed for: it keeps the accent and carries `aria-busy`, because grey under
	// a finger reads as a broken button.
	import { type VariantProps, tv } from "tailwind-variants";
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAnchorAttributes, HTMLButtonAttributes } from "svelte/elements";

	// Two heights: 44px for anything a screen asks the operator to press (SPEC-UI §8.7, §11.3), and the
	// dense desktop pair `sm`/`icon-sm` at 36, one height so an icon and a text button in one toolbar
	// finish on the same line.
	export const buttonVariants = tv({
		base: "group/button inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded-[var(--radius-sm)] border border-transparent bg-clip-padding text-sm font-medium transition-colors select-none active:translate-y-px disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
		variants: {
			variant: {
				// The one primary action on a screen (DESIGN.md §3.4).
				default:
					"bg-[var(--color-accent)] text-[var(--color-accent-text)] hover:bg-[var(--color-accent-hover)] active:bg-[var(--color-accent-press)] disabled:bg-[var(--color-surface-3)] disabled:text-[var(--color-text-muted)] disabled:opacity-100 [&[aria-busy='true']:disabled]:bg-[var(--color-accent)] [&[aria-busy='true']:disabled]:text-[var(--color-accent-text)]",
				outline:
					"border-[var(--color-border)] bg-transparent text-[var(--color-text)] hover:bg-[var(--color-state-hover)] active:bg-[var(--color-state-press)]",
				ghost:
					"text-[var(--color-text-muted)] hover:bg-[var(--color-state-hover)] hover:text-[var(--color-text)] active:bg-[var(--color-state-press)]",
				// A filter chip. Selection rides `aria-pressed`, which is what announces the chosen one, so
				// the state is read from the attribute rather than from a class a call site must remember
				// to apply, and a chosen chip still answers a pointer instead of sitting at its own ground.
				toggle:
					"border-[var(--color-border)] bg-transparent text-[var(--color-text-muted)] hover:bg-[var(--color-state-hover)] hover:text-[var(--color-text)] active:bg-[var(--color-state-press)] aria-[pressed=true]:border-[var(--color-text)] aria-[pressed=true]:bg-[var(--color-surface-3)] aria-[pressed=true]:font-medium aria-[pressed=true]:text-[var(--color-text)] aria-[pressed=true]:hover:bg-[var(--color-state-deep)]",
				// The commit in a confirmation dialog. A solid control needs a fixed label pair the way the
				// accent does, so `danger-ink` is to `danger` what `accent-text` is to `accent`.
				destructive:
					"bg-[var(--color-danger)] text-[var(--color-danger-ink)] hover:bg-[var(--color-danger-hover)] active:bg-[var(--color-danger-press)] disabled:bg-[var(--color-surface-3)] disabled:text-[var(--color-text-muted)] disabled:opacity-100 [&[aria-busy='true']:disabled]:bg-[var(--color-danger)] [&[aria-busy='true']:disabled]:text-[var(--color-danger-ink)]",
				// A danger action beside others in a row: the glyph carries the colour and the fill stays
				// the neutral wash, because a red block on every row shouts. The glyph cannot sit on a
				// pressed wash in its rest colour (4.17:1 on a raised panel), so it steps the danger ramp.
				"destructive-ghost":
					"text-[var(--color-danger)] hover:bg-[var(--color-state-hover)] hover:text-[var(--color-danger-hover)] active:bg-[var(--color-state-press)] active:text-[var(--color-danger-press)]"
			},
			size: {
				default: "min-h-11 gap-2 px-3",
				sm: "min-h-9 gap-1.5 px-2.5 text-xs",
				icon: "size-11",
				"icon-sm": "size-9"
			}
		},
		defaultVariants: {
			variant: "default",
			size: "default"
		}
	});

	export type ButtonVariant = VariantProps<typeof buttonVariants>["variant"];
	export type ButtonSize = VariantProps<typeof buttonVariants>["size"];

	export type ButtonProps = WithElementRef<HTMLButtonAttributes> &
		WithElementRef<HTMLAnchorAttributes> & {
			variant?: ButtonVariant;
			size?: ButtonSize;
		};
</script>

<script lang="ts">
	let {
		class: className,
		variant = "default",
		size = "default",
		ref = $bindable(null),
		href = undefined,
		type = "button",
		disabled,
		children,
		...restProps
	}: ButtonProps = $props();
</script>

{#if href}
	<a
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		href={disabled ? undefined : href}
		aria-disabled={disabled}
		role={disabled ? "link" : undefined}
		tabindex={disabled ? -1 : undefined}
		{...restProps}
	>
		{@render children?.()}
	</a>
{:else}
	<button
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		{type}
		{disabled}
		{...restProps}
	>
		{@render children?.()}
	</button>
{/if}
