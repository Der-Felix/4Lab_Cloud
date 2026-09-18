<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props {
		id?: string;
		name?: string;
		type?: string;
		value?: string | number;
		placeholder?: string;
		label?: string;
		error?: string;
		hint?: string;
		required?: boolean;
		disabled?: boolean;
		autofocus?: boolean;
		minlength?: number;
		autocomplete?: HTMLInputAttributes['autocomplete'];
		class?: string;
		icon?: Snippet;
		oninput?: (event: Event) => void;
		onchange?: (event: Event) => void;
	}

	let {
		id,
		name,
		type = 'text',
		value = $bindable(''),
		placeholder = '',
		label = '',
		error = '',
		hint = '',
		required = false,
		disabled = false,
		autofocus = false,
		minlength = undefined,
		autocomplete = undefined,
		class: customClass = '',
		icon,
		oninput,
		onchange
	}: Props = $props();
</script>

<div class="w-full flex flex-col gap-1.5 {customClass}">
	{#if label}
		<label for={id} class="text-xs font-semibold uppercase tracking-wider text-muted-light dark:text-muted-dark">
			{label}
			{#if required}<span class="text-danger ml-0.5">*</span>{/if}
		</label>
	{/if}

	<div class="relative flex items-center">
		{#if icon}
			<div class="absolute left-3 flex items-center pointer-events-none text-muted-light dark:text-muted-dark">
				{@render icon()}
			</div>
		{/if}

		<input
			{id}
			{name}
			{type}
			bind:value
			{placeholder}
			{required}
			{disabled}
			{minlength}
			{autocomplete}
			{oninput}
			{onchange}
			class="w-full h-9.5 px-3.5 rounded-lg text-xs transition-all duration-150 outline-none
				bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark
				border {error ? 'border-danger focus:border-danger focus:ring-1 focus:ring-danger' : 'border-border-light dark:border-border-dark focus:border-primary focus:ring-1 focus:ring-primary'}
				placeholder:text-muted-light/60 dark:placeholder:text-muted-dark/50
				disabled:opacity-50 disabled:cursor-not-allowed
				{icon ? 'pl-9' : ''}"
		/>
	</div>

	{#if error}
		<p class="text-xs text-danger font-medium mt-0.5">{error}</p>
	{:else if hint}
		<p class="text-xs text-muted-light dark:text-muted-dark mt-0.5">{hint}</p>
	{/if}
</div>
