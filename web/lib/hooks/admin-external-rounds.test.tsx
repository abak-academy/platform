import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useApplyExternalRound,
  useDeleteExternalRound,
  usePresignExternalRound,
  usePreviewExternalRound,
  useSetExternalRoundPublished,
} from "./admin-external-rounds";

const mockAuthFetch = vi.fn();
vi.mock("@/lib/api", () => ({ authFetch: (...a: unknown[]) => mockAuthFetch(...a) }));

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{children}</QueryClientProvider>
);

describe("external round hooks", () => {
  it("presigns with an encoded filename", async () => {
    mockAuthFetch.mockResolvedValueOnce({ url: "u", method: "PUT", key: "k" });
    const { result } = renderHook(() => usePresignExternalRound("e1"), { wrapper });
    await act(() => result.current.mutateAsync("hasil ronde.csv"));
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/e1/external-rounds/presign?filename=hasil%20ronde.csv", { method: "POST" });
  });

  it("posts key and round_label to preview and apply", async () => {
    mockAuthFetch.mockResolvedValue({});
    const prev = renderHook(() => usePreviewExternalRound("e1"), { wrapper });
    await act(() => prev.result.current.mutateAsync({ key: "k", round_label: "R1" }));
    const app = renderHook(() => useApplyExternalRound("e1"), { wrapper });
    await act(() => app.result.current.mutateAsync({ key: "k", round_label: "R1" }));
    const body = JSON.stringify({ key: "k", round_label: "R1" });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/e1/external-rounds/preview", { method: "POST", body });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/exams/e1/external-rounds/apply", { method: "POST", body });
  });

  it("publishes, unpublishes and deletes by round id", async () => {
    mockAuthFetch.mockResolvedValue({});
    const pub = renderHook(() => useSetExternalRoundPublished(), { wrapper });
    await act(() => pub.result.current.mutateAsync({ roundId: "r1", publish: true }));
    await act(() => pub.result.current.mutateAsync({ roundId: "r1", publish: false }));
    const del = renderHook(() => useDeleteExternalRound(), { wrapper });
    await act(() => del.result.current.mutateAsync("r1"));
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/external-rounds/r1/publish", { method: "POST" });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/external-rounds/r1/unpublish", { method: "POST" });
    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/external-rounds/r1", { method: "DELETE" });
  });
});
