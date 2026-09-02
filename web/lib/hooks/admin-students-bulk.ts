"use client";

import { useMutation } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";

export interface PresignedUpload {
  url: string;
  method: string;
  key: string;
}

export interface EnqueueBulkResult {
  job_id: string;
}

/**
 * Request a presigned MinIO PUT URL for a student-bulk CSV upload.
 * POST /admin/students/bulk/presign?filename=&content_type=
 *
 * Returns the presigned URL, HTTP method, and the object key the caller must
 * send back to /admin/students/bulk once the file is uploaded.
 */
export function usePresignStudentBulkUpload() {
  return useMutation({
    mutationFn: ({ filename, contentType }: { filename: string; contentType: string }) => {
      const qs = new URLSearchParams({ filename, content_type: contentType }).toString();
      return authFetch<PresignedUpload>(`/admin/students/bulk/presign?${qs}`, {
        method: "POST",
      });
    },
  });
}

/**
 * Upload a file directly to a presigned MinIO URL via raw fetch.
 *
 * This MUST NOT use authFetch / the app's Authorization header: the presigned
 * URL already carries the access signature in the query string, and MinIO
 * rejects requests that include an unexpected Authorization header.
 */
export async function putFileToPresignedURL(
  url: string,
  file: Blob,
  contentType: string,
): Promise<void> {
  const res = await fetch(url, {
    method: "PUT",
    body: file,
    headers: { "Content-Type": contentType },
  });
  if (!res.ok) {
    throw new Error(`MinIO presigned PUT failed: HTTP ${res.status}`);
  }
}

/**
 * Enqueue a student-bulk import job for an already-uploaded CSV.
 * POST /admin/students/bulk {file_key, temp_password?} -> {job_id}
 *
 * tempPassword, when provided, becomes the shared temporary password for every
 * row without a CSV password value; the backend forces a change at first login.
 */
export function useEnqueueStudentBulkImport() {
  return useMutation({
    mutationFn: ({ fileKey, tempPassword }: { fileKey: string; tempPassword?: string }) =>
      authFetch<EnqueueBulkResult>("/admin/students/bulk", {
        method: "POST",
        body: JSON.stringify(
          tempPassword ? { file_key: fileKey, temp_password: tempPassword } : { file_key: fileKey },
        ),
      }),
  });
}
