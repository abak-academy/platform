import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SchoolBand } from "./SchoolBand";
import { SITE_CONFIG } from "@/lib/site-config";

describe("SchoolBand", () => {
  it("opens WhatsApp with the partnership message and has no form", () => {
    const { container } = render(<SchoolBand />);
    const cta = screen.getByRole("link", { name: "Ajukan kerja sama" });
    const url = new URL(cta.getAttribute("href")!);
    expect(url.origin + url.pathname).toBe(`https://wa.me/${SITE_CONFIG.whatsappNumber}`);
    expect(url.searchParams.get("text")).toBe("Halo Abak Academy, saya mewakili sekolah/lembaga dan ingin mengajukan kerja sama.");
    expect(container.querySelector("form")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("lists four capability tiles and links to the schedule", () => {
    render(<SchoolBand />);
    for (const label of ["Import CSV siswa", "Pesan ujian massal", "Monitor sesi live", "Laporan sekolah"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.getByRole("link", { name: "Lihat jadwal" })).toHaveAttribute("href", "/event");
  });
});
