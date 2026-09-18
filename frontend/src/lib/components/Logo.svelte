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
		src="/assets/logo.png"
		alt="4LabsCloud Logo"
		style="width: {size}px; height: {size}px;"
		class="object-contain shrink-0 rounded-md"
	/>
	{#if showWordmark}
		<span class="font-bold tracking-tight text-lg leading-none whitespace-nowrap">
			<span class="text-[#0F3A6E] dark:text-[#3BC1E8]">4Labs</span><span class="text-[#1E6FD9] dark:text-[#F1F5F9]">Cloud</span>
		</span>
	{/if}
</div>
