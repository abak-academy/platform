import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { formatRupiah } from "@/lib/format";
import { ExamRegistrationsTab } from "./ExamRegistrationsTab";

// ── Mock state ──────────────────────────────────────────────────────────

const previewMutateSpy = vi.fn();
const createMutateSpy = vi.fn();
let previewShouldError = false;
let previewMockResult:
  | {
      net_new_count: number;
      excluded: { student_id: string; name: string; reason: string }[];
      unit_price: number;
      total: number;
    }
  | null = null;
let createShouldError = false;
let createErrorMessage = "";

const grantMutateSpy = vi.fn();
let grantShouldError = false;
let grantErrorMessage = "";
let grantMockResult: any = null;

let authUser: { role: string; school_id?: string } = {
  role: "admin_school",
  school_id: "school-1",
};

vi.mock("@/lib/hooks/admin-bulk-exam-orders", () => ({
  usePreviewBulkExamOrder: () => {
    const { useState } = require("react");
    const [data, setData]: [any, (v: any) => void] = useState(undefined);
    return {
      data,
      isPending: false,
      isError: false,
      reset: () => setData(undefined),
      mutate: (input: any, opts?: any) => {
        previewMutateSpy(input, opts);
        if (previewShouldError) {
          opts?.onError?.(new Error("preview failed"));
        } else {
          setData(previewMockResult);
        }
      },
    };
  },
  useCreateBulkExamOrder: () => ({
    isPending: false,
    reset: () => {},
    mutate: (input: any, opts?: any) => {
      createMutateSpy(input, opts);
      if (createShouldError) {
        opts?.onError?.(new Error(createErrorMessage));
      } else {
        opts?.onSuccess?.({ id: "order-1" });
      }
    },
  }),
}));

let rosterData: { data: unknown[] } | undefined = { data: [] };
let rosterIsLoading = false;
let rosterIsError = false;

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExamRoster: () => ({
    data: rosterData,
    isLoading: rosterIsLoading,
    isError: rosterIsError,
  }),
}));

vi.mock("@/lib/hooks/admin-exam-grants", () => ({
  useGrantExamAccess: () => ({
    isPending: false,
    isError: false,
    reset: vi.fn(),
    mutate: (input: any, opts?: any) => {
      grantMutateSpy(input, opts);
      if (grantShouldError) {
        opts?.onError?.(new Error(grantErrorMessage));
      } else {
        opts?.onSuccess?.(grantMockResult);
      }
    },
  }),
}));

vi.mock("@/components/admin/ParticipantPicker", () => ({
  ParticipantPicker: ({ onChange }: { selected: string[]; onChange: (ids: string[]) => void }) => (
    <button
      data-testid="participant-add"
      onClick={() => onChange(["student-1", "student-2"])}
    >
      pick
    </button>
  ),
}));

