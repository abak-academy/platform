import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, fireEvent, within } from "@testing-library/react";
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
      excluded?: { student_id: string; name: string; reason: string }[];
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
let holdGrantMutation = false;
let heldGrantOptions: any = null;

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
          opts?.onSuccess?.(previewMockResult);
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

let rosterData: { data: unknown[]; next_cursor?: string } | undefined = { data: [] };
let rosterIsLoading = false;
let rosterIsError = false;
const rosterHookSpy = vi.fn();
const exportRosterSpy = vi.fn().mockResolvedValue(undefined);
let resolveRosterData: (filter: { cursor?: string; limit: number; sort: "asc" | "desc" }) =>
  | { data: unknown[]; next_cursor?: string }
  | undefined = () => rosterData;

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExamRoster: (examId: string, filter: { cursor?: string; limit: number; sort: "asc" | "desc" }) => {
    rosterHookSpy(examId, filter);
    return {
      data: resolveRosterData(filter),
      isLoading: rosterIsLoading,
      isFetching: rosterIsLoading,
      isError: rosterIsError,
    };
  },
  exportExamRoster: (...args: unknown[]) => exportRosterSpy(...args),
  adminExamsKeys: {
    all: ["adminExams"] as const,
    rosters: () => ["adminExams", "roster"] as const,
  },
}));

const csvPresignMutateAsync = vi.fn();
const csvEnqueueMutateAsync = vi.fn();
const csvPutFile = vi.fn();

const csvJobStatusState: {
  data: {
    id: string;
    type: string;
    status: string;
    progress: number;
    result_url: string | null;
    error: string | null;
    created_at: string;
    updated_at: string;
  } | null;
} = { data: null };

vi.mock("@/lib/hooks/admin-exam-grants", () => ({
  useGrantExamAccess: () => ({
    isPending: false,
    isError: false,
    reset: vi.fn(),
    mutate: (input: any, opts?: any) => {
      grantMutateSpy(input, opts);
      if (holdGrantMutation) {
        heldGrantOptions = opts;
        return;
      }
      if (grantShouldError) {
        opts?.onError?.(new Error(grantErrorMessage));
      } else {
        opts?.onSuccess?.(grantMockResult);
      }
    },
  }),
  usePresignExamGrantBulkUpload: (examId: string) => ({
    mutateAsync: (args: unknown) => csvPresignMutateAsync(examId, args),
    isPending: false,
  }),
  useEnqueueExamGrantBulk: () => ({
    mutateAsync: csvEnqueueMutateAsync,
    isPending: false,
  }),
}));

vi.mock("@/lib/hooks/admin-students-bulk", () => ({
  putFileToPresignedURL: (...args: Parameters<typeof csvPutFile>) => csvPutFile(...args),
}));

vi.mock("@/lib/hooks/jobs", () => ({
  useJobStatus: () => ({
    data: csvJobStatusState.data,
    isLoading: false,
    isError: false,
    error: null,
  }),
}));

