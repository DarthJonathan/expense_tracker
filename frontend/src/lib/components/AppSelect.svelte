<script lang="ts">
	import { Select } from 'bits-ui';

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
					<Select.Item class="bits-select-item" value={option.value} label={option.label} disabled={option.disabled}>
						{option.label}
					</Select.Item>
				{/each}
			</Select.Viewport>
		</Select.Content>
	</Select.Portal>
</Select.Root>
