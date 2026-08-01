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

describe("CardPrintPage", () => {
  beforeEach(() => {
    mockedGet.mockReset();
  });

  // notFound() throws (caught by Next's rendering pipeline in production and
  // turned into a 404 response with an empty body via app/documents/not-found.tsx)
  // rather than returning null: a 200-with-empty-body was indistinguishable
  // from success to Gotenberg, which cached the resulting blank PDF forever
  // (NFR-R1). See the fix's comment on CardPrintPage.
  it("signals failure via notFound() when no token is present (FR-22, NFR-R1)", async () => {
    await expect(
      CardPrintPage({ searchParams: Promise.resolve({}) })
    ).rejects.toMatchObject({ digest: "NEXT_HTTP_ERROR_FALLBACK;404" });

    expect(mockedGet).not.toHaveBeenCalled();
  });

  it("signals failure via notFound() for a rejected token (FR-23, NFR-R1)", async () => {
    mockedGet.mockResolvedValue(null);

    await expect(
      CardPrintPage({
        searchParams: Promise.resolve({ token: "not-a-real-token", id: "reg-1" }),
      })
    ).rejects.toMatchObject({ digest: "NEXT_HTTP_ERROR_FALLBACK;404" });
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

  // Task 25: the print route must carry the four values buildCardHTML
  // (backend/internal/service/card_html.go) showed before Task 12 switched
  // GetExamCard to this print route — tenant name/logo, student photo, and
  // the check-in footer note.
  it("renders tenant name, tenant logo, student photo and the check-in note when the payload supplies them", async () => {
    mockedGet.mockResolvedValue({
      ...sampleCardData,
      tenant_name: "Bimbel Prima",
      tenant_logo_url: "https://cdn.example.com/logo.png",
      photo_url: "https://storage.example.com/avatars/budi.png?sig=abc",
      footer_note: "Harap check-in dalam waktu 15 menit sebelum ujian.",
    });

    const jsx = await CardPrintPage({
      searchParams: Promise.resolve({ token: "good-token", id: "reg-1" }),
    });
    const { container } = render(jsx as ReactElement);

    expect(container.textContent).toContain("Bimbel Prima");
    const logo = container.querySelector(
      'img[alt="Bimbel Prima"]'
    ) as HTMLImageElement | null;
    expect(logo?.src).toBe("https://cdn.example.com/logo.png");

    const photo = screen.getByAltText("Budi Santoso") as HTMLImageElement;
    expect(photo.src).toBe(
      "https://storage.example.com/avatars/budi.png?sig=abc"
    );

    expect(
      screen.getByText("Harap check-in dalam waktu 15 menit sebelum ujian.")
    ).toBeInTheDocument();
  });

  it("falls back to today's defaults when the payload omits tenant/photo/footer fields", async () => {
    mockedGet.mockResolvedValue(sampleCardData);

    const jsx = await CardPrintPage({
      searchParams: Promise.resolve({ token: "good-token", id: "reg-1" }),
    });
    const { container } = render(jsx as ReactElement);

    expect(container.textContent).toContain("Abak Academy");
    expect(container.querySelector('img[alt="Abak Academy"]')).toBeNull();
    expect(
      container.querySelector('svg[aria-label="abak academy"]')
    ).not.toBeNull();
    expect(screen.queryByAltText("Budi Santoso")).toBeNull();
  });
});
