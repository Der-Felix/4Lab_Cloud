<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import Logo from '$lib/components/Logo.svelte';
	import { currentUser, isAuthenticated, authInitialized } from '$lib/stores';
	import { initializeSession, logout } from '$lib/auth';
	import {
		Toast,
		Avatar,
		Badge,
		Button,
		Tooltip,
		Dropdown
	} from '$lib/components/ui';
	import {
		IconLayoutDashboard,
		IconLayoutDashboardFilled,
		IconFolder,
		IconFolderFilled,
		IconPhoto,
		IconPhotoFilled,
		IconShare,
		IconSettings,
		IconSettingsFilled,
		IconLayoutSidebarLeftCollapse,
		IconLayoutSidebarLeftExpand,
		IconSearch,
		IconBell,
		IconLogout,
		IconSun,
		IconMoon,
		IconChevronRight,
		IconUser,
		IconShieldCheck
	} from '@tabler/icons-svelte';

	let { children } = $props();

	// Sidebar-Zustand (Standard: 240px ausgeklappt, via Cookie persistiert)
	let isCollapsed = $state(false);
	let isDarkMode = $state(true);
	let searchQuery = $state('');

	// Auth-Pfade, auf denen kein Dashboard-Layout gerendert werden soll
	let isAuthPage = $derived(
		page.url.pathname.startsWith('/login') ||
		page.url.pathname.startsWith('/register') ||
		page.url.pathname.startsWith('/logout') ||
		page.url.pathname.startsWith('/share')
	);

	let currentPath = $derived(page.url.pathname);

	// Breadcrumb-Label
	let currentTitle = $derived.by(() => {
		if (currentPath === '/') return 'Dashboard';
		if (currentPath.startsWith('/files/upload')) return 'Upload';
		if (currentPath.startsWith('/files')) return 'Dateien';
		if (currentPath.startsWith('/photos')) return 'Fotos';
		if (currentPath.startsWith('/shares')) return 'Freigaben';
		if (currentPath.startsWith('/settings/mfa')) return 'MFA Sicherheit';
		if (currentPath.startsWith('/settings')) return 'Einstellungen';
		return '4labscloud';
	});

	// Cookie Helper fuer Sidebar-Zustand (Path=/, SameSite=Strict, max-age=1y, Secure=true)
	function getSidebarCookie(): boolean {
		if (typeof document === 'undefined') return false;
		const match = document.cookie.match(/(?:^|;\s*)sidebar_collapsed=([^;]*)/);
		return match ? match[1] === 'true' : false;
	}

	function setSidebarCookie(collapsed: boolean) {
		if (typeof document === 'undefined') return;
		const secure = window.location.protocol === 'https:' ? '; Secure' : '';
		document.cookie = `sidebar_collapsed=${collapsed}; path=/; max-age=31536000; SameSite=Strict${secure}`;
	}

	onMount(async () => {
		// Theme-Initialisierung: Dark Mode First (UI-Preference im localStorage erlaubt)
		const savedTheme = localStorage.getItem('theme');
		if (savedTheme === 'light') {
			isDarkMode = false;
			document.documentElement.classList.remove('dark');
		} else {
			isDarkMode = true;
			document.documentElement.classList.add('dark');
		}

		// Sidebar-Zustand aus Cookie lesen
		isCollapsed = getSidebarCookie();

		// Auth Session pruefen
		const loggedIn = await initializeSession();
		if (!loggedIn && !isAuthPage) {
			goto('/login');
		} else if (loggedIn && (page.url.pathname === '/login' || page.url.pathname === '/register')) {
			goto('/');
		}
	});

	function toggleTheme() {
		isDarkMode = !isDarkMode;
		if (isDarkMode) {
			document.documentElement.classList.add('dark');
			localStorage.setItem('theme', 'dark');
		} else {
			document.documentElement.classList.remove('dark');
			localStorage.setItem('theme', 'light');
		}
	}

	function toggleSidebar() {
		isCollapsed = !isCollapsed;
		setSidebarCookie(isCollapsed);
	}

	async function handleLogout() {
		await logout();
		goto('/login');
	}

	function handleSearch(e: KeyboardEvent) {
		if (e.key === 'Enter' && searchQuery.trim()) {
			goto(`/files?search=${encodeURIComponent(searchQuery.trim())}`);
		}
	}

	interface NavItem {
		href: string;
		label: string;
		iconOutline: any;
		iconFilled: any;
		exact?: boolean;
	}

	interface NavGroup {
		title: string;
		items: NavItem[];
	}

	const navGroups: NavGroup[] = [
		{
			title: 'DASHBOARD',
			items: [
				{
					href: '/',
					label: 'Dashboard',
					iconOutline: IconLayoutDashboard,
					iconFilled: IconLayoutDashboardFilled,
					exact: true
				}
			]
		},
		{
			title: 'DATEIEN',
			items: [
				{
					href: '/files',
					label: 'Dateien',
					iconOutline: IconFolder,
					iconFilled: IconFolderFilled,
					exact: false
				},
				{
					href: '/photos',
					label: 'Fotos',
					iconOutline: IconPhoto,
					iconFilled: IconPhotoFilled,
					exact: false
				},
				{
					href: '/shares',
					label: 'Freigaben',
					iconOutline: IconShare,
					iconFilled: IconShare, // Fallback Outline
					exact: false
				}
			]
		},
		{
			title: 'SYSTEM',
			items: [
				{
					href: '/settings',
					label: 'Einstellungen',
					iconOutline: IconSettings,
					iconFilled: IconSettingsFilled,
					exact: false
				}
			]
		}
	];
