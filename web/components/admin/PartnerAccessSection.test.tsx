import { fireEvent, render, renderHook, screen, waitFor, act } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { PartnerAccessSection, usePartnerAccessDraft, type PartnerAccessDraft } from "./PartnerAccessSection";
import type { PartnerAccess } from "@/lib/types";

const hook = vi.hoisted(() => ({
  access: { isLoading: false, isError: false, data: undefined } as { isLoading: boolean; isError: boolean; data?: PartnerAccess },
  save: vi.fn(),
}));

vi.mock("@/lib/hooks/partner", () => ({
  usePartnerProductAccess: () => hook.access,
  useSetPartnerProductAccess: () => ({ mutateAsync: hook.save }),
}));
vi.mock("@/components/admin/PartnerPicker", () => ({
  PartnerPicker: ({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) => (
    <button type="button" onClick={() => onChange([...value, "mitra-1"])}>
      pilih ({value.length})
    </button>
  ),
}));

function draftOf(over: Partial<PartnerAccessDraft> = {}): PartnerAccessDraft {
  return { scope: "none", ids: [], setScope: vi.fn(), setIds: vi.fn(), isLoading: false, isError: false, dirty: false, valid: true, save: vi.fn(), ...over };
}

describe("usePartnerAccessDraft", () => {
  beforeEach(() => {
    hook.save.mockReset().mockResolvedValue({});
    hook.access = { isLoading: false, isError: false, data: { scope: "none", ids: [] } };
  });

  it("starts at what the server says and is clean until something changes", async () => {
    const { result } = renderHook(() => usePartnerAccessDraft("p1"));
    await waitFor(() => expect(result.current.scope).toBe("none"));
    expect(result.current.dirty).toBe(false);

    act(() => result.current.setScope("all"));
    expect(result.current.dirty).toBe(true);
    act(() => result.current.setScope("none"));
    expect(result.current.dirty).toBe(false);
  });

  it("needs a Mitra for 'selected', and sends the chosen ones only then", async () => {
    const { result } = renderHook(() => usePartnerAccessDraft("p1"));
    act(() => result.current.setScope("selected"));
    expect(result.current.valid).toBe(false);

    act(() => result.current.setIds(["mitra-1"]));
    expect(result.current.valid).toBe(true);
    await act(async () => {
      await result.current.save();
    });
    expect(hook.save).toHaveBeenCalledWith({ productId: "p1", access: { scope: "selected", ids: ["mitra-1"] } });
  });

  it("drops the chosen Mitra when the scope is not 'selected'", async () => {
    hook.access = { isLoading: false, isError: false, data: { scope: "selected", ids: ["mitra-9"] } };
    const { result } = renderHook(() => usePartnerAccessDraft("p1"));
    await waitFor(() => expect(result.current.scope).toBe("selected"));

    act(() => result.current.setScope("none"));
    await act(async () => {
      await result.current.save();
    });
    expect(hook.save).toHaveBeenCalledWith({ productId: "p1", access: { scope: "none", ids: [] } });
  });

  it("does not call the server while there is no product", () => {
    hook.access = { isLoading: false, isError: false, data: undefined };
    const { result } = renderHook(() => usePartnerAccessDraft(null));
    expect(result.current.dirty).toBe(false);
  });
});

describe("PartnerAccessSection", () => {
  it("shows the scope from the draft and edits it", () => {
    const draft = draftOf({ scope: "none" });
    render(<PartnerAccessSection draft={draft} />);
    expect(screen.getByRole("radio", { name: /tidak tampil/i })).toBeChecked();

    fireEvent.click(screen.getByRole("radio", { name: /mitra terpilih/i }));
    expect(draft.setScope).toHaveBeenCalledWith("selected");
  });

  it("asks for a Mitra when 'selected' has none, and has no button of its own to save", () => {
    render(<PartnerAccessSection draft={draftOf({ scope: "selected", valid: false })} />);
    expect(screen.getByText("Pilih minimal satu Mitra.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /simpan/i })).toBeNull();
  });

  it("shows loading and error states", () => {
    const loading = render(<PartnerAccessSection draft={draftOf({ isLoading: true })} />);
    expect(screen.getByText("Memuat...")).toBeInTheDocument();
    loading.unmount();

    render(<PartnerAccessSection draft={draftOf({ isError: true })} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat akses Mitra.");
  });
});