vi.mock("@/components/admin/ParticipantPicker", () => ({
  ParticipantPicker: ({ examId, onChange }: { examId: string; selected: string[]; onChange: (ids: string[]) => void }) => (
    <button
      data-testid="participant-add"
      data-exam-id={examId}
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

beforeEach(() => {
  rosterData = { data: [] };
  rosterIsLoading = false;
  rosterIsError = false;
  rosterHookSpy.mockClear();
  exportRosterSpy.mockClear();
  resolveRosterData = () => rosterData;
  csvPresignMutateAsync.mockReset();
  csvEnqueueMutateAsync.mockReset();
  csvPutFile.mockReset();
  csvJobStatusState.data = null;
  holdGrantMutation = false;
  heldGrantOptions = null;
});

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function openGrantModal() {
  fireEvent.click(screen.getByTestId("open-grant-modal"));
  return within(screen.getByRole("dialog"));
}

function openOrderModal() {
  fireEvent.click(screen.getByTestId("open-order-modal"));
  return within(screen.getByRole("dialog"));
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

  it("shows the roster only on initial render — no ParticipantPicker, no dialog", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.queryByTestId("participant-add")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByTestId("open-order-modal")).toBeInTheDocument();
  });

  it("opens a dialog containing the ParticipantPicker when the add participants action is clicked", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openOrderModal();
    expect(dialog.getByTestId("participant-add")).toHaveAttribute("data-exam-id", "exam-1");
    expect(dialog.queryByText("exam_grant_grant")).not.toBeInTheDocument();
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

    const dialog = openOrderModal();
    fireEvent.click(dialog.getByTestId("participant-add"));

    const previewButton = await dialog.findByText("bulk_exam_order_preview");
    fireEvent.click(previewButton);

    expect(previewMutateSpy).toHaveBeenCalledWith(
      { exam_id: "exam-1", student_ids: ["student-1", "student-2"] },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(dialog.getByText(formatRupiah(150000))).toBeInTheDocument();
    });

    const confirmButton = await dialog.findByText("bulk_exam_order_confirm");
    fireEvent.click(confirmButton);

    expect(createMutateSpy).toHaveBeenCalledWith(
      { exam_id: "exam-1", student_ids: ["student-1", "student-2"] },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(dialog.getByText("bulk_exam_order_created")).toBeInTheDocument();
    });
    expect(dialog.getByText(/Tryout UTBK 2026/)).toBeInTheDocument();
  });

  it("renders a preview when the backend omits an empty excluded list", async () => {
    previewMockResult = {
      net_new_count: 2,
      unit_price: 75000,
      total: 150000,
    };

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const dialog = openOrderModal();
    fireEvent.click(dialog.getByTestId("participant-add"));
    fireEvent.click(await dialog.findByText("bulk_exam_order_preview"));

    await waitFor(() => {
      expect(dialog.getByText(formatRupiah(150000))).toBeInTheDocument();
    });
    expect(dialog.queryByText("already_registered")).not.toBeInTheDocument();
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

    const dialog = openOrderModal();
    fireEvent.click(dialog.getByTestId("participant-add"));

    const previewButton = await dialog.findByText("bulk_exam_order_preview");
    fireEvent.click(previewButton);

    const confirmButton = await dialog.findByText("bulk_exam_order_confirm");
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "duplicate student_id in participant selector: student-1",
      );
    });
    expect(screen.queryByText("bulk_exam_order_created")).not.toBeInTheDocument();
  });

  it("resets selection, preview and created-order state when the dialog is closed", async () => {
    previewMockResult = {
      net_new_count: 2,
      excluded: [],
      unit_price: 75000,
      total: 150000,
    };

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const dialog = openOrderModal();
    fireEvent.click(dialog.getByTestId("participant-add"));
    fireEvent.click(await dialog.findByText("bulk_exam_order_preview"));
    fireEvent.click(await dialog.findByText("bulk_exam_order_confirm"));
    await waitFor(() => {
      expect(dialog.getByText("bulk_exam_order_created")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    const reopened = openOrderModal();
    expect(reopened.getByTestId("participant-add")).toBeInTheDocument();
    expect(reopened.queryByText("bulk_exam_order_created")).not.toBeInTheDocument();
    expect(reopened.queryByText("bulk_exam_order_preview_title")).not.toBeInTheDocument();
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

  it("shows the roster only on initial render — no ParticipantPicker, no dialog", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    expect(screen.queryByTestId("participant-add")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByTestId("open-grant-modal")).toBeInTheDocument();
  });

  it("does not render the order/preview button for super_admin", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("participant-add"));
    expect(dialog.queryByText("bulk_exam_order_preview")).not.toBeInTheDocument();
  });

  it("calls useGrantExamAccess with { exam_id, student_ids } and shows granted student names and usernames", async () => {
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

    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("participant-add"));

    const grantButton = await dialog.findByText("exam_grant_grant");
    fireEvent.click(grantButton);

    expect(grantMutateSpy).toHaveBeenCalledWith(
      { exam_id: "exam-1", student_ids: ["student-1", "student-2"] },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(dialog.getByText("Andi Saputra")).toBeInTheDocument();
    });
    expect(dialog.getByText("@andi123")).toBeInTheDocument();
    expect(dialog.getByText("Budi Santoso")).toBeInTheDocument();
    expect(dialog.getByText("@budi456")).toBeInTheDocument();
  });

  it("shows an error toast when the backend rejects duplicate student_ids", async () => {
    grantShouldError = true;
    grantErrorMessage = "duplicate student_id in participant selector: s1";

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("participant-add"));

    const grantButton = await dialog.findByText("exam_grant_grant");
    fireEvent.click(grantButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "duplicate student_id in participant selector: s1",
      );
    });
    expect(screen.queryByText("Andi Saputra")).not.toBeInTheDocument();
  });

  it("resets selection and grant-result state when the dialog is closed", async () => {
    grantMockResult = {
      granted_count: 1,
      granted_students: [{ id: "s1", name: "Andi Saputra", username: "andi123" }],
    };

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("participant-add"));
    fireEvent.click(await dialog.findByText("exam_grant_grant"));
    await waitFor(() => {
      expect(dialog.getByText("Andi Saputra")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    const reopened = openGrantModal();
    expect(reopened.getByTestId("participant-add")).toBeInTheDocument();
    expect(reopened.queryByText("Andi Saputra")).not.toBeInTheDocument();
  });

  it("ignores a stale grant success that resolves after the modal was closed", async () => {
    holdGrantMutation = true;
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("participant-add"));
    fireEvent.click(await dialog.findByText("exam_grant_grant"));
    expect(heldGrantOptions).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    heldGrantOptions.onSuccess({
      granted_count: 1,
      granted_students: [{ id: "s1", name: "Andi Saputra", username: "andi123" }],
    });

    const reopened = openGrantModal();
    expect(reopened.getByTestId("participant-add")).toBeInTheDocument();
    expect(reopened.queryByText("Andi Saputra")).not.toBeInTheDocument();
    expect(reopened.queryByText("exam_grant_success_title")).not.toBeInTheDocument();
  });
});

