import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ShippingAddressSummary } from "./ShippingAddressSummary";

const address = {
  penerima: "Saifullah Panca",
  telepon: "08123456789",
  alamat: "Jl. Merdeka No. 1",
  provinsi_id: "32",
  kota_id: "3273",
  kecamatan_id: "327301",
  kode_pos: "40123",
};

describe("ShippingAddressSummary", () => {
  it("shows the saved recipient, phone and street instead of a form", () => {
    render(<ShippingAddressSummary address={address} onEdit={() => {}} />);
    expect(screen.getByText("Saifullah Panca")).toBeTruthy();
    expect(screen.getByText(/08123456789/)).toBeTruthy();
    expect(screen.getByText(/Jl. Merdeka No. 1/)).toBeTruthy();
    expect(screen.queryByLabelText("Provinsi")).toBeNull();
  });

  it("asks the parent to open the form when Ubah is pressed", () => {
    const onEdit = vi.fn();
    render(<ShippingAddressSummary address={address} onEdit={onEdit} />);
    fireEvent.click(screen.getByRole("button", { name: "Ubah" }));
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("prompts to complete the address when a required part is missing", () => {
    render(
      <ShippingAddressSummary
        address={{ ...address, alamat: "" }}
        onEdit={() => {}}
      />,
    );
    expect(screen.getByText("Lengkapi alamat pengiriman untuk melanjutkan.")).toBeTruthy();
  });

  // Cek Ongkir moved next to the goods it prices. Leaving a copy here would
  // give the buyer two buttons for one action, in two places.
  it("carries no control other than Ubah", () => {
    render(<ShippingAddressSummary address={address} onEdit={() => {}} />);
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toHaveTextContent("Ubah");
  });
});
