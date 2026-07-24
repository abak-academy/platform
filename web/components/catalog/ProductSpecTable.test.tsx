import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProductSpecTable } from "./ProductSpecTable";

describe("ProductSpecTable", () => {
  it("renders label and value pairs", () => {
    render(
      <ProductSpecTable
        specs={[
          { key: "penerbit", label: "Perusahaan Penerbit", value: "Yayasan Abak Cendekia" },
          { key: "jenis_cover", label: "Jenis Cover", value: "Hard Cover" },
        ]}
      />,
    );

    expect(screen.getByText("Perusahaan Penerbit")).toBeTruthy();
    expect(screen.getByText("Yayasan Abak Cendekia")).toBeTruthy();
    expect(screen.getByText("Hard Cover")).toBeTruthy();
  });

  it("skips rows with a blank value", () => {
    render(
      <ProductSpecTable
        specs={[
          { key: "penerbit", label: "Perusahaan Penerbit", value: "Yayasan Abak Cendekia" },
          { key: "isbn", label: "ISBN", value: "" },
        ]}
      />,
    );

    expect(screen.queryByText("ISBN")).toBeNull();
  });

  it("renders nothing at all when there is no displayable row", () => {
    const { container } = render(
      <ProductSpecTable specs={[{ key: "isbn", label: "ISBN", value: "" }]} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when specs are absent", () => {
    const { container } = render(<ProductSpecTable />);
    expect(container.firstChild).toBeNull();
  });
});