describe("ExamRegistrationsTab — super_admin CSV bulk grant flow (Frontend B2)", () => {
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

  let lastDownloadedFilename: string | null = null;
  let lastCapturedBlob: Blob | null = null;
  const originalCreateElement = document.createElement.bind(document);

  beforeEach(() => {
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

    URL.createObjectURL = vi.fn().mockImplementation((blob: Blob) => {
      lastCapturedBlob = blob;
      return "blob:mock" as unknown as string;
    }) as typeof URL.createObjectURL;
  });

  it("does not show a CSV mode toggle in the admin_school order modal", () => {
    authUser = { role: "admin_school", school_id: "school-1" };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openOrderModal();
    expect(dialog.queryByTestId("grant-mode-csv")).not.toBeInTheDocument();
    expect(dialog.queryByTestId("grant-mode-manual")).not.toBeInTheDocument();
  });

  it("shows a manual/csv mode toggle only for super_admin, defaulting to manual", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openGrantModal();
    expect(dialog.getByTestId("grant-mode-manual")).toBeInTheDocument();
    expect(dialog.getByTestId("grant-mode-csv")).toBeInTheDocument();
    expect(dialog.getByTestId("participant-add")).toBeInTheDocument();
  });

  it("downloads a username-only template with the exact content", async () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("grant-mode-csv"));

    fireEvent.click(dialog.getByTestId("csv-download-template"));

    await waitFor(() => expect(lastDownloadedFilename).not.toBeNull());
    expect(lastCapturedBlob).not.toBeNull();
    const text = await lastCapturedBlob!.text();
    expect(text).toBe("username\nandi123\nbudi456\n");
  });

  it("switching to CSV mode hides the manual ParticipantPicker and grant button", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("grant-mode-csv"));

    expect(dialog.queryByTestId("participant-add")).not.toBeInTheDocument();
    expect(dialog.getByTestId("csv-file-input")).toBeInTheDocument();
  });

  it("runs presign -> PUT -> enqueue in order, then polls the job and shows the result link on success", async () => {
    csvPresignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/exam-grant-bulk/exam-1/uuid.csv?sig=abc",
      method: "PUT",
      key: "exam-grant-bulk/exam-1/uuid.csv",
    });
    csvPutFile.mockResolvedValueOnce(undefined);
    csvEnqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-1" });

    const { rerender } = render(
      <ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />,
      { wrapper: wrapperFactory() },
    );
    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("grant-mode-csv"));

    const file = new File(["username\nandi123"], "grants.csv", { type: "text/csv" });
    fireEvent.change(dialog.getByTestId("csv-file-input"), { target: { files: [file] } });
    fireEvent.click(dialog.getByTestId("csv-upload-submit"));

    await waitFor(() => expect(csvPresignMutateAsync).toHaveBeenCalledTimes(1));
    expect(csvPresignMutateAsync).toHaveBeenCalledWith("exam-1", {
      filename: "grants.csv",
      contentType: "text/csv",
    });
    await waitFor(() => expect(csvPutFile).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(csvEnqueueMutateAsync).toHaveBeenCalledTimes(1));
    expect(csvEnqueueMutateAsync).toHaveBeenCalledWith({
      examId: "exam-1",
      fileKey: "exam-grant-bulk/exam-1/uuid.csv",
    });

    expect(csvPresignMutateAsync.mock.invocationCallOrder[0]).toBeLessThan(
      csvPutFile.mock.invocationCallOrder[0],
    );
    expect(csvPutFile.mock.invocationCallOrder[0]).toBeLessThan(
      csvEnqueueMutateAsync.mock.invocationCallOrder[0],
    );

    csvJobStatusState.data = {
      id: "job-1",
      type: "exam_grant_bulk",
      status: "succeeded",
      progress: 100,
      result_url: "http://minio.local/result.csv?sig=xyz",
      error: null,
      created_at: "2026-08-01T00:00:00Z",
      updated_at: "2026-08-01T00:01:00Z",
    };
    rerender(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />);

    await waitFor(() => {
      const link = within(screen.getByRole("dialog")).getByRole("link");
      expect((link as HTMLAnchorElement).href).toBe("http://minio.local/result.csv?sig=xyz");
    });
  });

  it("shows a failed status with the result link when the job fails but produced a report", async () => {
    csvPresignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=abc",
      method: "PUT",
      key: "exam-grant-bulk/exam-1/uuid.csv",
    });
    csvPutFile.mockResolvedValueOnce(undefined);
    csvEnqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-2" });

    const { rerender } = render(
      <ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />,
      { wrapper: wrapperFactory() },
    );
    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("grant-mode-csv"));

    const file = new File(["username\nbaduser"], "grants.csv", { type: "text/csv" });
    fireEvent.change(dialog.getByTestId("csv-file-input"), { target: { files: [file] } });
    fireEvent.click(dialog.getByTestId("csv-upload-submit"));

    await waitFor(() => expect(csvEnqueueMutateAsync).toHaveBeenCalled());

    csvJobStatusState.data = {
      id: "job-2",
      type: "exam_grant_bulk",
      status: "failed",
      progress: 100,
      result_url: "http://minio.local/allfail.csv?sig=def",
      error: "exam_grant_bulk job job-2: all rows failed",
      created_at: "2026-08-01T00:00:00Z",
      updated_at: "2026-08-01T00:01:00Z",
    };
    rerender(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />);

    await waitFor(() => {
      const link = within(screen.getByRole("dialog")).getByRole("link");
      expect((link as HTMLAnchorElement).href).toBe("http://minio.local/allfail.csv?sig=def");
    });
  });

  it("resets CSV file and job state when the modal is closed and reopened", async () => {
    csvPresignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=abc",
      method: "PUT",
      key: "exam-grant-bulk/exam-1/uuid.csv",
    });
    csvPutFile.mockResolvedValueOnce(undefined);
    csvEnqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-3" });

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    let dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("grant-mode-csv"));

    const file = new File(["username\nandi123"], "grants.csv", { type: "text/csv" });
    fireEvent.change(dialog.getByTestId("csv-file-input"), { target: { files: [file] } });
    fireEvent.click(dialog.getByTestId("csv-upload-submit"));

    await waitFor(() => expect(csvEnqueueMutateAsync).toHaveBeenCalled());
    expect(dialog.getByText("grants.csv")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    csvJobStatusState.data = null;
    dialog = openGrantModal();
    expect(dialog.getByTestId("grant-mode-manual")).toBeInTheDocument();
    expect(dialog.queryByText("grants.csv")).not.toBeInTheDocument();
  });

  it("does not continue CSV upload/enqueue after the modal is closed during presign", async () => {
    let resolvePresign: (value: { url: string; method: string; key: string }) => void = () => {};
    csvPresignMutateAsync.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolvePresign = resolve;
        }),
    );

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    const dialog = openGrantModal();
    fireEvent.click(dialog.getByTestId("grant-mode-csv"));

    const file = new File(["username\nandi123"], "grants.csv", { type: "text/csv" });
    fireEvent.change(dialog.getByTestId("csv-file-input"), { target: { files: [file] } });
    fireEvent.click(dialog.getByTestId("csv-upload-submit"));

    await waitFor(() => expect(csvPresignMutateAsync).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await act(async () => {
      resolvePresign({
        url: "http://minio.local/exam-grant-bulk/exam-1/uuid.csv?sig=abc",
        method: "PUT",
        key: "exam-grant-bulk/exam-1/uuid.csv",
      });
      await Promise.resolve();
    });

    expect(csvPutFile).not.toHaveBeenCalled();
    expect(csvEnqueueMutateAsync).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
  });
});

