"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { Product, Order, CheckoutResult } from "@/lib/types";

// ── Query keys ────────────────────────────────────────────────────────────

export const bulkExamOrderKeys = {
  all: ["admin", "bulkExamOrders"] as const,
  orderableExams: () => [...bulkExamOrderKeys.all, "orderableExams"] as const,
  preview: (examId: string) => [...bulkExamOrderKeys.all, "preview", examId] as const,
};

// ── Types ─────────────────────────────────────────────────────────────────

export interface BulkOrderExcluded {
  student_id: string;
  name: string;
  reason: string;
}

// Matches backend service.BulkOrderPreview
// (backend/internal/service/bulk_exam_order.go:23-28)
export interface BulkExamOrderPreview {
  net_new_count: number;
  excluded?: BulkOrderExcluded[];
  unit_price: number;
  total: number;
}

export interface CreateBulkOrderInput {
  exam_id: string;
  student_ids: string[];
}

// ── Hooks ─────────────────────────────────────────────────────────────────

/**
 * Fetch exams that have a published product (can be ordered via bulk).
 * GET /admin/bulk-exam-orders/exams  (FR-BULK-01)
 */
export function useOrderableExams() {
  return useQuery({
    queryKey: bulkExamOrderKeys.orderableExams(),
    queryFn: () =>
      authFetch<{ data: Product[] }>("/admin/bulk-exam-orders/exams"),
  });
}

/**
 * Preview a bulk exam order: given an exam + student list, what will the
 * order look like.
 * POST /admin/bulk-exam-orders/preview
 */
export function usePreviewBulkExamOrder() {
  return useMutation({
    mutationFn: (input: CreateBulkOrderInput) =>
      authFetch<BulkExamOrderPreview>("/admin/bulk-exam-orders/preview", {
        method: "POST",
        body: JSON.stringify(input),
      }),
  });
}

/**
 * Create a bulk exam order.
 * POST /admin/bulk-exam-orders
 */
export function useCreateBulkExamOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateBulkOrderInput) =>
      authFetch<Order>("/admin/bulk-exam-orders", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bulkExamOrderKeys.all });
    },
  });
}
