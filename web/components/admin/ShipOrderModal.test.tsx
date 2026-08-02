import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ShipOrderModal } from "./ShipOrderModal";
import { t } from "@/lib/i18n";
import { useUIStore } from "@/stores/ui";

function renderModal() {
  return render(
    <ShipOrderModal
      open
      onOpenChange={vi.fn()}
      orderNumber="#5d00bcc4"
      onBook={vi.fn()}
      onSubmitManual={vi.fn()}
      isPending={false}
    />,
  );
}

describe("ShipOrderModal", () => {
  const lang = useUIStore.getState().lang;

  it("does not promise a tracking-number field on the booking-choice step", async () => {
    renderModal();

    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(
      screen.queryByText(t(lang, "orders_ship_subtitle").replace("{order}", "#5d00bcc4")),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(t(lang, "orders_ship_choice_subtitle").replace("{order}", "#5d00bcc4")),
    ).toBeInTheDocument();
  });

  it("offers exactly the two shipping paths — no Cancel competing with them", () => {
    renderModal();

    const footer = document.querySelector('[data-slot="dialog-footer"]')!;
    expect([...footer.querySelectorAll("button")].map((b) => b.textContent?.trim())).toEqual([
      t(lang, "orders_ship_manual_choice"),
      t(lang, "orders_ship_book_courier"),
    ]);
  });

  it("asks for the tracking number once the manual step is open", async () => {
    renderModal();

    await userEvent.click(screen.getByRole("button", { name: t(lang, "orders_ship_manual_choice") }));

    expect(screen.getByRole("textbox")).toBeInTheDocument();
    expect(
      screen.getByText(t(lang, "orders_ship_subtitle").replace("{order}", "#5d00bcc4")),
    ).toBeInTheDocument();
  });
});
