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

function assertEmpty(container: HTMLElement) {
  const text = container.textContent ?? "";
  expect(text).not.toContain("Budi Santoso");
  expect(text).not.toContain("95");
  expect(text).not.toContain("ABK/2026/0001/000123");
  expect(text).not.toContain("Ujian Matematika Dasar");
}

describe("CertificatePrintPage", () => {
  beforeEach(() => {
    mockedGet.mockReset();
  });

  it("renders an empty document when no token is present (FR-22)", async () => {
    const jsx = await CertificatePrintPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx as ReactElement);

    expect(mockedGet).not.toHaveBeenCalled();
    assertEmpty(container);
  });

  it("renders the same empty document for a rejected token (FR-23)", async () => {
    mockedGet.mockResolvedValue(null);

    const jsx = await CertificatePrintPage({
      searchParams: Promise.resolve({ token: "not-a-real-token", id: "session-1" }),
    });
    const { container } = render(jsx as ReactElement);

    assertEmpty(container);
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
