import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EditionCard } from "./EditionCard";
import { edition, event } from "@/components/events/fixtures";

describe("EditionCard", () => {
  it("links to the product and shows event, date, level, scope, price and phase", () => {
    render(<EditionCard event={event()} edition={edition({ product_id: "prod-3" })} />);
    const card = screen.getByRole("link");
    expect(card).toHaveAttribute("href", "/catalog/prod-3");
    expect(card).toHaveAttribute("data-tilt");
    expect(card).toHaveTextContent("ESOLIO");
    expect(card).toHaveTextContent("5 Des 2026");
    expect(card).toHaveTextContent("SD · Kelas 1–6");
    expect(card).toHaveTextContent("Nasional");
    expect(card).toHaveTextContent("Rp150.000");
    expect(card).toHaveTextContent("Pendaftaran dibuka");
  });

  it("shows Gratis and the live badge", () => {
    render(<EditionCard event={event()} edition={edition({ price: 0, phase: "live" })} />);
    expect(screen.getByText("Gratis")).toBeInTheDocument();
    expect(screen.getByText("Sedang berlangsung")).toBeInTheDocument();
  });
});
