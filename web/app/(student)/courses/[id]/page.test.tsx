import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import CourseDetailPage from "./page";

const mutate = vi.fn();
const toastSuccess = vi.fn();
let course: unknown;

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "c1" }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
}));
vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/lib/hooks/courses", () => ({
  useCourse: () => ({ data: course, isLoading: false, isError: false, error: null, refetch: vi.fn() }),
  useCompleteLesson: () => ({ mutate, isPending: false }),
}));
vi.mock("@/components/courses/VideoPlayer", () => ({
  VideoPlayer: ({ title }: { title?: string }) => <div data-testid="player">{title}</div>,
}));

const lesson = (id: string, completed = false) => ({ id, title: `Materi ${id}`, completed, duration_seconds: 600 });

function setCourse(lessons: ReturnType<typeof lesson>[]) {
  course = { id: "c1", title: "Kursus Uji", sections: [{ id: "s1", title: "Bab 1", lessons }] };
}

// Runs the mutation's onSuccess like react-query would after a 200.
function succeedNextMutation() {
  const [, opts] = mutate.mock.calls.at(-1)!;
  opts.onSuccess();
}

describe("CourseDetailPage mark complete", () => {
  beforeEach(() => {
    mutate.mockReset();
    toastSuccess.mockReset();
  });

  it("reads 'Tandai selesai & lanjut' on an open lesson that has a next one and waits for the server", () => {
    setCourse([lesson("l1"), lesson("l2")]);
    render(<CourseDetailPage />);
    expect(screen.getByTestId("player")).toHaveTextContent("Materi l1");
    fireEvent.click(screen.getByRole("button", { name: "Tandai selesai & lanjut" }));
    expect(mutate).toHaveBeenCalledWith({ courseId: "c1", lessonId: "l1" }, expect.anything());
    // Nothing moves until the server confirms.
    expect(screen.getByTestId("player")).toHaveTextContent("Materi l1");
  });

  it("advances the player to the next lesson once the save succeeds", async () => {
    setCourse([lesson("l1"), lesson("l2")]);
    render(<CourseDetailPage />);
    fireEvent.click(screen.getByRole("button", { name: "Tandai selesai & lanjut" }));
    succeedNextMutation();
    expect(await screen.findByText("Materi l2", { selector: "[data-testid=player]" })).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith("Pelajaran selesai.");
  });

  it("the last lesson keeps plain 'Tandai selesai' and finishing the course shows the course-done toast", () => {
    setCourse([lesson("l1", true), lesson("l2")]);
    render(<CourseDetailPage />);
    fireEvent.click(screen.getByRole("button", { name: "Tandai selesai" }));
    succeedNextMutation();
    expect(toastSuccess).toHaveBeenCalledWith("Selamat! Kursus selesai.");
  });

  it("a finished lesson still toggles back to undone and does not advance", () => {
    setCourse([lesson("l1", true), lesson("l2")]);
    render(<CourseDetailPage />);
    // l2 is the first open lesson, so activate l1 through the list first.
    fireEvent.click(screen.getByText("Materi l1"));
    fireEvent.click(screen.getByRole("button", { name: "Selesai" }));
    succeedNextMutation();
    expect(toastSuccess).toHaveBeenCalledWith("Pelajaran dibatalkan.");
    expect(screen.getByTestId("player")).toHaveTextContent("Materi l1");
  });
});