vi.mock("@/components/cart/SnapCheckout", () => ({
  SnapCheckout: () => <div data-testid="snap-checkout" />,
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: any) => sel({ user: authUser }),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({
    lang: "id",
    t: (key: string) => key,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { toast } from "sonner";

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("ExamRegistrationsTab — admin_school order flow (no exam picker, exam is fixed by tab context)", () => {
  beforeEach(() => {
    authUser = { role: "admin_school", school_id: "school-1" };
    previewMutateSpy.mockClear();
    createMutateSpy.mockClear();
    previewShouldError = false;
    previewMockResult = null;
    createShouldError = false;
    createErrorMessage = "";
    rosterData = { data: [] };
    rosterIsLoading = false;
    rosterIsError = false;
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
  });

  it("does not render a grant button for admin_school", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    fireEvent.click(screen.getByTestId("participant-add"));
    expect(screen.queryByText("exam_grant_grant")).not.toBeInTheDocument();
  });

  it("previews and creates an order end-to-end, showing the real total", async () => {
    previewMockResult = {
      net_new_count: 2,
      excluded: [],
      unit_price: 75000,
      total: 150000,
    };

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByTestId("participant-add"));

    const previewButton = await screen.findByText("bulk_exam_order_preview");
    fireEvent.click(previewButton);

    expect(previewMutateSpy).toHaveBeenCalledWith(
      { exam_id: "exam-1", student_ids: ["student-1", "student-2"] },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(screen.getByText(formatRupiah(150000))).toBeInTheDocument();
    });

    const confirmButton = await screen.findByText("bulk_exam_order_confirm");
    fireEvent.click(confirmButton);

    expect(createMutateSpy).toHaveBeenCalledWith(
      { exam_id: "exam-1", student_ids: ["student-1", "student-2"] },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(screen.getByText("bulk_exam_order_created")).toBeInTheDocument();
    });
    expect(screen.getByText(/Tryout UTBK 2026/)).toBeInTheDocument();
  });

  it("shows an error toast when the backend rejects duplicate participant_ids on create", async () => {
    previewMockResult = {
      net_new_count: 2,
      excluded: [],
      unit_price: 75000,
      total: 150000,
    };
    createShouldError = true;
    createErrorMessage = "duplicate student_id in participant selector: student-1";

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByTestId("participant-add"));

    const previewButton = await screen.findByText("bulk_exam_order_preview");
    fireEvent.click(previewButton);

    const confirmButton = await screen.findByText("bulk_exam_order_confirm");
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "duplicate student_id in participant selector: student-1",
      );
    });
    expect(screen.queryByText("bulk_exam_order_created")).not.toBeInTheDocument();
  });
});

describe("ExamRegistrationsTab — super_admin grant flow (cross-school, no order/payment)", () => {
  beforeEach(() => {
    authUser = { role: "super_admin" };
    grantMutateSpy.mockClear();
    grantShouldError = false;
    grantErrorMessage = "";
    grantMockResult = null;
    rosterData = { data: [] };
    rosterIsLoading = false;
    rosterIsError = false;
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
  });

  it("does not render the order/preview button for super_admin", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    fireEvent.click(screen.getByTestId("participant-add"));
    expect(screen.queryByText("bulk_exam_order_preview")).not.toBeInTheDocument();
  });

  it("shows granted student names and usernames after a successful grant", async () => {
    grantMockResult = {
      granted_count: 2,
      granted_students: [
        { id: "s1", name: "Andi Saputra", username: "andi123" },
        { id: "s2", name: "Budi Santoso", username: "budi456" },
      ],
    };

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByTestId("participant-add"));

    const grantButton = await screen.findByText("exam_grant_grant");
    fireEvent.click(grantButton);

    expect(grantMutateSpy).toHaveBeenCalledWith(
      { exam_id: "exam-1", student_ids: ["student-1", "student-2"] },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(screen.getByText("Andi Saputra")).toBeInTheDocument();
    });
    expect(screen.getByText("@andi123")).toBeInTheDocument();
    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText("@budi456")).toBeInTheDocument();
  });

  it("shows an error toast when the backend rejects duplicate student_ids", async () => {
    grantShouldError = true;
    grantErrorMessage = "duplicate student_id in participant selector: s1";

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByTestId("participant-add"));

    const grantButton = await screen.findByText("exam_grant_grant");
    fireEvent.click(grantButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "duplicate student_id in participant selector: s1",
      );
    });
    expect(screen.queryByText("Andi Saputra")).not.toBeInTheDocument();
  });
});

