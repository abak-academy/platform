import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { VideoPlayer } from "./VideoPlayer";

describe("VideoPlayer", () => {
  beforeEach(() => {
    vi.stubGlobal("YT", undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("renders the empty state when there is no video", () => {
    render(<VideoPlayer title="Pelajaran 1" />);
    expect(screen.getByText(/Video belum tersedia/i)).toBeInTheDocument();
  });

  it("renders a click shield over the player", () => {
    const { container } = render(
      <VideoPlayer videoRef="https://www.youtube.com/watch?v=abc123" title="L1" />,
    );
    expect(container.querySelector('[data-testid="video-shield"]')).not.toBeNull();
  });

  it("exposes no link to youtube.com anywhere in the tree", () => {
    const { container } = render(
      <VideoPlayer videoRef="https://www.youtube.com/watch?v=abc123" title="L1" />,
    );
    const hrefs = Array.from(container.querySelectorAll("a")).map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.some((h) => h.includes("youtube.com") || h.includes("youtu.be"))).toBe(false);
  });

  it("renders our own control bar, not YouTube's", () => {
    render(<VideoPlayer videoRef="abc123" title="L1" />);
    expect(screen.getByRole("button", { name: /putar|play/i })).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: /posisi|progress/i })).toBeInTheDocument();
  });

  it("falls back to a plain embed when the IFrame API fails to load", async () => {
    const { container } = render(
      <VideoPlayer videoRef="abc123" title="L1" forceFallback />,
    );
    const iframe = container.querySelector("iframe");
    expect(iframe).not.toBeNull();
    expect(container.querySelector('[data-testid="video-shield"]')).toBeNull();
  });
});
