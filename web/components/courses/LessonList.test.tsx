import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LessonList } from "./LessonList";

describe("LessonList", () => {
  it("marks only the current lesson with aria-current", () => {
    render(
      <LessonList
        title="Kursus"
        sections={[{ id: "s1", title: "Bab", lessons: [{ id: "l1", title: "Satu" }, { id: "l2", title: "Dua" }] } as any]}
        activeLessonId="l2"
        onSelectLesson={() => {}}
        doneCount={0}
        totalCount={2}
        progressPct={0}
      />,
    );
    expect(screen.getByRole("button", { name: /Dua/ })).toHaveAttribute("aria-current", "true");
    expect(screen.getByRole("button", { name: /Satu/ })).not.toHaveAttribute("aria-current");
  });
});
