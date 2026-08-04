import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { OrderRow, STALE_AFTER_DAYS } from "./OrderRow";
import type { Order, OrderStatus } from "@/lib/types";

// shipment-status.ts imports the bare `t` from this module, so the mock has to
// provide it too or shipmentStatusLabel() throws.
vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key, lang: "id" }),
  t: (_lang: string, key: string) => key,
}));

const DAY_MS = 86_400_000;

function daysAgo(days: number): string {
  return new Date(Date.now() - days * DAY_MS).toISOString();
}

function makeOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: "11111111-2222-3333-4444-a3f91c04",
    student_id: "stu-1",
    student_name: "Rani Puspita",
    status: "paid" as OrderStatus,
    subtotal: 485000,
    discount: 0,
    shipping_cost: 0,
    total: 485000,
    created_at: daysAgo(1),
    items: [
      {
        id: "oi-1",
        order_id: "11111111-2222-3333-4444-a3f91c04",
        product_id: "p-1",
        product_type: "book",
        name: "Buku TKA Saintek",
        unit_price: 485000,
        qty: 1,
        jumlah: 485000,
      },
    ],
    ...overrides,
  };
}

function renderRow(props: Partial<React.ComponentProps<typeof OrderRow>> = {}) {
  const onOpen = props.onOpen ?? vi.fn();
  const onTrack = props.onTrack ?? vi.fn();
  render(
    <table>
      <tbody>
        <OrderRow
          order={props.order ?? makeOrder()}
          onOpen={onOpen}
          onTrack={onTrack}
          primaryAction={props.primaryAction}
          menuActions={props.menuActions ?? []}
        />
      </tbody>
    </table>,
  );
  return { onOpen, onTrack };
}

describe("OrderRow courier sub-line", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders a track button when the order has a tracking number", () => {
    renderRow({
      order: makeOrder({
        status: "shipped",
        tracking_number: "JP1234567890",
        shipment_status: "in_transit",
      }),
    });

    const track = screen.getByTestId("row-track-button");
    expect(track.tagName).toBe("BUTTON");
    expect(screen.getByText("JP1234567890")).toBeInTheDocument();
    expect(screen.queryByTestId("row-shipment-status")).not.toBeInTheDocument();
  });

  it("clicking the track button calls onTrack and does not open the order", () => {
    const { onOpen, onTrack } = renderRow({
      order: makeOrder({
        status: "shipped",
        tracking_number: "JP1234567890",
        shipment_status: "in_transit",
      }),
    });

    fireEvent.click(screen.getByTestId("row-track-button"));

    expect(onTrack).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("renders plain text, not a button, when there is no tracking number", () => {
    renderRow({ order: makeOrder({ status: "paid", shipment_status: "confirmed" }) });

    expect(screen.queryByTestId("row-track-button")).not.toBeInTheDocument();
    const status = screen.getByTestId("row-shipment-status");
    expect(status.tagName).toBe("SPAN");
  });

  it("renders a failed shipment as plain destructive text even when a resi exists", () => {
    renderRow({
      order: makeOrder({
        status: "shipped",
        tracking_number: "JP1234567890",
        shipment_status: "courier_not_found",
      }),
    });

    expect(screen.queryByTestId("row-track-button")).not.toBeInTheDocument();
    const status = screen.getByTestId("row-shipment-status");
    expect(status.tagName).toBe("SPAN");
    expect(status).toHaveClass("text-destructive");
  });
});

describe("OrderRow age staleness", () => {
  it(`reddens the age of a payment_pending order older than ${STALE_AFTER_DAYS} days`, () => {
    renderRow({
      order: makeOrder({ status: "payment_pending", created_at: daysAgo(STALE_AFTER_DAYS + 0.5) }),
    });

    expect(screen.getByTestId("row-age")).toHaveClass("text-destructive");
  });

  it("does not redden a fresh payment_pending order", () => {
    renderRow({ order: makeOrder({ status: "payment_pending", created_at: daysAgo(1) }) });

    expect(screen.getByTestId("row-age")).not.toHaveClass("text-destructive");
  });

  it("never reddens a completed order however old", () => {
    renderRow({ order: makeOrder({ status: "completed", created_at: daysAgo(400) }) });

    expect(screen.getByTestId("row-age")).not.toHaveClass("text-destructive");
  });

  it("never reddens a completed order whose shipment failed", () => {
    renderRow({
      order: makeOrder({
        status: "completed",
        created_at: daysAgo(400),
        shipment_status: "returned",
      }),
    });

    expect(screen.getByTestId("row-age")).not.toHaveClass("text-destructive");
  });

  it("reddens a stale order whose shipment failed", () => {
    renderRow({
      order: makeOrder({
        status: "shipped",
        created_at: daysAgo(10),
        shipment_status: "courier_not_found",
      }),
    });

    expect(screen.getByTestId("row-age")).toHaveClass("text-destructive");
  });
});

describe("OrderRow actions", () => {
  it("opens the order from the order-number button, not from a row that claims to be one", () => {
    const { onOpen } = renderRow();

    const row = screen.getByRole("row");
    expect(row).not.toHaveAttribute("role", "button");
    expect(row).not.toHaveAttribute("tabindex");

    fireEvent.click(screen.getByRole("button", { name: /orders_detail_open/ }));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it("renders the primary action and does not open the order when it is clicked", () => {
    const onClick = vi.fn();
    const { onOpen } = renderRow({ primaryAction: { label: "Kirim", onClick } });

    fireEvent.click(screen.getByRole("button", { name: "Kirim" }));

    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("marks destructive menu items with the destructive variant", async () => {
    renderRow({
      menuActions: [
        { label: "Lacak", onClick: vi.fn() },
        { label: "Refund", onClick: vi.fn(), destructive: true },
      ],
    });

    fireEvent.pointerDown(screen.getByTestId("row-menu-trigger"), { button: 0 });

    expect(await screen.findByRole("menuitem", { name: "Lacak" })).toHaveAttribute(
      "data-variant",
      "default",
    );
    expect(screen.getByRole("menuitem", { name: "Refund" })).toHaveAttribute(
      "data-variant",
      "destructive",
    );
  });
});
