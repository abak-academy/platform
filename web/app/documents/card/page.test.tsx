import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";

import CardPrintPage from "./page";
import { getCardPrintData } from "@/lib/server/print-api";
import type { CardPrintData } from "@/lib/server/print-api";

vi.mock("@/lib/server/print-api", () => ({
  getCardPrintData: vi.fn(),
}));

const mockedGet = vi.mocked(getCardPrintData);

const sampleCardData: CardPrintData = {
  participant_no: "260601-0001-000005",
  student_name: "Budi Santoso",
  school: "SMA Negeri 1 Jakarta",
  exam_title: "Ujian Simulasi UTBK",
  exam_schedule: "01 Aug 2026 09:00 WIB",
  check_in_code: "ABC12345",
};

function assertEmpty(container: HTMLElement) {
  const text = container.textContent ?? "";
  expect(text).not.toContain("260601-0001-000005");
  expect(text).not.toContain("Budi Santoso");
  expect(text).not.toContain("ABC12345");
}

describe("CardPrintPage", () => {
  beforeEach(() => {
    mockedGet.mockReset();
  });

  it("renders an empty document when no token is present (FR-22)", async () => {
    const jsx = await CardPrintPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx as ReactElement);

    expect(mockedGet).not.toHaveBeenCalled();
    assertEmpty(container);
  });

  it("renders the same empty document for a rejected token (FR-23)", async () => {
    mockedGet.mockResolvedValue(null);

    const jsx = await CardPrintPage({
      searchParams: Promise.resolve({ token: "not-a-real-token", id: "reg-1" }),
    });
    const { container } = render(jsx as ReactElement);

    assertEmpty(container);
  });

  it("renders server-authored values for a valid token (FR-24)", async () => {
    mockedGet.mockResolvedValue(sampleCardData);

    const jsx = await CardPrintPage({
      searchParams: Promise.resolve({ token: "good-token", id: "reg-1" }),
    });
    render(jsx as ReactElement);

    expect(mockedGet).toHaveBeenCalledWith("reg-1", "good-token");
    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getAllByText("260601-0001-000005").length).toBeGreaterThan(0);
    expect(screen.getByText("ABC12345")).toBeInTheDocument();
    expect(screen.getByText("Ujian Simulasi UTBK")).toBeInTheDocument();
  });
});
