import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import CoursesPage from "./page";
import type { Course } from "@/lib/types";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let coursesState = {
  data: null as Course[] | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-courses", () => ({
  useAdminCourses: () => coursesState,
  useCreateCourse: () => createState,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const sampleCourses: Course[] = [
  { id: "c1", title: "Matematika Dasar", level: "SMA", subject: "Matematika", instructor_name: "Pak Budi" },
  { id: "c2", title: "Fisika SMA", level: "SMA", subject: "Fisika" },
];

describe("CoursesPage", () => {
  beforeEach(() => {
    coursesState = {
      data: sampleCourses,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    mockPush.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders a card per course with title, level, subject, instructor", async () => {
    render(<CoursesPage />);

    await waitFor(() => {
      expect(screen.getByText("Matematika Dasar")).toBeInTheDocument();
      expect(screen.getByText("Fisika SMA")).toBeInTheDocument();
    });

    expect(screen.getByText("Pak Budi")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.getByText("Matematika Dasar").closest("button")).toHaveClass("md-card-flush");
  });

  it("filters the cards by title or instructor", async () => {
    render(<CoursesPage />);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "budi" } });
    expect(screen.getByText("Matematika Dasar")).toBeInTheDocument();
    expect(screen.queryByText("Fisika SMA")).not.toBeInTheDocument();
  });

  it("navigates to builder when a row is clicked", async () => {
    render(<CoursesPage />);

    await waitFor(() => expect(screen.getByText("Matematika Dasar")).toBeInTheDocument());

    const row = screen.getByText("Matematika Dasar").closest("button");
    fireEvent.click(row!);

    expect(mockPush).toHaveBeenCalledWith("/admin/courses/c1");
  });

  it("opens the create modal and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "c3", title: "Kimia SMA" });

    render(<CoursesPage />);

    await waitFor(() => expect(screen.getByText("Matematika Dasar")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat kursus/i }));

    expect(screen.getByRole("dialog", { name: /buat kursus/i })).toBeInTheDocument();

    const nameInput = screen.getByLabelText(/^judul$/i);
    fireEvent.input(nameInput, { target: { value: "Kimia SMA" } });

    const levelInput = screen.getByLabelText(/^jenjang$/i);
    fireEvent.input(levelInput, { target: { value: "SMA" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ title: "Kimia SMA", level: "SMA" })
      );
      expect(toast.success).toHaveBeenCalledWith("Kursus dibuat.");
    });
  });

  it("surfaces an API error as inline error text", async () => {
    coursesState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<CoursesPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});
