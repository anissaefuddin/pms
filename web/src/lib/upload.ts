// Unggah berkas dengan progres nyata (D-05). fetch tidak bisa melaporkan progres unggah, jadi memakai XMLHttpRequest.
import { ApiError } from "@/lib/api";
export { progressPercent } from "@/lib/upload-progress";

export type UploadProgress = { loaded: number; total: number; computable: boolean };

/** Kirim FormData ke path. `onProgress` menerima progres apa adanya; jika `computable` salah, UI harus menampilkan indikator tak tentu. */
export function uploadWithProgress(path: string, body: FormData, onProgress: (progress: UploadProgress) => void): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("POST", path);
    request.withCredentials = true;
    request.upload.onprogress = (event) => onProgress({ loaded: event.loaded, total: event.total, computable: event.lengthComputable });
    request.onload = () => {
      let parsed: unknown;
      try { parsed = request.responseText ? JSON.parse(request.responseText) : undefined; } catch { parsed = undefined; }
      if (request.status >= 200 && request.status < 300) {
        resolve(parsed);
        return;
      }
      const problem = (parsed ?? {}) as { detail?: string; title?: string; code?: string };
      reject(new ApiError(problem.detail ?? problem.title ?? "Upload failed", request.status, problem.code));
    };
    request.onerror = () => reject(new ApiError("Upload failed: network error", 0));
    request.send(body);
  });
}