describe("ExamRegistrationsTab — admin_exam read-only (FR-8/FR-9)", () => {
  beforeEach(() => {
    authUser = { role: "admin_exam" };
    rosterData = { data: [] };
    rosterIsLoading = false;
    rosterIsError = false;
  });

  const rosterRows = [
    {
      registration_id: "reg-1",
      student_id: "s1",
      student_name: "Andi Saputra",
      student_username: "andi123",
      participant_number: 1,
      participant_no: "250620-0042-000001",
      status: "registered",
      checked_in_at: null,
      token: "TOKEN-ANDI-001",
    },
  ];

  it("shows the roster, sort control and CSV export", () => {
    rosterData = { data: rosterRows };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    expect(screen.getByText("Andi Saputra")).toBeInTheDocument();
    expect(screen.getByText("exam_roster_th_participant_no")).toBeInTheDocument();
    expect(screen.getByText("exam_roster_export_csv")).toBeInTheDocument();
  });

  it("shows the manual-registration notice", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    expect(screen.getByText("exam_registrations_manual_notice")).toBeInTheDocument();
  });

  it("shows none of the write controls — no action button, no dialog, no picker", () => {
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    expect(screen.queryByTestId("open-grant-modal")).not.toBeInTheDocument();
    expect(screen.queryByTestId("open-order-modal")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.queryByTestId("participant-add")).not.toBeInTheDocument();
    expect(screen.queryByText("exam_grant_grant")).not.toBeInTheDocument();
    expect(screen.queryByText("bulk_exam_order_preview")).not.toBeInTheDocument();
  });

  // Positive control: a permitted role at the same initial state, with a
  // selection seeded through the picker, does render the write controls —
  // proving the admin_exam absences above are role-gated, not just always-off.
  it("shows the write controls for a permitted role once the modal is opened and participants are picked", () => {
    authUser = { role: "admin_school", school_id: "school-1" };
    rosterData = { data: [] };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const dialog = openOrderModal();
    expect(dialog.getByTestId("participant-add")).toBeInTheDocument();
    fireEvent.click(dialog.getByTestId("participant-add"));
    expect(dialog.getByText("bulk_exam_order_preview")).toBeInTheDocument();
  });
});

