import type * as LeafletType from 'leaflet';

export interface MapModule {
	L: typeof LeafletType;
}

// Dynamischer Client-Import fuer Leaflet und MarkerCluster (vermeidet SSR window errors)
export async function loadLeaflet(): Promise<typeof LeafletType> {
	if (typeof window === 'undefined') {
		throw new Error('Leaflet kann nur im Browser (Client) geladen werden');
	}

	const L = (await import('leaflet')).default;
	// @ts-ignore - markercluster erweitert L
	await import('leaflet.markercluster');

	// Lokale Icons konfigurieren (Kein CDN / kein unpkg)
	delete (L.Icon.Default.prototype as any)._getIconUrl;
	L.Icon.Default.mergeOptions({
		iconUrl: '/assets/leaflet/marker-icon.svg',
		iconRetinaUrl: '/assets/leaflet/marker-icon-2x.svg',
		shadowUrl: '/assets/leaflet/marker-shadow.svg',
		iconSize: [25, 41],
		iconAnchor: [12, 41],
		popupAnchor: [1, -34],
		shadowSize: [41, 41]
	});

	return L;
}

// Erstellt ein benutzerdefiniertes Teal-Marker-Icon fuer Fotos
export function createTealMarkerIcon(L: typeof LeafletType) {
	return L.icon({
		iconUrl: '/assets/leaflet/marker-icon.svg',
		iconRetinaUrl: '/assets/leaflet/marker-icon-2x.svg',
		shadowUrl: '/assets/leaflet/marker-shadow.svg',
		iconSize: [25, 41],
		iconAnchor: [12, 41],
		popupAnchor: [1, -34],
		shadowSize: [41, 41]
	});
}

// Standard OpenStreetMap Kachel-Layer mit Pflicht-Attribution nach DSGVO/OSM
export function createOsmTileLayer(L: typeof LeafletType) {
	return L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
		maxZoom: 19,
		attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
	});
}
