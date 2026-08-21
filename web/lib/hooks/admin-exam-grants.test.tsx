import { describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSearchStudentsAcrossSchools } from "./admin-exam-grants";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) => mockAuthFetch(...args),
}));

it("passes exam context and pagination to cross-school search", async () => {
  mockAuthFetch.mockResolvedValueOnce({ data: [], next_cursor: undefined });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );

  renderHook(
    () =>
      useSearchStudentsAcrossSchools({
        examId: "exam-1",
        cursor: "cursor-1",
        limit: 20,
        q: "budi",
      }),
    { wrapper },
  );

  await waitFor(() =>
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/exam-grants/students/search?q=budi&exam_id=exam-1&cursor=cursor-1&limit=20",
    ),
  );
});