describe("ExamRegistrationsTab — participant roster (FR-32)", () => {
  const originalCreateElement = document.createElement.bind(document);
  let lastDownloadedFilename: string | null = null;
  let lastCapturedBlob: Blob | null = null;

  beforeEach(() => {
    authUser = { role: "admin_school", school_id: "school-1" };
    rosterData = { data: [] };
    rosterIsLoading = false;
    rosterIsError = false;
    lastDownloadedFilename = null;
    lastCapturedBlob = null;

    document.createElement = ((tag: string) => {
      const el = originalCreateElement(tag);
      if (tag === "a") {
        (el as HTMLAnchorElement).click = vi.fn(function (this: HTMLAnchorElement) {
          lastDownloadedFilename = this.download;
        });
      }
      return el;
    }) as typeof document.createElement;

    if (!(URL.createObjectURL as any).__mocked) {
      URL.createObjectURL = vi.fn().mockImplementation((blob: Blob) => {
        lastCapturedBlob = blob;
        return "blob:mock" as unknown as string;
      }) as typeof URL.createObjectURL;
      (URL.createObjectURL as any).__mocked = true;
    }
  });

  const rows = [
    {
      registration_id: "reg-2",
      student_id: "s2",
      student_name: "Budi Santoso",
      student_username: "budi456",
      participant_number: 2,
      participant_no: "250620-0042-000002",
      status: "registered",
      checked_in_at: null,
    },
    {
      registration_id: "reg-1",
      student_id: "s1",
      student_name: "Andi Saputra",
      student_username: "andi123",
      participant_number: 1,
      participant_no: "250620-0042-000001",
      status: "registered",
      checked_in_at: "2026-06-20T01:00:00Z",
    },
    {
      registration_id: "reg-3",
      student_id: "s3",
      student_name: "Citra Dewi",
      student_username: null,
      participant_number: null,
      participant_no: "",
      status: "registered",
      checked_in_at: null,
    },
  ];

  it("shows the load-failed message when the roster fails to fetch", () => {
    rosterIsError = true;
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.getByText("exam_roster_load_failed")).toBeInTheDocument();
  });

  it("shows the empty state when there are no registrations", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.getByText("exam_roster_empty")).toBeInTheDocument();
  });

  it("renders rows sorted by participant number ascending by default, with a nil-safe dash for unassigned numbers", () => {
    rosterData = { data: rows };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const cells = screen.getAllByTestId("roster-participant-no");
    expect(cells.map((c) => c.textContent)).toEqual([
      "250620-0042-000001",
      "250620-0042-000002",
      "—",
    ]);
  });

  it("reverses sort order when the No. Peserta header is clicked", () => {
    rosterData = { data: rows };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByText("exam_roster_th_participant_no"));

    const cells = screen.getAllByTestId("roster-participant-no");
    expect(cells.map((c) => c.textContent)).toEqual([
      "—",
      "250620-0042-000002",
      "250620-0042-000001",
    ]);
  });

  it("exports a CSV blob of the roster rows when Export CSV is clicked", async () => {
    rosterData = { data: rows };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByText("exam_roster_export_csv"));

    expect(lastDownloadedFilename).toBe("roster.csv");
    expect(lastCapturedBlob).not.toBeNull();
    const text = await lastCapturedBlob!.text();
    expect(text).toContain("No. Peserta,Nama,Username,Status,Checked In");
    expect(text).toContain("250620-0042-000001,Andi Saputra,andi123,registered,yes");
    expect(text).toContain(",Citra Dewi,,registered,no");
  });

  // Student names are attacker-supplied at registration, so a leading =/+/-/@
  // must not reach the spreadsheet as a live formula.
  it("neutralizes formula-leading fields in the exported CSV", async () => {
    rosterData = {
      data: [
        {
          ...rows[0],
          registration_id: "reg-evil",
          student_id: "s-evil",
          student_name: '=HYPERLINK("http://evil.test","claim prize")',
          student_username: "+cmd|' /C calc'!A0",
        },
      ],
    };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByText("exam_roster_export_csv"));

    const text = await lastCapturedBlob!.text();
    expect(text).toContain(`"'=HYPERLINK(""http://evil.test"",""claim prize"")"`);
    expect(text).toContain(`"'+cmd|' /C calc'!A0"`);
    expect(text).not.toContain(",=HYPERLINK");
    expect(text).not.toContain(",+cmd");
  });
});
