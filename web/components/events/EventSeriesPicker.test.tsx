import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { EventSeriesPicker } from "./EventSeriesPicker";
import { event } from "./fixtures";

const events = [event(), event({ slug: "ikmc", name: "IKMC Indonesia", short_name: "IKMC", logo_url: "logos/ikmc.png" })];

describe("EventSeriesPicker", () => {
  it("renders one radio per event, using short_name when set, with the selected one checked", () => {
    render(<EventSeriesPicker events={events} selected="ikmc" onSelect={() => {}} />);
    expect(screen.getAllByRole("radio")).toHaveLength(2);
    expect(screen.getByRole("radio", { name: /IKMC/ })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("radio", { name: /ESOLIO/ })).toHaveAttribute("aria-checked", "false");
    expect(screen.queryByText(/IKMC Indonesia/)).toBeNull();
  });

  it("selects on click and with arrow keys", async () => {
    const onSelect = vi.fn();
    render(<EventSeriesPicker events={events} selected="esolio" onSelect={onSelect} />);
    await userEvent.click(screen.getByRole("radio", { name: /IKMC/ }));
    expect(onSelect).toHaveBeenLastCalledWith("ikmc");
    screen.getByRole("radio", { name: /ESOLIO/ }).focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(onSelect).toHaveBeenLastCalledWith("ikmc");
  });
});
