import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminNotifications,
  useMarkNotificationRead,
  adminNotifsKeys,
} from "./admin-notifications";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) => mockAuthFetch(...args),
  ApiError: class extends Error {
    code: string;
    status: number;
    constructor(code: string, message: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

const sampleNotification = {
  id: "notif-1",
  type: "order_confirmed",
  order_id: "ord-1",
  student_name: "Budi Santoso",
  amount: 150000,
  created_at: "2026-07-05T10:00:00Z",
  read: false,
};

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}

describe("admin-notifications hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  describe("query keys", () => {
    it("adminNotifsKeys.all is stable", () => {
      expect(adminNotifsKeys.all).toEqual(["admin", "notifications"]);
    });

    it("adminNotifsKeys.list() returns default list key", () => {
      expect(adminNotifsKeys.list()).toEqual(["admin", "notifications", "list", "any"]);
    });

    it("adminNotifsKeys.list({}) returns list key without filters", () => {
      expect(adminNotifsKeys.list({})).toEqual(["admin", "notifications", "list", "any"]);
    });

    it("adminNotifsKeys.list({unreadOnly:true}) includes unreadOnly", () => {
      const key = adminNotifsKeys.list({ unreadOnly: true });
      expect(key).toEqual(["admin", "notifications", "list", "unread"]);
    });

    it("separates the unread key from the unfiltered one", () => {
      expect(adminNotifsKeys.list({ unreadOnly: true })).not.toEqual(
        adminNotifsKeys.list()
      );
    });
  });

  describe("useAdminNotifications", () => {
    it("fetches GET /admin/notifications and returns the flattened items", async () => {
      mockAuthFetch.mockResolvedValueOnce({
        data: [sampleNotification],
        next_cursor: "10",
      });

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAdminNotifications(), { wrapper });

      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/notifications");
      expect(result.current.items).toEqual([sampleNotification]);
      expect(result.current.hasNextPage).toBe(true);
    });

    it("appends unread_only query param when unreadOnly is true", async () => {
      mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: "" });

      const { wrapper } = wrapperFactory();
      renderHook(() => useAdminNotifications({ unreadOnly: true }), { wrapper });

      await waitFor(() =>
        expect(mockAuthFetch).toHaveBeenCalledWith("/admin/notifications?unread_only=true")
      );
    });

    it("reports no further page when the server omits the cursor", async () => {
      mockAuthFetch.mockResolvedValueOnce({ data: [sampleNotification], next_cursor: "" });

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAdminNotifications(), { wrapper });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.hasNextPage).toBe(false);
    });

    it("sends the cursor, with the filter, when fetching the next page", async () => {
      const older = { ...sampleNotification, id: "notif-2", student_name: "Siti" };
      mockAuthFetch
        .mockResolvedValueOnce({ data: [sampleNotification], next_cursor: "1750_notif-1" })
        .mockResolvedValueOnce({ data: [older], next_cursor: "" });

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAdminNotifications({ unreadOnly: true }), {
        wrapper,
      });

      await waitFor(() => expect(result.current.isLoading).toBe(false));

      await act(async () => {
        await result.current.fetchNextPage();
      });

      expect(mockAuthFetch).toHaveBeenLastCalledWith(
        "/admin/notifications?unread_only=true&cursor=1750_notif-1"
      );
      await waitFor(() =>
        expect(result.current.items).toEqual([sampleNotification, older])
      );
    });

    // The feed used to accumulate pages in component state, so re-running the
    // query after marking a row read appended the same page a second time.
    it("does not duplicate rows when the query refetches after paging", async () => {
      const older = { ...sampleNotification, id: "notif-2", student_name: "Siti" };
      mockAuthFetch
        .mockResolvedValueOnce({ data: [sampleNotification], next_cursor: "1750_notif-1" })
        .mockResolvedValueOnce({ data: [older], next_cursor: "" })
        // Refetch re-runs every page that has been loaded.
        .mockResolvedValueOnce({ data: [sampleNotification], next_cursor: "1750_notif-1" })
        .mockResolvedValueOnce({ data: [older], next_cursor: "" });

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAdminNotifications(), { wrapper });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      await act(async () => {
        await result.current.fetchNextPage();
      });
      await waitFor(() => expect(result.current.items).toHaveLength(2));

      await act(async () => {
        await result.current.refetch();
      });

      await waitFor(() =>
        expect(result.current.items.map((n) => n.id)).toEqual(["notif-1", "notif-2"])
      );
      expect(result.current.items).toHaveLength(2);
    });

    it("tolerates a null data array without crashing", async () => {
      mockAuthFetch.mockResolvedValueOnce({ data: null, next_cursor: "" });

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAdminNotifications(), { wrapper });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.items).toEqual([]);
    });
  });

  describe("useMarkNotificationRead", () => {
    it("calls PATCH /admin/notifications/:id/read and invalidates query", async () => {
      mockAuthFetch.mockResolvedValueOnce({ message: "notification marked read" });

      const { wrapper, queryClient } = wrapperFactory();
      const spy = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useMarkNotificationRead(), { wrapper });

      await act(async () => {
        await result.current.mutateAsync("notif-1");
      });

      expect(mockAuthFetch).toHaveBeenCalledWith("/admin/notifications/notif-1/read", {
        method: "PATCH",
      });
      expect(spy).toHaveBeenCalledWith({ queryKey: adminNotifsKeys.all });
    });
  });
});
