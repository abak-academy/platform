import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StudySummaryCard } from "./StudySummaryCard";

const study = (over = {}) => ({
  visited_lectures: 12,
  total_lectures: 20,
  enrolled_courses_count: 4,
  completed_courses: 1,
  total_minutes: 150,
  ...over,
});

describe("StudySummaryCard", () => {
  it("shows hours with one decimal in locale format and the three legend values", () => {
    render(<StudySummaryCard study={study()} />);
    expect(screen.getByText("Total jam belajar")).toBeInTheDocument();
    expect(screen.getByText(/2,5/)).toBeInTheDocument();
    expect(screen.getByText("Materi ditonton").closest("li")).toHaveTextContent("12");
    expect(screen.getByText("Sisa materi").closest("li")).toHaveTextContent("8");
    expect(screen.getByText("Kursus selesai").closest("li")).toHaveTextContent("1/4");
  });

  it("pads whole hours to one decimal", () => {
    render(<StudySummaryCard study={study({ total_minutes: 120 })} />);
    expect(screen.getByText(/2,0/)).toBeInTheDocument();
  });

  it("shows the empty copy when there are no lectures", () => {
    render(<StudySummaryCard study={study({ total_lectures: 0, visited_lectures: 0 })} />);
    expect(screen.getByText("Ringkasan waktu belajar akan muncul setelah kamu mulai menonton materi.")).toBeInTheDocument();
    expect(screen.queryByText("Materi ditonton")).not.toBeInTheDocument();
  });
});
