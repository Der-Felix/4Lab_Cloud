import Uppy from '@uppy/core';
import Tus from '@uppy/tus';
import Dashboard from '@uppy/dashboard';
import { api } from './api';
import { writable } from 'svelte/store';

// Event-Store zur Benachrichtigung ueber erfolgreich abgeschlossene Uploads
export const uploadCompletedTrigger = writable<number>(0);

export function createUploader(targetSelector = '#uppy-dashboard') {
	const uppy = new Uppy({
		autoProceed: false,
		restrictions: {
			maxFileSize: 5_000_000_000, // 5 GB
			maxNumberOfFiles: 20
		}
	});

	// Asynchroner Pre-Processor zur Initialisierung jedes Uploads beim Go App-Server
	uppy.addPreProcessor(async (fileIDs) => {
		for (const fileId of fileIDs) {
			const file = uppy.getFile(fileId);
			if (!file) continue;

			try {
				const initResp = await api<{
					upload_id: string;
					presigned_url?: string;
					location?: string;
				}>('/uploads/init', {
					method: 'POST',
					body: JSON.stringify({
						filename: file.name,
						size_bytes: file.size
					})
				});

				const uploadId = initResp.upload_id;
				const uploadUrl = initResp.presigned_url || initResp.location || `/api/v1/uploads/tus/${uploadId}`;

				uppy.setFileState(fileId, {
					meta: { ...file.meta, upload_id: uploadId },
					tus: { uploadUrl }
				});
			} catch (err: unknown) {
				console.error('Fehler bei Upload-Init:', err);
				throw err;
			}
		}
	});

	uppy.use(Tus, {
		endpoint: '/api/v1/uploads/tus',
		chunkSize: 5 * 1024 * 1024, // 5 MiB Chunks gemaess Architektur
		retryDelays: [0, 1000, 3000, 5000],
		removeFingerprintOnSuccess: true
	});

	uppy.use(Dashboard, {
		inline: true,
		target: targetSelector,
		width: '100%',
		height: 380,
		hideProgressDetails: false,
		proudlyDisplayPoweredByUppy: false,
		theme: 'auto',
		note: 'Dateien bis zu 5 GB (verschlüsselt mit AES-256-GCM)'
	});

	// Nach erfolgreichem Chunk-Upload den Complete-Aufruf an Go senden
	uppy.on('upload-success', async (file, response) => {
		let uploadId = file?.meta?.upload_id as string | undefined;
		if (response && response.uploadURL) {
			const parts = response.uploadURL.split('/');
			const lastPart = parts[parts.length - 1];
			if (lastPart && lastPart.length > 10) {
				uploadId = lastPart;
			}
		}
		if (uploadId) {
			try {
				await api('/uploads/complete', {
					method: 'POST',
					body: JSON.stringify({
						upload_id: uploadId
					})
				});
				uploadCompletedTrigger.update((n) => n + 1);
			} catch (err: unknown) {
				console.error('Fehler beim Abschliessen des Uploads:', err);
			}
		}
	});

	return uppy;
}
