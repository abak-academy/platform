import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EventStoryCard } from "./EventStoryCard";
import { edition, event } from "./fixtures";

describe("EventStoryCard", () => {
  it("with no open edition offers a WhatsApp ask and no Daftar", () => {
    render(<EventStoryCard event={event()} />);
    const ask = screen.getByRole("link", { name: "Tanya jadwal" });
    expect(ask.getAttribute("href")).toContain("https://wa.me/");
    expect(decodeURIComponent(ask.getAttribute("href")!)).toContain("jadwal ESOLIO");
    expect(ask).toHaveAttribute("target", "_blank");
    expect(ask).toHaveAttribute("rel", expect.stringContaining("noopener"));
    expect(screen.queryByRole("link", { name: /^Daftar/ })).toBeNull();
  });

  it("with one open edition links Daftar level and price to the catalog", () => {
    render(<EventStoryCard event={event({ open_editions: [edition({ product_id: "prod-9" })] })} />);
    const link = screen.getByRole("link", { name: /^Daftar SD · Kelas 1–6 · Rp150\.000/ });
    expect(link).toHaveAttribute("href", "/catalog/prod-9");
    expect(screen.queryByRole("link", { name: "Tanya jadwal" })).toBeNull();
  });

  it("shows Gratis for a free edition", () => {
    render(<EventStoryCard event={event({ open_editions: [edition({ price: 0 })] })} />);
    expect(screen.getByRole("link", { name: /Gratis/ })).toBeInTheDocument();
  });

  it("caps Daftar links at two", () => {
    const eds = ["a", "b", "c"].map((id) => edition({ product_id: id }));
    render(<EventStoryCard event={event({ open_editions: eds })} />);
    const links = screen.getAllByRole("link", { name: /^Daftar/ });
    expect(links.map((l) => l.getAttribute("href"))).toEqual(["/catalog/a", "/catalog/b"]);
  });

  it("renders Situs resmi only when official_url is set", () => {
    const { rerender } = render(<EventStoryCard event={event({ official_url: "https://example.org" })} />);
    expect(screen.getByRole("link", { name: /Situs resmi/ })).toHaveAttribute("href", "https://example.org");
    rerender(<EventStoryCard event={event({ official_url: null })} />);
    expect(screen.queryByRole("link", { name: /Situs resmi/ })).toBeNull();
  });

  it("renders story, rounds, categories, facts and schedule state", () => {
    render(<EventStoryCard event={event()} />);
    expect(screen.getByText("Paragraf satu.")).toBeInTheDocument();
    expect(screen.getByText("Penyisihan")).toBeInTheDocument();
    expect(screen.getByText("Fisika dan biologi")).toBeInTheDocument();
    expect(screen.getByText("Online")).toBeInTheDocument();
    expect(screen.getByText("Berlangsung")).toBeInTheDocument();
  });
});
