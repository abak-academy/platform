import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Package, Receipt } from "lucide-react";
import { QuickActionTiles } from "./QuickActionTiles";

describe("QuickActionTiles", () => {
  it("renders one navigable link per action", () => {
    render(
      <QuickActionTiles
        title="Akses Cepat"
        actions={[
          { icon: Package, label: "Kelola Produk", href: "/admin/products" },
          { icon: Receipt, label: "Lihat Pesanan", href: "/admin/orders" },
        ]}
      />,
    );

    expect(screen.getByText("Akses Cepat")).toBeTruthy();
    // Links, not buttons — a tile must survive a middle-click.
    expect(screen.getByRole("link", { name: "Kelola Produk" })).toHaveAttribute(
      "href",
      "/admin/products",
    );
    expect(screen.getByRole("link", { name: "Lihat Pesanan" })).toHaveAttribute(
      "href",
      "/admin/orders",
    );
  });

  it("renders nothing at all when there are no actions", () => {
    const { container } = render(<QuickActionTiles title="Akses Cepat" actions={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
