<script lang="ts">
	interface Props {
		value: number;
		max?: number;
		size?: 'sm' | 'md' | 'lg';
		showLabel?: boolean;
		label?: string;
		color?: 'primary' | 'accent' | 'warning' | 'danger';
		class?: string;
	}

	let {
		value = 0,
		max = 100,
		size = 'md',
		showLabel = false,
		label = '',
		color = 'primary',
		class: customClass = ''
	}: Props = $props();

	let percentage = $derived(
		Math.min(100, Math.max(0, Math.round((value / (max || 1)) * 100)))
	);

	let barHeight = $derived(
		size === 'sm' ? 'h-1.5' : size === 'lg' ? 'h-3' : 'h-2'
	);

	let colorClasses = $derived(
		color === 'accent'
			? 'bg-accent'
			: color === 'warning'
				? 'bg-warning'
				: color === 'danger'
					? 'bg-danger'
					: 'bg-gradient-to-r from-primary to-primary-light'
	);
</script>

<div class="w-full {customClass}">
	{#if showLabel || label}
		<div class="flex justify-between text-xs text-muted-light dark:text-muted-dark mb-1 font-medium tabular-nums">
			<span>{label}</span>
			<span>{percentage}%</span>
		</div>
	{/if}
	<div class="w-full bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden {barHeight}">
		<div
			class="{colorClasses} {barHeight} rounded-full transition-all duration-300 ease-out"
			style="width: {percentage}%"
			role="progressbar"
			aria-valuenow={value}
			aria-valuemin={0}
			aria-valuemax={max}
		></div>
	</div>
</div>
