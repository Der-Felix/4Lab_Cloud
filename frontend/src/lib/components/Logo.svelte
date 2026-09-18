<script lang="ts">
	interface Props {
		size?: number;
		showWordmark?: boolean;
		class?: string;
	}

	let { size = 32, showWordmark = true, class: customClass = '' }: Props = $props();
</script>

<div class="flex items-center gap-2.5 select-none {customClass}">
	<img
		src="/assets/logo-icon.svg"
		alt="4LabCloud Logo"
		style="width: {size}px; height: {size}px;"
		class="object-contain shrink-0"
	/>
	{#if showWordmark}
		<span class="font-bold tracking-tight text-lg leading-none whitespace-nowrap text-text-light dark:text-text-dark">
			<span class="text-[#0F3A6E] dark:text-[#3BC1E8]">4Lab</span><span class="text-[#1E6FD9] dark:text-[#F1F5F9]">Cloud</span>
		</span>
	{/if}
</div>
