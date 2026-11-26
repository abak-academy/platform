import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ContinueLearningCard } from "./ContinueLearningCard";

const course = (over = {}) => ({ id: "c1", title: "Matematika SD", progress: 0.4, total_lessons: 10, done_lessons: 4, ...over });

describe("ContinueLearningCard", () => {
  it("renders title, percent, next lesson text and the row link", () => {
    render(<ContinueLearningCard courses={[course()]} />);
    expect(screen.getByText("Matematika SD")).toBeInTheDocument();
    expect(screen.getByText("40%")).toBeInTheDocument();
    expect(screen.getByText("Materi 5 dari 10")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Matematika SD/ })).toHaveAttribute("href", "/courses/c1");
  });

  it("shows Kursus selesai at 100%", () => {
    render(<ContinueLearningCard courses={[course({ progress: 1, done_lessons: 10 })]} />);
    expect(screen.getByText("Kursus selesai")).toBeInTheDocument();
    expect(screen.queryByText(/Materi \d+ dari/)).not.toBeInTheDocument();
  });

  it("renders a cover image only when the course has one", () => {
    const { container } = render(
      <ContinueLearningCard courses={[course({ cover: "https://cdn.test/a.png" }), course({ id: "c2", title: "IPA" })]} />,
    );
    const imgs = container.querySelectorAll("img");
    expect(imgs).toHaveLength(1);
    expect(imgs[0]).toHaveAttribute("src", "https://cdn.test/a.png");
  });

  it("links header and add button, and shows the empty state", () => {
    render(<ContinueLearningCard courses={[]} />);
    expect(screen.getByText("Belum ada kursus.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "+ Tambah kursus" })).toHaveAttribute("href", "/catalog");
    expect(screen.getByRole("link", { name: "Lihat semua" })).toHaveAttribute("href", "/courses");
  });

  it("keeps the add button when courses exist", () => {
    render(<ContinueLearningCard courses={[course()]} />);
    expect(screen.queryByText("Belum ada kursus.")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "+ Tambah kursus" })).toHaveAttribute("href", "/catalog");
  });
});
