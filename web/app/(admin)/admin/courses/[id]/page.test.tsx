import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import CourseBuilderPage from "./page";
import type { AdminCourseDetail } from "@/lib/types";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ id: "c1" }),
}));

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let courseState = {
  data: null as AdminCourseDetail | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-courses", () => ({
  useAdminCourse: () => courseState,
  useUpdateCourse: () => updateState,
}));

vi.mock("@/components/admin/SectionEditor", () => ({
  SectionEditor: ({ courseId }: { courseId: string }) => <div data-testid="section-editor">{courseId}</div>,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const sampleCourse: AdminCourseDetail = {
  id: "c1",
  title: "Matematika Dasar",
  level: "SMA",
  subject: "Matematika",
  instructor_name: "Pak Budi",
  section_count: 2,
  lesson_count: 5,
};

describe("CourseBuilderPage", () => {
  beforeEach(() => {
    courseState = {
      data: sampleCourse,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    mockPush.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders course metadata form and section editor", async () => {
    render(<CourseBuilderPage />);

    await waitFor(() => {
      expect(screen.getByDisplayValue("Matematika Dasar")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Pak Budi")).toBeInTheDocument();
    });

    expect(screen.getByTestId("section-editor")).toHaveTextContent("c1");
    expect(screen.getByRole("link", { name: /kembali ke kursus/i })).toHaveAttribute("href", "/admin/courses");
  });

  it("calls update mutation when metadata is changed and saved", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "c1", title: "Matematika Lanjut" });

    render(<CourseBuilderPage />);

    await waitFor(() => expect(screen.getByDisplayValue("Matematika Dasar")).toBeInTheDocument());

    const titleInput = screen.getByLabelText(/judul/i);
    fireEvent.input(titleInput, { target: { value: "Matematika Lanjut" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan metadata/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ id: "c1", input: expect.objectContaining({ title: "Matematika Lanjut" }) })
      );
      expect(toast.success).toHaveBeenCalledWith("Metadata kursus disimpan.");
    });
  });

  it("surfaces an API error as inline error text", async () => {
    courseState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<CourseBuilderPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});