</script>

<svelte:head>
	<title>{currentTitle} - 4labscloud</title>
</svelte:head>

<!-- Globale Toasts -->
<Toast />

{#if isAuthPage}
	<main class="min-h-[100dvh] flex flex-col justify-center items-center bg-bg-light dark:bg-bg-dark p-4 text-text-light dark:text-text-dark">
		{@render children()}
	</main>
{:else}
	<div class="min-h-[100dvh] flex bg-bg-light dark:bg-bg-dark text-text-light dark:text-text-dark font-sans antialiased">
		<!-- Sidebar (Seafile/Nextcloud-Struktur, 240px <-> 64px) -->
		<aside
			class="shrink-0 flex flex-col justify-between border-r border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark transition-[width] duration-200 ease-in-out z-40 sticky top-0 h-screen select-none {isCollapsed ? 'w-16' : 'w-60'}"
		>
			<!-- Oberer Bereich: Logo (64px) & Collapse-Toggle oben rechts immer sichtbar -->
			<div>
				<div class="h-16 flex items-center border-b border-border-light dark:border-border-dark {isCollapsed ? 'justify-between px-2.5' : 'justify-between px-3.5'}">
					<a href="/" class="flex items-center overflow-hidden focus:outline-hidden" title="4labscloud">
						{#if isCollapsed}
							<Logo size={24} showWordmark={false} />
						{:else}
							<Logo size={28} showWordmark={true} />
						{/if}
					</a>
					<button
						type="button"
						onclick={toggleSidebar}
						class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-primary hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
						aria-label={isCollapsed ? 'Sidebar ausklappen' : 'Sidebar einklappen'}
						title={isCollapsed ? 'Sidebar ausklappen' : 'Sidebar einklappen'}
					>
						{#if isCollapsed}
							<IconLayoutSidebarLeftExpand size={18} stroke={1.75} />
						{:else}
							<IconLayoutSidebarLeftCollapse size={18} stroke={1.75} />
						{/if}
					</button>
				</div>

				<!-- Navigation mit Gruppen-Struktur (DASHBOARD, DATEIEN, SYSTEM) -->
				<nav class="p-2 space-y-4 mt-2">
					{#each navGroups as group}
						<div>
							{#if !isCollapsed}
								<div class="text-[10px] font-semibold uppercase tracking-wider text-muted-light dark:text-muted-dark mb-2 ml-3 select-none">
									{group.title}
								</div>
							{/if}

							<div class="space-y-1">
								{#each group.items as item}
									{@const isActive = item.exact ? currentPath === item.href : currentPath.startsWith(item.href)}
									{@const OutlineIcon = item.iconOutline}
									{@const FilledIcon = item.iconFilled}

									{#if isCollapsed}
										<Tooltip text={item.label} position="right" class="w-full flex justify-center">
											<a
												href={item.href}
												class="w-10 h-10 flex items-center justify-center rounded-lg transition-all duration-120 ease-out {
													isActive
														? 'bg-primary text-white border-l-[3px] border-primary-light'
														: 'border-l-[3px] border-transparent text-muted-light dark:text-muted-dark hover:text-primary hover:bg-slate-100 dark:hover:bg-slate-800/60'
												}"
												aria-label={item.label}
											>
												{#if isActive}
													<FilledIcon size={20} class="shrink-0 text-white" />
												{:else}
													<OutlineIcon size={20} stroke={1.75} class="shrink-0" />
												{/if}
											</a>
										</Tooltip>
									{:else}
										<a
											href={item.href}
											class="h-10 px-3 flex items-center gap-2 rounded-lg text-sm font-medium transition-all duration-120 ease-out {
												isActive
													? 'bg-primary text-white border-l-[3px] border-primary-light'
													: 'border-l-[3px] border-transparent text-muted-light dark:text-muted-dark hover:text-primary hover:bg-slate-100 dark:hover:bg-slate-800/60'
											}"
										>
											{#if isActive}
												<FilledIcon size={20} class="shrink-0 text-white" />
											{:else}
												<OutlineIcon size={20} stroke={1.75} class="shrink-0" />
											{/if}
											<span class="truncate">{item.label}</span>
										</a>
									{/if}
								{/each}
							</div>
						</div>
					{/each}
				</nav>
			</div>

			<!-- Unterer Bereich: User-Bereich, Theme-Toggle & Logout -->
			<div class="p-2 border-t border-border-light dark:border-border-dark space-y-2">
				<!-- Theme Toggle -->
				{#if isCollapsed}
					<Tooltip text={isDarkMode ? 'Heller Modus' : 'Dunkler Modus'} position="right" class="w-full flex justify-center">
						<button
							type="button"
							onclick={toggleTheme}
							class="flex items-center justify-center w-10 h-10 rounded-lg text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors cursor-pointer"
							aria-label="Design umschalten"
						>
							{#if isDarkMode}
								<IconSun size={20} stroke={1.75} class="text-amber-400" />
							{:else}
								<IconMoon size={20} stroke={1.75} class="text-primary" />
							{/if}
						</button>
					</Tooltip>
				{:else}
					<button
						type="button"
						onclick={toggleTheme}
						class="w-full flex items-center justify-between px-3 py-2 rounded-lg text-xs font-medium text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors cursor-pointer"
					>
						<span class="flex items-center gap-2">
							{#if isDarkMode}
								<IconSun size={18} stroke={1.75} class="text-amber-400" />
								<span>Hell</span>
							{:else}
								<IconMoon size={18} stroke={1.75} class="text-primary" />
								<span>Dunkel</span>
							{/if}
						</span>
						<Badge variant="neutral" size="sm">
							{isDarkMode ? 'Dark' : 'Light'}
						</Badge>
					</button>
				{/if}

				<!-- User-Bereich unten: Avatar 40px, Email + Rolle 2-Zeiler, Logout-Icon rechts -->
				{#if isCollapsed}
					<Tooltip text={$currentUser?.email ? `${$currentUser.email} (Abmelden)` : 'Abmelden'} position="right" class="w-full flex justify-center">
						<button
							type="button"
							onclick={handleLogout}
							class="flex items-center justify-center w-10 h-10 rounded-lg text-danger hover:bg-danger/10 transition-colors cursor-pointer"
							aria-label="Abmelden"
						>
							<IconLogout size={20} stroke={1.75} />
						</button>
					</Tooltip>
				{:else}
					<div class="flex items-center justify-between p-2 rounded-lg bg-slate-50 dark:bg-slate-900/50 border border-border-light dark:border-border-dark">
						<div class="flex items-center gap-2.5 min-w-0">
							<Avatar email={$currentUser?.email} size="md" />
							<div class="min-w-0">
								<p class="text-xs font-semibold truncate text-text-light dark:text-text-dark leading-tight">
									{$currentUser?.email || 'Benutzer'}
								</p>
								<p class="text-[10px] text-muted-light dark:text-muted-dark mt-0.5 leading-tight">
									{$currentUser?.isAdmin ? 'Administrator' : 'Benutzer'}
								</p>
							</div>
						</div>
						<button
							type="button"
							onclick={handleLogout}
							title="Abmelden"
							class="p-1.5 text-muted-light hover:text-danger dark:text-muted-dark dark:hover:text-danger rounded-md transition-colors cursor-pointer"
							aria-label="Abmelden"
						>
							<IconLogout size={18} stroke={1.75} />
						</button>
					</div>
				{/if}
			</div>
		</aside>

		<!-- Hauptbereich (Header + Content) -->
		<div class="flex-1 flex flex-col min-w-0 overflow-x-hidden">
			<!-- Sticky Header -->
			<header
				class="h-16 sticky top-0 z-30 flex items-center justify-between px-6 border-b border-border-light dark:border-border-dark bg-surface-light/85 dark:bg-surface-dark/85 backdrop-blur-md"
			>
				<!-- Breadcrumbs (klein, muted) -->
				<div class="flex items-center gap-1.5 text-xs text-muted-light dark:text-muted-dark">
					<a href="/" class="hover:text-text-light dark:hover:text-text-dark transition-colors">
						4labscloud
					</a>
					<IconChevronRight size={14} stroke={1.75} class="opacity-60" />
					<span class="font-semibold text-text-light dark:text-text-dark">
						{currentTitle}
					</span>
				</div>

				<!-- Rechte Sektion: Globale Suche (240px), Glocke, Avatar-Dropdown -->
				<div class="flex items-center gap-3">
					<!-- Globale Dateisuche (exakt 240px) -->
					<div class="relative w-60">
						<IconSearch size={18} stroke={1.75} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-light dark:text-muted-dark pointer-events-none" />
						<input
							type="text"
							placeholder="Dateien suchen..."
							bind:value={searchQuery}
							onkeydown={handleSearch}
							class="w-full pl-9 pr-3 py-1.5 text-xs rounded-lg bg-slate-100 dark:bg-bg-dark border border-border-light dark:border-border-dark text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:outline-hidden focus:border-primary focus:ring-1 focus:ring-primary transition-all"
						/>
					</div>

					<!-- Benachrichtigungs-Glocke (18px, mit Badge-Punkt) -->
					<button
						type="button"
						class="relative p-2 rounded-lg text-muted-light dark:text-muted-dark hover:text-primary hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors cursor-pointer"
						title="Benachrichtigungen"
						aria-label="Benachrichtigungen"
					>
						<IconBell size={18} stroke={1.75} />
						<span class="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-accent ring-2 ring-surface-light dark:ring-surface-dark"></span>
					</button>

					<!-- Avatar-Dropdown -->
					<Dropdown align="right" width="w-56">
						{#snippet trigger()}
							<button
								type="button"
								class="flex items-center gap-2 p-1 rounded-full hover:ring-2 hover:ring-primary/40 transition-all focus:outline-hidden cursor-pointer"
								aria-label="Benutzer-Menü"
							>
								<Avatar email={$currentUser?.email} size="sm" />
							</button>
						{/snippet}

						<div class="px-4 py-2.5 border-b border-border-light dark:border-border-dark">
							<p class="text-xs font-semibold truncate text-text-light dark:text-text-dark">
								{$currentUser?.email || 'Benutzer'}
							</p>
							<p class="text-[11px] text-muted-light dark:text-muted-dark mt-0.5">
								{$currentUser?.isAdmin ? 'Administrator' : 'Standardbenutzer'}
							</p>
						</div>

						<div class="py-1">
							<a
								href="/settings"
								class="flex items-center gap-2.5 px-4 py-2 text-xs text-text-light dark:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors"
							>
								<IconUser size={16} stroke={1.75} class="text-muted-light dark:text-muted-dark" />
								<span>Profil & Sitzungen</span>
							</a>

							<a
								href="/settings/mfa"
								class="flex items-center gap-2.5 px-4 py-2 text-xs text-text-light dark:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors"
							>
								<IconShieldCheck size={16} stroke={1.75} class="text-accent" />
								<span>MFA Sicherheit</span>
							</a>
						</div>

						<div class="pt-1 border-t border-border-light dark:border-border-dark">
							<button
								type="button"
								onclick={handleLogout}
								class="w-full flex items-center gap-2.5 px-4 py-2 text-xs text-danger hover:bg-danger/10 transition-colors text-left cursor-pointer"
							>
								<IconLogout size={16} stroke={1.75} />
								<span>Abmelden</span>
							</button>
						</div>
					</Dropdown>
				</div>
			</header>


			<!-- Seiteninhalt (max 1200px zentriert) -->
			<main class="flex-1 overflow-y-auto">
				<div class="max-w-[1200px] mx-auto p-4 sm:p-6 lg:p-8 w-full">
					{@render children()}
				</div>
			</main>
		</div>
	</div>
{/if}
