<script lang="ts">
	interface Props {
		name?: string;
		email?: string;
		src?: string;
		size?: 'sm' | 'md' | 'lg' | 'xl';
		class?: string;
	}

	let {
		name = '',
		email = '',
		src = undefined,
		size = 'md',
		class: customClass = ''
	}: Props = $props();

	let initials = $derived.by(() => {
		if (name && name.trim().length > 0) {
			const parts = name.trim().split(/\s+/);
			if (parts.length >= 2) {
				return (parts[0][0] + parts[1][0]).toUpperCase();
			}
			return parts[0].slice(0, 2).toUpperCase();
		}
		if (email && email.trim().length > 0) {
			return email.trim().slice(0, 2).toUpperCase();
		}
		return '4L';
	});

	let sizeClasses = $derived(
		size === 'sm'
			? 'w-7 h-7 text-xs'
			: size === 'lg'
				? 'w-11 h-11 text-base'
				: size === 'xl'
					? 'w-16 h-16 text-xl'
					: 'w-10 h-10 text-xs'
	);
</script>

<div
	class="inline-flex items-center justify-center rounded-full font-semibold select-none shrink-0 overflow-hidden ring-1 ring-white/10 {sizeClasses} {customClass}"
>
	{#if src}
		<img {src} alt={name || email || 'Avatar'} class="w-full h-full object-cover" />
	{:else}
		<div class="w-full h-full bg-gradient-to-br from-primary-dark to-primary text-white flex items-center justify-center">
			{initials}
		</div>
	{/if}
</div>
