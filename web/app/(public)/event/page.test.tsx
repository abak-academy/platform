import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import EventPage from "./page";
import * as api from "@/lib/api";
import { edition, event } from "@/components/events/fixtures";

const events = [
  event({
    open_editions: [
      edition({ product_id: "e-sd", title: "ESOLIO SD", stages: ["SD"] }),
      edition({ product_id: "e-smp", title: "ESOLIO SMP", stages: ["SMP"] }),
    ],
  }),
  event({
    slug: "ikmc",
    name: "IKMC",
    scope: "international",
    story: ["Cerita IKMC."],
    open_editions: [edition({ product_id: "i-sd", title: "IKMC SD", stages: ["SD"] })],
  }),
];

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <EventPage />
    </QueryClientProvider>,
  );
}

const grid = () => document.querySelector("[data-reveal-grid]") as HTMLElement;
const group = (name: string) => screen.getByRole("group", { name });

beforeEach(() => {
  window.location.hash = "";
  vi.spyOn(api, "apiFetch").mockResolvedValue({ data: events } as never);
});
afterEach(() => vi.restoreAllMocks());

describe("EventPage", () => {
  it("shows the first event by default", async () => {
    renderPage();
    expect(await screen.findByRole("heading", { name: "ESOLIO", level: 3 })).toBeInTheDocument();
  });

  it("preselects the event named in the URL hash", async () => {
    window.location.hash = "#ikmc";
    renderPage();
    expect(await screen.findByText("Cerita IKMC.")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /IKMC/ })).toHaveAttribute("aria-checked", "true");
  });

  it("filters by event, tingkat and jenjang and Reset restores the grid", async () => {
    renderPage();
    await screen.findByText("3 edisi");
    expect(within(grid()).getAllByRole("link")).toHaveLength(3);

    await userEvent.click(within(group("Event")).getByRole("button", { name: "IKMC" }));
    expect(within(grid()).getAllByRole("link")).toHaveLength(1);
    await userEvent.click(within(group("Event")).getByRole("button", { name: "Semua" }));

    await userEvent.click(within(group("Tingkat")).getByRole("button", { name: "Internasional" }));
    expect(within(grid()).getAllByRole("link")).toHaveLength(1);
    await userEvent.click(within(group("Tingkat")).getByRole("button", { name: "Semua" }));

    await userEvent.click(within(group("Jenjang")).getByRole("button", { name: "SMP" }));
    expect(within(grid()).getAllByRole("link")).toHaveLength(1);
    expect(screen.getByText("1 edisi")).toBeInTheDocument();

    await userEvent.click(within(group("Event")).getByRole("button", { name: "IKMC" }));
    expect(screen.getByText("Tidak ada edisi")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Reset filter" }));
    expect(within(grid()).getAllByRole("link")).toHaveLength(3);
  });

  it("shows an EmptyState with a WhatsApp CTA when the API fails", async () => {
    vi.spyOn(api, "apiFetch").mockRejectedValue(new Error("boom"));
    renderPage();
    await waitFor(() => expect(screen.getAllByText("Event belum tersedia").length).toBeGreaterThan(0));
    expect(screen.getAllByRole("link", { name: "Tanya via WhatsApp" })[0].getAttribute("href")).toContain("wa.me");
  });

  it("shows skeletons while loading", () => {
    vi.spyOn(api, "apiFetch").mockReturnValue(new Promise(() => {}) as never);
    renderPage();
    expect(screen.getByTestId("events-skeleton")).toBeInTheDocument();
  });
});
