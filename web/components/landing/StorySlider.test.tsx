import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { StorySlider } from "./StorySlider";
import { fileUrl } from "@/lib/api";
import type { PublicTestimonial } from "@/lib/hooks/testimonials";

const item = (n: number, over: Partial<PublicTestimonial> = {}): PublicTestimonial => ({
  id: `t${n}`,
  name: `Peserta ${n}`,
  role_label: `Siswa ${n}`,
  photo_url: `testimonials/p${n}.jpg`,
  quote: `Kutipan ${n}`,
  achievements: [`Medali ${n}`],
  ...over,
});

describe("StorySlider", () => {
  it("renders nothing when there are no testimonials", () => {
    const { container } = render(<StorySlider items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("splits photo and text 1:1 and lets the photo fill the text's height, not a fixed 4:5", () => {
    const { container } = render(<StorySlider items={[item(1)]} />);
    const photo = container.querySelector<HTMLElement>("[data-story-photo]")!;
    expect(photo.parentElement!.className).toContain("md:grid-cols-2");
    expect(photo.className).not.toContain("aspect-[4/5]");
    const shown = photo.querySelector<HTMLImageElement>("img[alt]:not([alt=\"\"])")!;
    expect(shown.className).toContain("object-contain");
  });

  it("shows one story with its photo, role, chips and counter", () => {
    render(<StorySlider items={[item(1, { achievements: ["A", "B", "C", "D"] }), item(2)]} />);
    expect(screen.getByRole("heading", { name: "Cerita peserta" })).toBeInTheDocument();
    expect(screen.getByText(/Kutipan 1/)).toBeInTheDocument();
    expect(screen.getByText("Siswa 1")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Foto Peserta 1" })).toHaveAttribute("src", fileUrl("testimonials/p1.jpg"));
    expect(screen.getByText("01/02")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
    expect(screen.getByText("C")).toBeInTheDocument();
    expect(screen.queryByText("D")).not.toBeInTheDocument();
  });

  it("renders duplicate achievements without key collisions", () => {
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    render(<StorySlider items={[item(1, { achievements: ["Emas", "Emas"] })]} />);
    expect(screen.getAllByText("Emas")).toHaveLength(2);
    expect(err).not.toHaveBeenCalled();
    err.mockRestore();
  });

  it("next swaps quote and photo; prev wraps around", () => {
    render(<StorySlider items={[item(1), item(2)]} />);
    fireEvent.click(screen.getByRole("button", { name: "Cerita berikutnya" }));
    expect(screen.getByText(/Kutipan 2/)).toBeInTheDocument();
    expect(screen.queryByText(/Kutipan 1/)).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Foto Peserta 2" })).toHaveAttribute("src", fileUrl("testimonials/p2.jpg"));
    expect(screen.getByText("02/02")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cerita berikutnya" }));
    expect(screen.getByText("01/02")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cerita sebelumnya" }));
    expect(screen.getByText(/Kutipan 2/)).toBeInTheDocument();
    expect(screen.getByText("02/02")).toBeInTheDocument();
  });
});