describe("ExamRegistrationsTab — participant roster (FR-32)", () => {
  beforeEach(() => {
    authUser = { role: "admin_school", school_id: "school-1" };
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
      token: "TOKEN-BUDI-002",
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
      token: "TOKEN-ANDI-001",
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
      token: "TOKEN-CITRA-003",
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

  it("requests the first page ascending by default and renders the server order", () => {
    rosterData = { data: [rows[1], rows[0], rows[2]] };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    expect(rosterHookSpy).toHaveBeenLastCalledWith("exam-1", {
      cursor: undefined,
      limit: 20,
      sort: "asc",
    });
    const cells = screen.getAllByTestId("roster-participant-no");
    expect(cells.map((c) => c.textContent)).toEqual([
      "250620-0042-000001",
      "250620-0042-000002",
      "—",
    ]);
  });

  it("requests descending sort from page one when the header is clicked", () => {
    const ascending = { data: [rows[1], rows[0], rows[2]], next_cursor: "asc-2" };
    const descending = { data: [rows[0], rows[1], rows[2]], next_cursor: "desc-2" };
    resolveRosterData = (filter) => filter.sort === "desc" ? descending : ascending;
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByText("exam_roster_th_participant_no"));

    expect(rosterHookSpy).toHaveBeenLastCalledWith("exam-1", {
      cursor: undefined,
      limit: 20,
      sort: "desc",
    });
    const cells = screen.getAllByTestId("roster-participant-no");
    expect(cells.map((c) => c.textContent)).toEqual([
      "250620-0042-000002",
      "250620-0042-000001",
      "—",
    ]);
  });

  it("appends unique registrations when Load More follows next_cursor", () => {
    const firstPage = { data: [rows[1], rows[0]], next_cursor: "page-2" };
    const secondPage = { data: [rows[0], rows[2]] };
    resolveRosterData = (filter) => filter.cursor === "page-2" ? secondPage : firstPage;

    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });
    fireEvent.click(screen.getByText("sys_load_more"));

    expect(rosterHookSpy).toHaveBeenLastCalledWith("exam-1", {
      cursor: "page-2",
      limit: 20,
      sort: "asc",
    });
    expect(screen.getAllByText("Budi Santoso")).toHaveLength(1);
    expect(screen.getByText("Citra Dewi")).toBeInTheDocument();
  });

  // NFR-S7: the exam token is a check-in credential. The realistic leak is a
  // screen-share/screenshot of the roster, so every row must render masked by
  // default and only reveal its real token after that row's own toggle fires.
  it("renders every row's token masked by default", () => {
    rosterData = { data: rows };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    for (const row of rows) {
      expect(screen.queryByText(row.token)).not.toBeInTheDocument();
    }
    expect(screen.getAllByText("••••••••")).toHaveLength(rows.length);
  });

  it("reveals only the toggled row's real token, leaving the others masked", () => {
    rosterData = { data: [rows[1], rows[0], rows[2]] };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const revealButtons = screen.getAllByLabelText("exam_roster_show_token");
    fireEvent.click(revealButtons[0]);

    const andi = rows.find((r) => r.registration_id === "reg-1")!;
    const others = rows.filter((r) => r.registration_id !== "reg-1");

    expect(screen.getByText(andi.token)).toBeInTheDocument();
    for (const other of others) {
      expect(screen.queryByText(other.token)).not.toBeInTheDocument();
    }
    expect(screen.getAllByText("••••••••")).toHaveLength(rows.length - 1);
  });

  it("exports from the full-roster endpoint even when another UI page exists", async () => {
    rosterData = { data: [rows[1]], next_cursor: "page-2" };
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    fireEvent.click(screen.getByText("exam_roster_export_csv"));

    await waitFor(() => expect(exportRosterSpy).toHaveBeenCalledWith("exam-1"));
  });

  it("shows an error and restores the export button when CSV export fails", async () => {
    rosterData = { data: [rows[1]] };
    vi.mocked(toast.error).mockClear();
    exportRosterSpy.mockRejectedValueOnce(new Error("forbidden"));
    render(<ExamRegistrationsTab examId="exam-1" examName="Tryout UTBK 2026" />, {
      wrapper: wrapperFactory(),
    });

    const button = screen.getByRole("button", { name: "exam_roster_export_csv" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("exam_roster_export_failed");
    });
    expect(button).toBeEnabled();
  });
});
