"use client";

import { useMutation } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { PresignedUpload, EnqueueBulkResult } from "@/lib/hooks/admin-students-bulk";

export type { PresignedUpload, EnqueueBulkResult };
export { putFileToPresignedURL } from "@/lib/hooks/admin-students-bulk";

/**
 * Request a presigned MinIO PUT URL for a school-bulk CSV upload.
 * POST /admin/schools/bulk/presign?filename=&content_type=
 */
export function usePresignSchoolBulkUpload() {
  return useMutation({
    mutationFn: ({ filename, contentType }: { filename: string; contentType: string }) => {
      const qs = new URLSearchParams({ filename, content_type: contentType }).toString();
      return authFetch<PresignedUpload>(`/admin/schools/bulk/presign?${qs}`, {
        method: "POST",
      });
    },
  });
}

/**
 * Enqueue a school-bulk import job for an already-uploaded CSV.
 * POST /admin/schools/bulk {file_key} -> {job_id}
 */
export function useEnqueueSchoolBulkImport() {
  return useMutation({
    mutationFn: ({ fileKey }: { fileKey: string }) =>
      authFetch<EnqueueBulkResult>("/admin/schools/bulk", {
        method: "POST",
        body: JSON.stringify({ file_key: fileKey }),
      }),
  });
}
