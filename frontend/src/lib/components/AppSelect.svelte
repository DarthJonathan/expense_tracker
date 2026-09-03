<script module lang="ts">
	let mountedSelectCount = 0;
	let clickThroughGuardExpiresAt = 0;
	let clearClickThroughGuard: (() => void) | null = null;

	function isIOSBrowser(): boolean {
		if (typeof navigator === 'undefined') return false;
		return /iP(ad|hone|od)/.test(navigator.userAgent) ||
			(navigator.maxTouchPoints > 2 && /iPad|Macintosh/.test(navigator.userAgent));
	}

	function isSelectItemEvent(event: PointerEvent): boolean {
		return event.composedPath().some(
			(node) =>
				typeof Element !== 'undefined' &&
				node instanceof Element &&
				(node.matches('[data-bits-select-item]') || node.classList.contains('bits-select-item'))
		);
	}

	function armClickThroughGuard(event: PointerEvent): void {
		// Bits UI selects an iOS option on pointerup, then unmounts the portal.
		// Safari can send the resulting click to the submit button revealed below it.
		if (event.pointerType !== 'touch' || !isIOSBrowser() || !isSelectItemEvent(event)) return;

		clearClickThroughGuard?.();
		clickThroughGuardExpiresAt = Date.now() + 900;
		const consumeNextClick = (click: MouseEvent) => {
			click.preventDefault();
			click.stopImmediatePropagation();
			clearClickThroughGuard?.();
		};
		const timeoutId = window.setTimeout(() => clearClickThroughGuard?.(), 900);
		clearClickThroughGuard = () => {
			document.removeEventListener('click', consumeNextClick, true);
			window.clearTimeout(timeoutId);
			clickThroughGuardExpiresAt = 0;
			clearClickThroughGuard = null;
		};
		document.addEventListener('click', consumeNextClick, true);
	}

	function handleDocumentPointerUp(event: PointerEvent): void {
		armClickThroughGuard(event);
	}

	function mountClickThroughGuard(): void {
		if (typeof document === 'undefined') return;
		mountedSelectCount += 1;
		if (mountedSelectCount === 1) {
			document.addEventListener('pointerup', handleDocumentPointerUp, true);
		}
	}

	function unmountClickThroughGuard(): void {
		if (typeof document === 'undefined') return;
		mountedSelectCount = Math.max(0, mountedSelectCount - 1);
		if (mountedSelectCount === 0) {
			document.removeEventListener('pointerup', handleDocumentPointerUp, true);
			clearClickThroughGuard?.();
		}
	}

	/**
	 * A fallback for Safari variants that dispatch a submit without exposing the
	 * synthetic click to the document capture listener.
	 */
	export function consumeSelectClickThroughGuard(): boolean {
		if (Date.now() > clickThroughGuardExpiresAt) {
			clearClickThroughGuard?.();
			return false;
		}
		if (!clickThroughGuardExpiresAt) return false;
		clearClickThroughGuard?.();
		return true;
	}
</script>

<script lang="ts">
	import { Select } from 'bits-ui';
	import { onMount } from 'svelte';

	type SelectOption = {
		value: string;
		label: string;
		disabled?: boolean;
	};

	export let value = '';
	export let name: string | undefined = undefined;
	export let options: SelectOption[] = [];
	export let placeholder = 'Select option';
	export let ariaLabel = 'Select option';
	export let required = false;
	export let disabled = false;
	export let triggerClass = '';

	onMount(() => {
		mountClickThroughGuard();
		return unmountClickThroughGuard;
	});
</script>

<Select.Root type="single" bind:value {name} {required} {disabled} items={options}>
	<Select.Trigger class={`bits-select-trigger ${triggerClass}`.trim()} aria-label={ariaLabel}>
		<Select.Value placeholder={placeholder} />
		<span class="bits-select-chevron" aria-hidden="true">▾</span>
	</Select.Trigger>
	<Select.Portal>
		<Select.Content class="bits-select-content" sideOffset={8}>
			<Select.Viewport class="bits-select-viewport">
				{#each options as option}
					<Select.Item
						class="bits-select-item"
						value={option.value}
						label={option.label}
						disabled={option.disabled}
					>
						{option.label}
					</Select.Item>
				{/each}
			</Select.Viewport>
		</Select.Content>
	</Select.Portal>
</Select.Root>
