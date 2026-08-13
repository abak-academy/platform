"use client";

import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";

export interface AdminNotification {
  id: string;
  type: string;
  order_id: string;
  student_name: string;
  amount: number;
  created_at: string;
  read: boolean;
}

export interface AdminNotificationsResponse {
  data: AdminNotification[] | null;
  next_cursor?: string;
}

export interface AdminNotifsFilters {
  unreadOnly?: boolean;
}

export const adminNotifsKeys = {
  all: ["admin", "notifications"] as const,
  list: (filters?: AdminNotifsFilters) =>
    [...adminNotifsKeys.all, "list", filters?.unreadOnly ? "unread" : "any"] as const,
};

function buildNotifsQuery(cursor: string, filters?: AdminNotifsFilters): string {
  const params = new URLSearchParams();
  if (filters?.unreadOnly) params.set("unread_only", "true");
  if (cursor) params.set("cursor", cursor);
  const qs = params.toString();
  return qs ? `/admin/notifications?${qs}` : "/admin/notifications";
}

// Paging lives in react-query rather than in component state: the feed used to
// accumulate pages by hand, and marking one row read re-ran the fetch and
// appended the same page a second time.
export function useAdminNotifications(filters?: AdminNotifsFilters) {
  const query = useInfiniteQuery({
    queryKey: adminNotifsKeys.list(filters),
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      authFetch<AdminNotificationsResponse>(buildNotifsQuery(pageParam, filters)),
    // The server omits the cursor on the final page, which is what ends this.
    getNextPageParam: (last) => last.next_cursor || undefined,
  });

  const items = query.data?.pages.flatMap((page) => page.data ?? []) ?? [];

  return {
    items,
    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    hasNextPage: query.hasNextPage,
    isFetchingNextPage: query.isFetchingNextPage,
    fetchNextPage: query.fetchNextPage,
    refetch: query.refetch,
  };
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(`/admin/notifications/${encodeURIComponent(id)}/read`, {
        method: "PATCH",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminNotifsKeys.all });
    },
  });
}
