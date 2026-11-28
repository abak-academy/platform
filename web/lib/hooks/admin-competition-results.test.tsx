import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useApplyCompetitionResult,
  useDeleteCompetitionResult,
  usePresignCompetitionResult,
  usePreviewCompetitionResult,
  useSetCompetitionResultPublished,
} from "./admin-competition-results";

const mockAuthFetch = vi.fn();
vi.mock("@/lib/api", () => ({ authFetch: (...a: unknown[]) => mockAuthFetch(...a) }));

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{children}</QueryClientProvider>
);

describe("competition result hooks", () => {
  it("presigns with the filename in the query and no body", async () => {
    mockAuthFetch.mockResolvedValueOnce({ url: "u", method: "PUT", key: "k" });
    const { result } = renderHook(() => usePresignCompetitionResult("p1"), { wrapper });
    await act(() => result.current.mutateAsync("hasil kompetisi.csv"));
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/p1/competition-result/presign?filename=hasil%20kompetisi.csv", {
      method: "POST",
    });
  });

  it("posts only the key to preview and apply", async () => {
    mockAuthFetch.mockResolvedValue({});
    const prev = renderHook(() => usePreviewCompetitionResult("p1"), { wrapper });
    await act(() => prev.result.current.mutateAsync("k"));
    const app = renderHook(() => useApplyCompetitionResult("p1"), { wrapper });
    await act(() => app.result.current.mutateAsync("k"));
    const body = JSON.stringify({ key: "k" });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/p1/competition-result/preview", { method: "POST", body });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/p1/competition-result/apply", { method: "POST", body });
  });

  it("publishes, unpublishes and deletes by exam id", async () => {
    mockAuthFetch.mockResolvedValue({});
    const pub = renderHook(() => useSetCompetitionResultPublished("p1"), { wrapper });
    await act(() => pub.result.current.mutateAsync(true));
    await act(() => pub.result.current.mutateAsync(false));
    const del = renderHook(() => useDeleteCompetitionResult("p1"), { wrapper });
    await act(() => del.result.current.mutateAsync());
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/p1/competition-result/publish", { method: "POST" });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/p1/competition-result/unpublish", { method: "POST" });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/p1/competition-result", { method: "DELETE" });
  });
});
