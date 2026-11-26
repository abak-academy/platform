import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ApiError } from "@/lib/api";

import SessionLeaderboardPage from "./page";

type LbEntry = {
  rank: number;
  session_id: string;
  student_id: string;
  student_name: string;
  score: number;
};

type LbState = {
  data: { data: LbEntry[]; next_cursor?: string } | null;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  refetch: ReturnType<typeof vi.fn>;
};

let lbState: LbState;
let lbStateNext: LbState;

vi.mock("@/lib/hooks/exam", () => ({
  useSessionLeaderboard: (_id: string, filter?: { cursor?: string }) =>
    filter?.cursor ? lbStateNext : lbState,
}));

const sampleEntries: LbEntry[] = [
  { rank: 1, session_id: "sess1", student_id: "s1", student_name: "Budi Santoso", score: 95 },
  { rank: 2, session_id: "sess2", student_id: "s2", student_name: "Siti Aminah", score: 88 },
  { rank: 3, session_id: "sess3", student_id: "s3", student_name: "Agus Wijaya", score: 82 },
];

describe("SessionLeaderboardPage", () => {
  beforeEach(() => {
    lbState = {
      data: { data: sampleEntries },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    lbStateNext = {
      data: null,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
  });

  it("renders ranked rows on success", async () => {
    render(<SessionLeaderboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    expect(screen.getByText("Siti Aminah")).toBeInTheDocument();
    expect(screen.getByText("Agus Wijaya")).toBeInTheDocument();
    expect(screen.getByText("95")).toBeInTheDocument();
  });

  it("renders the not-available message on 403 leaderboard_not_available", () => {
    lbState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new ApiError("leaderboard_not_available", "Peringkat belum tersedia", 403),
      refetch: vi.fn(),
    };

    render(<SessionLeaderboardPage />);

    expect(screen.getByText("Peringkat belum tersedia")).toBeInTheDocument();
  });

  it("appends the next page when Load more is clicked", async () => {
    lbState.data = { data: sampleEntries, next_cursor: "82,sess3" };
    lbStateNext.data = {
      data: [
        { rank: 4, session_id: "sess4", student_id: "s4", student_name: "Dewi Lestari", score: 75 },
      ],
    };

    render(<SessionLeaderboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Muat lebih banyak" }));

    await waitFor(() => {
      expect(screen.getByText("Dewi Lestari")).toBeInTheDocument();
    });
    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Muat lebih banyak" })).not.toBeInTheDocument();
  });

  it("hides Load more when there is no next cursor", async () => {
    render(<SessionLeaderboardPage />);
    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Muat lebih banyak" })).not.toBeInTheDocument();
  });

  it("renders retake rows (same student twice) without duplicate React keys", async () => {
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    lbState.data = {
      data: [
        { rank: 1, session_id: "sess1", student_id: "s1", student_name: "Budi Santoso", score: 95 },
        { rank: 2, session_id: "sess9", student_id: "s1", student_name: "Budi Santoso", score: 88 },
      ],
    };

    render(<SessionLeaderboardPage />);
    await waitFor(() => {
      expect(screen.getAllByText("Budi Santoso")).toHaveLength(2);
    });

    const dupKeyWarning = errSpy.mock.calls.some((args) =>
      String(args[0]).includes("same key"),
    );
    errSpy.mockRestore();
    expect(dupKeyWarning).toBe(false);
  });

  it("lays the top three out as a podium ordered 2-1-3 and lists the rest on the board", async () => {
    lbState = {
      ...lbState,
      data: {
        data: [...sampleEntries, { rank: 4, session_id: "sess4", student_id: "s4", student_name: "Dewi Lestari", score: 70 }],
      },
    };
    render(<SessionLeaderboardPage />);

    const podium = await screen.findByTestId("podium");
    const names = Array.from(podium.querySelectorAll("b.truncate")).map((n) => n.textContent);
    expect(names).toEqual(["Siti Aminah", "Budi Santoso", "Agus Wijaya"]);
    const board = screen.getByRole("list");
    expect(board).toHaveTextContent("Dewi Lestari");
    expect(board).not.toHaveTextContent("Budi Santoso");
    expect(screen.getByRole("heading", { level: 1, name: "Peringkat" })).toBeInTheDocument();
  });

  it("renders no board card when everyone fits on the podium", async () => {
    render(<SessionLeaderboardPage />);
    await screen.findByTestId("podium");
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
