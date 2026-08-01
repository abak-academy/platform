import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";

import CertificatePrintPage from "./page";
import { getCertificatePrintData } from "@/lib/server/print-api";
import type { CertificatePrintData } from "@/lib/server/print-api";

vi.mock("@/lib/server/print-api", () => ({
  getCertificatePrintData: vi.fn(),
}));

const mockedGet = vi.mocked(getCertificatePrintData);

const samplePrintData: CertificatePrintData = {
  layout: {
    page: { width_mm: 297, height_mm: 210 },
    background: { kind: "builtin", ref: "classic" },
    fields: [
      { id: "student_name", x_mm: 40, y_mm: 60, w_mm: 200, align: "center", size_pt: 26, visible: true },
      { id: "exam_title", x_mm: 40, y_mm: 90, w_mm: 200, align: "center", size_pt: 16, visible: true },
      { id: "certificate_number", x_mm: 40, y_mm: 180, w_mm: 200, align: "center", size_pt: 10, visible: true },
      { id: "score", x_mm: 40, y_mm: 120, w_mm: 200, align: "center", size_pt: 14, visible: true, content: "{{score}}" },
    ],
  },
  values: {
    student_name: "Budi Santoso",
    exam_title: "Ujian Matematika Dasar",
    certificate_number: "ABK/2026/0001/000123",
    score: "95",
  },
  certificate_number: "ABK/2026/0001/000123",
  background_url: "https://example.test/bg.png",
  image_urls: {},
};

describe("CertificatePrintPage", () => {
  beforeEach(() => {
    mockedGet.mockReset();
  });

  // notFound() throws (caught by Next's rendering pipeline in production and
  // turned into a 404 response with an empty body via app/documents/not-found.tsx)
  // rather than returning null: a 200-with-empty-body was indistinguishable
  // from success to Gotenberg, which cached the resulting blank PDF forever
  // (NFR-R1). See the fix's comment on CertificatePrintPage.
  it("signals failure via notFound() when no token is present (FR-22, NFR-R1)", async () => {
    await expect(
      CertificatePrintPage({ searchParams: Promise.resolve({}) })
    ).rejects.toMatchObject({ digest: "NEXT_HTTP_ERROR_FALLBACK;404" });

    expect(mockedGet).not.toHaveBeenCalled();
  });

  it("signals failure via notFound() for a rejected token (FR-23, NFR-R1)", async () => {
    mockedGet.mockResolvedValue(null);

    await expect(
      CertificatePrintPage({
        searchParams: Promise.resolve({ token: "not-a-real-token", id: "session-1" }),
      })
    ).rejects.toMatchObject({ digest: "NEXT_HTTP_ERROR_FALLBACK;404" });
  });

  it("renders server-authored values for a valid token (FR-24)", async () => {
    mockedGet.mockResolvedValue(samplePrintData);

    const jsx = await CertificatePrintPage({
      searchParams: Promise.resolve({ token: "good-token", id: "session-1" }),
    });
    render(jsx as ReactElement);

    expect(mockedGet).toHaveBeenCalledWith("session-1", "good-token");
    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText("Ujian Matematika Dasar")).toBeInTheDocument();
    expect(screen.getByText("ABK/2026/0001/000123")).toBeInTheDocument();
    expect(screen.getByText("95")).toBeInTheDocument();
  });
});
