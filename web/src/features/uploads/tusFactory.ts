import { DetailedError, Upload } from "tus-js-client";
import type { UploadFactory } from "./UploadQueue";

const chunkSize = 64 * 1024 * 1024;
const retryDelays = [0, 1000, 3000, 5000, 10000, 20000];

export const tusFactory = createTusFactory("/api/v1/uploads");

export function createTusFactory(endpoint: string): UploadFactory {
  return (file, folder, callbacks, skipIdentical = false) => {
    const conflict = skipIdentical ? "skip-identical" : "rename";
    const upload = new Upload(file, {
      endpoint,
      chunkSize,
      retryDelays,
      removeFingerprintOnSuccess: true,
      onShouldRetry: (error) => {
        const status = error instanceof DetailedError ? (error.originalResponse?.getStatus() ?? 0) : 0;
        if (status === 409 && error instanceof DetailedError) {
          if (error.originalRequest?.getMethod() === "HEAD") return false;
          try {
            const body = JSON.parse(error.originalResponse?.getBody() ?? "") as { error?: { message?: string } };
            if (body.error?.message === "already exists") return false;
          } catch {
            return true;
          }
        }
        return status === 0 || status === 409 || status === 423 || status === 429 || status >= 500;
      },
      metadata: { filename: file.name, folder, conflict, filetype: file.type || "application/octet-stream" },
      onProgress: (sent) => callbacks.onProgress(sent),
      onSuccess: () => callbacks.onSuccess(),
      onError: (error) => callbacks.onError(describe(error)),
    });

    let ready = false;
    let preparing: Promise<void> | undefined;
    let active = false;
    return {
      start: () => {
        active = true;
        if (ready) {
          upload.start();
          return;
        }
        preparing ??= upload.findPreviousUploads().then((previous) => {
          const last = previous.filter((item) => item.metadata.folder === folder && item.metadata.filename === file.name && (item.metadata.conflict ?? "rename") === conflict).at(-1);
          if (last) upload.resumeFromPreviousUpload(last);
          ready = true;
          if (active) upload.start();
        }).catch(() => {
          preparing = undefined;
          if (active) callbacks.onError("Could not resume the upload. Try again.");
        });
      },
      pause: () => { active = false; void upload.abort(false); },
      cancel: () => { active = false; void upload.abort(true); },
    };
  };
}

function describe(error: Error): string {
  if (error instanceof DetailedError && error.originalResponse) {
    try {
      const body = JSON.parse(error.originalResponse.getBody()) as { error?: { message?: string } };
      if (body.error?.message) return body.error.message.charAt(0).toUpperCase() + body.error.message.slice(1) + ".";
    } catch {
      return `Upload failed with status ${error.originalResponse.getStatus()}.`;
    }
  }
  return navigator.onLine ? "Upload failed. Try again." : "You are offline. Resume when the connection is back.";
}
