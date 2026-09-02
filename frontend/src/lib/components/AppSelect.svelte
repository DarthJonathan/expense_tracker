<script lang="ts">
	import { Select } from 'bits-ui';
	import { onDestroy } from 'svelte';

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

	let clearClickThroughGuard: (() => void) | null = null;

	function isIOSBrowser(): boolean {
		if (typeof navigator === 'undefined') return false;
		return /iP(ad|hone|od)/.test(navigator.userAgent) ||
			(navigator.maxTouchPoints > 2 && /iPad|Macintosh/.test(navigator.userAgent));
	}

	function guardClickThrough(event: PointerEvent): void {
		if (event.defaultPrevented || event.button !== 0) return;
		const target = event.target;
		if (!(target instanceof Element) || !target.closest('[data-bits-select-item]')) return;

		// Bits UI selects an iOS item on pointerup and closes its portal immediately.
		// Safari can then dispatch the follow-up click to the button now exposed below it.
		// Android touch selects on click, so its click must continue to the item.
		if (event.pointerType === 'touch' && !isIOSBrowser()) return;

		clearClickThroughGuard?.();
		const consumeNextClick = (click: MouseEvent) => {
			click.preventDefault();
			click.stopImmediatePropagation();
			clearClickThroughGuard?.();
		};
		const timeoutId = window.setTimeout(() => clearClickThroughGuard?.(), 450);
		clearClickThroughGuard = () => {
			document.removeEventListener('click', consumeNextClick, true);
			window.clearTimeout(timeoutId);
			clearClickThroughGuard = null;
		};
		document.addEventListener('click', consumeNextClick, true);
	}

	onDestroy(() => clearClickThroughGuard?.());
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
						onpointerup={guardClickThrough}
					>
						{option.label}
					</Select.Item>
				{/each}
			</Select.Viewport>
		</Select.Content>
	</Select.Portal>
</Select.Root>
