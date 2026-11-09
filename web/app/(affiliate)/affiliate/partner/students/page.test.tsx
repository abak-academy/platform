import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import PartnerStudentsPage from "./page";
import { ApiError } from "@/lib/api";
import type { PartnerStudent } from "@/lib/types";

const hook = vi.hoisted(() => ({
  state: { isLoading: false, isError: false, data: undefined, hasNextPage: false, isFetchingNextPage: false } as Record<string, unknown>,
  fetchNextPage: vi.fn(),
  refetch: vi.fn(),
  registerMutate: vi.fn(),
  reissueMutate: vi.fn(),
  studentParams: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
  fetchAll: vi.fn(),
  downloadCsv: vi.fn(),
}));

vi.mock("@/lib/hooks/partner", () => ({
  usePartnerStudentsPage: (params: unknown) => {
    hook.studentParams(params);
    return { ...hook.state, refetch: hook.refetch };
  },
  usePartnerSchools: () => ({ data: { data: [{ id: "sch-1", name: "SMA Pilihan", code: "P1", npsn: "123", status: "active" }] } }),
  fetchAllPartnerPages: (...a: unknown[]) => hook.fetchAll(...a),
  useRegisterPartnerStudent: () => ({ mutate: hook.registerMutate, isPending: false }),
  useReissuePartnerCredentials: () => ({ mutate: hook.reissueMutate, isPending: false, data: undefined, reset: vi.fn() }),
}));
vi.mock("@/components/admin/BulkImportModal", () => ({
  BulkImportModal: ({ open, partner }: { open: boolean; partner?: boolean }) => (open ? <div data-testid="bulk-modal" data-partner={String(partner)} /> : null),
}));
vi.mock("@/components/SchoolPicker", () => ({ SchoolPicker: () => <div /> }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: hook.replace }),
  usePathname: () => "/affiliate/partner/students",
  useSearchParams: () => new URLSearchParams(hook.search.value),
}));
vi.mock("@/lib/csv", () => ({ downloadCsv: (...a: unknown[]) => hook.downloadCsv(...a) }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function student(over: Partial<PartnerStudent>): PartnerStudent {
  return {
    id: "s1",
    name: "Ani Siswa",
    username: "ani.siswa",
    school_name: "SMA Contoh",
    grade: 11,
    jenjang: "SMA",
    status: "active",
    activated: true,
    registered_by_me: false,
    created_at: "2026-11-01T00:00:00Z",
    ...over,
  };
}

function setPages(pages: { data: PartnerStudent[]; hint?: string }[], extra: Record<string, unknown> = {}) {
  const rows = pages.flatMap((p) => p.data);
  hook.state = { isLoading: false, isError: false, data: { data: rows, page: 1, per_page: 20, total: rows.length, hint: pages[0]?.hint }, ...extra };
}

describe("PartnerStudentsPage", () => {
  beforeEach(() => {
    hook.search.value = "";
    hook.studentParams.mockClear();
    hook.registerMutate.mockClear();
    hook.reissueMutate.mockClear();
  });

  it("lists the safe columns and offers a new password only for a student the Mitra registered who has not activated", () => {
    setPages([
      {
        data: [
          student({ id: "a", name: "Ani Aktif", registered_by_me: true, activated: true }),
          student({ id: "b", name: "Budi Baru", registered_by_me: true, activated: false }),
          student({ id: "c", name: "Cici Sekolah", registered_by_me: false, activated: false }),
        ],
      },
    ]);
    render(<PartnerStudentsPage />);

    expect(screen.getByText("Budi Baru")).toBeInTheDocument();
    expect(screen.getAllByText("SMA Contoh")).toHaveLength(3);
    expect(screen.getAllByText("Belum ganti kata sandi")).toHaveLength(2);
    expect(screen.getAllByRole("button", { name: "Kata sandi baru" })).toHaveLength(1);
    expect(screen.queryByText(/@/)).toBeNull();
  });

  it("says who registered each student", () => {
    setPages([{ data: [student({ id: "a", name: "Ani Aktif", registered_by_me: true }), student({ id: "c", name: "Cici Sekolah", registered_by_me: false })] }]);
    render(<PartnerStudentsPage />);
    expect(screen.getByText("Anda")).toBeInTheDocument();
    expect(screen.getByText("Bukan Anda")).toBeInTheDocument();
  });

  it("asks for the page, search and filters the URL holds", () => {
    hook.search.value = "q=ani&page=2&per_page=10&type=mine&school=sch-1&status=deactivated";
    setPages([{ data: [student({})] }]);
    render(<PartnerStudentsPage />);

    expect(hook.studentParams).toHaveBeenLastCalledWith({
      q: "ani",
      status: "deactivated",
      school_id: "sch-1",
      registered_by_me: true,
      page: 2,
      per_page: 10,
    });
  });

  it("downloads every student that matches the filters, not only the loaded page", async () => {
    setPages([{ data: [student({})] }]);
    hook.fetchAll.mockResolvedValue([student({ name: "Ani A" }), student({ id: "s2", name: "Budi B", registered_by_me: true })]);
    render(<PartnerStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));

    await waitFor(() => expect(hook.downloadCsv).toHaveBeenCalled());
    expect(hook.fetchAll).toHaveBeenCalledWith("/partners/me/students", expect.objectContaining({ registered_by_me: false }));
    const [, , rows] = hook.downloadCsv.mock.calls[0];
    expect(rows).toHaveLength(2);
    expect(rows[1]).toContain("Budi B");
  });

  it("explains an empty list that is empty because no school is assigned", () => {
    setPages([{ data: [], hint: "no_schools" }]);
    render(<PartnerStudentsPage />);
    expect(screen.getByText(/belum ada sekolah yang ditugaskan/i)).toBeInTheDocument();
  });

  it("says plainly when there are simply no students", () => {
    setPages([{ data: [] }]);
    render(<PartnerStudentsPage />);
    expect(screen.getByText("Belum ada siswa.")).toBeInTheDocument();
  });

  it("has loading and error with retry states", () => {
    hook.state = { isLoading: true, isError: false, data: undefined };
    const loading = render(<PartnerStudentsPage />);
    expect(screen.queryByText("Belum ada siswa.")).toBeNull();
    loading.unmount();

    hook.state = { isLoading: false, isError: true, data: undefined };
    render(<PartnerStudentsPage />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /coba lagi/i }));
    expect(hook.refetch).toHaveBeenCalled();
  });

  it("opens the CSV import in Mitra mode", () => {
    setPages([{ data: [] }]);
    render(<PartnerStudentsPage />);
    fireEvent.click(screen.getByRole("button", { name: /impor csv/i }));
    expect(screen.getByTestId("bulk-modal")).toHaveAttribute("data-partner", "true");
  });

  it("registers a student and shows the temporary password once", async () => {
    setPages([{ data: [] }]);
    hook.registerMutate.mockImplementation((_input, opts) => opts.onSuccess({ id: "n", name: "Dedi", username: "dedi.baru", temp_password: "Rahasia-123" }));
    render(<PartnerStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));
    fireEvent.change(await screen.findByLabelText("Nama lengkap"), { target: { value: "Dedi" } });
    fireEvent.click(screen.getByRole("combobox", { name: "Jenjang" }));
    fireEvent.click(await screen.findByRole("option", { name: "SMA" }));
    fireEvent.click(screen.getByRole("button", { name: "Daftarkan" }));

    await waitFor(() => expect(hook.registerMutate).toHaveBeenCalled());
    expect(hook.registerMutate.mock.calls[0][0]).toEqual({ name: "Dedi", jenjang: "SMA" });
    expect(await screen.findByText("Rahasia-123")).toBeInTheDocument();
    expect(screen.getByText("dedi.baru")).toBeInTheDocument();
    expect(screen.getByText(/hanya ditampilkan sekali/i)).toBeInTheDocument();
  });

  it("does not send a registration without a name and a level", () => {
    setPages([{ data: [] }]);
    render(<PartnerStudentsPage />);
    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));
    expect(screen.getByRole("button", { name: "Daftarkan" })).toBeDisabled();
  });

  it("asks before reissuing and reports a student who has already activated", async () => {
    setPages([{ data: [student({ id: "b", name: "Budi Baru", registered_by_me: true, activated: false })] }]);
    hook.reissueMutate.mockImplementation((_id, opts) => opts.onError(new ApiError("student_already_activated", "x", 409)));
    render(<PartnerStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Kata sandi baru" }));
    expect(await screen.findByText(/kata sandi yang lama tidak berlaku lagi/i)).toBeInTheDocument();
    expect(hook.reissueMutate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Buat kata sandi baru" }));
    await waitFor(() => expect(hook.reissueMutate).toHaveBeenCalledWith("b", expect.anything()));
  });
});
