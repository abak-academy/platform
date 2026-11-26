import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import DashboardPage from "./page";
import { greetingKey } from "./home-greeting";
import type { Dashboard, Order, Product, PublicEvent, RegistrationListItem } from "@/lib/types";

const api = vi.hoisted(() => ({ handlers: {} as Record<string, () => unknown> }));

const route = async (path: string) => {
  const key = Object.keys(api.handlers).find((k) => path.startsWith(k));
  if (!key) throw new Error(`unmocked ${path}`);
  return api.handlers[key]();
};

vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  authFetch: (path: string) => route(path),
  apiFetch: (path: string) => route(path),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));

let authStore = { user: null as { name?: string; username?: string } | null, token: "t" };
vi.mock("@/stores/auth", () => {
  const useAuthStore = (selector: (s: typeof authStore) => unknown) => selector(authStore);
  useAuthStore.getState = () => authStore;
  return { useAuthStore };
});

const course = (id: string, title: string, progress: number, done: number, total: number) => ({
  id,
  title,
  progress,
  total_lessons: total,
  done_lessons: done,
});

const baseData: Dashboard = {
  enrolled_courses: [course("c1", "Matematika Dasar", 0.5, 5, 10), course("c2", "Fisika Lanjutan", 0.3, 2, 8)],
  study_summary: { visited_lectures: 7, total_lectures: 18, enrolled_courses_count: 2, completed_courses: 0, total_minutes: 420 },
  ranking: {
    position: 3,
    points: 844,
    leaderboard: [
      { rank: 1, name: "Nadia Salsabila", points: 871, is_me: false },
      { rank: 3, name: "Budi Santoso", points: 844, is_me: true },
    ],
  },
  exam_progress: [{ label: "Tryout 1", completed: 148, in_progress: 82 }],
  popular_lessons: [{ title: "Penalaran Matematika HOTS", topics: 12, students: 240, duration: "1j 30m", progress: 0.72 }],
};

const shippedOrder: Order = {
  id: "ord-ship",
  student_id: "stu-1",
  status: "shipped",
  subtotal: 100000,
  discount: 0,
  shipping_cost: 10000,
  total: 110000,
  created_at: "2026-11-20T10:00:00Z",
  items: [{ id: "i1", order_id: "ord-ship", product_id: "p1", product_type: "book", name: "Buku Latihan", unit_price: 1, qty: 1, jumlah: 1 }],
};

const registration = (examId: string): RegistrationListItem => ({
  id: `reg-${examId}`,
  student_id: "stu-1",
  exam_id: examId,
  token: "TKN",
  card_key: null,
  checked_in_at: null,
  attempts_used: 0,
  status: "registered",
  created_at: "2026-11-01T00:00:00Z",
  exam_title: "Tryout Alpha",
  scheduled_at: "2099-01-01T00:00:00Z",
  scheduled_end_at: "2099-01-01T02:00:00Z",
  is_free: true,
  requires_checkin: false,
  check_in_window_minutes: null,
  duration_minutes: 90,
  session_id: null,
  max_attempts: 1,
  navigation_policy: "free",
  timer_mode: "overall",
  section_count: 1,
  question_count: 10,
  sections_duration_minutes: null,
});

const edition = (productId: string, examIds: string[], title: string) => ({
  product_id: productId,
  exam_ids: examIds,
  title,
  stages: ["SD"],
  grade_min: 4,
  grade_max: 6,
  price: 50000,
  image_url: "",
  phase: "open" as const,
  scheduled_at: null,
  scheduled_end_at: null,
  available_until: null,
});

const event: PublicEvent = {
  name: "Olimpiade Sains",
  illustration: "science",
  tone: "teal",
  open_editions: [edition("pe-open", ["exam-free"], "Babak Terbuka"), edition("pe-reg", ["exam-mine"], "Babak Terdaftar")],
} as unknown as PublicEvent;

const book: Product = { id: "b1", type: "book", name: "Buku Pilihan", price: 50000, stock: 5 };

const ok = <T,>(data: T) => () => Promise.resolve({ data });
const fail = (msg = "Network Error") => () => Promise.reject(new Error(msg));

function setApi(over: Record<string, () => unknown> = {}) {
  api.handlers = {
    "/students/dashboard": () => Promise.resolve(baseData),
    "/orders": ok([] as Order[]),
    "/exam/registrations": ok([] as RegistrationListItem[]),
    "/event-series": ok([event]),
    "/products": ok([book]),
    ...over,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DashboardPage />
    </QueryClientProvider>,
  );
}

const before = (a: HTMLElement, b: HTMLElement) => Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);

describe("DashboardPage", () => {
  beforeEach(() => {
    authStore = { user: { name: "Budi Santoso" }, token: "t" };
    setApi();
    vi.stubGlobal("matchMedia", (q: string) => ({ matches: true, media: q, addEventListener: () => {}, removeEventListener: () => {} }));
  });

  afterEach(() => vi.unstubAllGlobals());

  it("greets by first name", async () => {
    renderPage();
    expect(await screen.findByRole("heading", { level: 1, name: /^Selamat (pagi|siang|sore|malam), Budi$/ })).toBeInTheDocument();
  });

  it("renders sections in order: greeting, pending, shipping, carousel, events, learning, study, shop", async () => {
    setApi({
      "/students/dashboard": () => Promise.resolve({ ...baseData, pending_order: { id: "ord1", product: "Buku Soal SNBT", amount: 75000 } }),
      "/orders": ok([shippedOrder]),
    });
    renderPage();
    const greeting = await screen.findByRole("heading", { level: 1 });
    const pending = await screen.findByText(/Pembayaran tertunda/);
    const shipping = await screen.findByText("Paketmu sedang dikirim");
    const carousel = await screen.findByRole("region", { name: "Event terbaru" });
    const events = await screen.findByText("Event & kompetisi");
    const learn = await screen.findByText("Lanjut belajar");
    const study = await screen.findByText("Total jam belajar");
    const shop = await screen.findByText("Dari Toko");
    const chain = [greeting, pending, shipping, carousel, events, learn, study, shop];
    chain.slice(1).forEach((el, i) => expect(before(chain[i], el), `section ${i + 1} follows section ${i}`).toBe(true));
  });

  it("shows enrolled courses in Continue learning", async () => {
    renderPage();
    expect(await screen.findByText("Matematika Dasar")).toBeInTheDocument();
    expect(screen.getByText("Fisika Lanjutan")).toBeInTheDocument();
  });

  it("omits the shipping strip when no order is eligible", async () => {
    setApi({ "/orders": ok([{ ...shippedOrder, status: "completed" }]) });
    renderPage();
    await screen.findByText("Dari Toko");
    expect(screen.queryByText("Paketmu sedang dikirim")).not.toBeInTheDocument();
    expect(screen.queryByText("Pesananmu sedang dikemas")).not.toBeInTheDocument();
  });

  it("omits the shop when products fail to load", async () => {
    setApi({ "/products": fail() });
    renderPage();
    await screen.findByText("Lanjut belajar");
    await waitFor(() => expect(screen.queryByText("Dari Toko")).not.toBeInTheDocument());
    expect(screen.queryByText("Buku Pilihan")).not.toBeInTheDocument();
  });

  it("excludes a registered edition from the carousel", async () => {
    setApi({ "/exam/registrations": ok([registration("exam-mine")]) });
    renderPage();
    expect(await screen.findByText(/Babak Terbuka/)).toBeInTheDocument();
    expect(screen.queryByText(/Babak Terdaftar/)).not.toBeInTheDocument();
  });

  it("holds the skeletons and never shows the empty copy while registrations load", async () => {
    setApi({ "/exam/registrations": () => new Promise(() => {}) });
    renderPage();
    await screen.findByText("Lanjut belajar");
    expect(screen.getByTestId("carousel-skeleton")).toBeInTheDocument();
    expect(screen.getByTestId("events-loading")).toBeInTheDocument();
    expect(screen.queryByText("Belum ada event yang kamu ikuti.")).not.toBeInTheDocument();
  });

  it("shows a neutral line, not the empty copy, when registrations fail", async () => {
    setApi({ "/exam/registrations": fail() });
    renderPage();
    expect(await screen.findByText("Gagal memuat dashboard.")).toBeInTheDocument();
    expect(screen.queryByText("Belum ada event yang kamu ikuti.")).not.toBeInTheDocument();
    expect(await screen.findByText(/Babak Terbuka/)).toBeInTheDocument();
  });

  it("falls back to the empty carousel slide when event-series fails", async () => {
    setApi({ "/event-series": fail() });
    renderPage();
    expect(await screen.findByText("Belum ada event baru")).toBeInTheDocument();
  });

  it("does not render ranking, exam progress, popular lessons or the old hero", async () => {
    renderPage();
    await screen.findByText("Lanjut belajar");
    expect(screen.queryByText("Ujian Terbaru")).not.toBeInTheDocument();
    expect(screen.queryByText(/Peringkat saya|My Ranking/)).not.toBeInTheDocument();
    expect(screen.queryByText("Nadia Salsabila")).not.toBeInTheDocument();
    expect(screen.queryByText(/Progress ujian|Exam progress/i)).not.toBeInTheDocument();
    expect(screen.queryByText("Penalaran Matematika HOTS")).not.toBeInTheDocument();
    expect(screen.queryByText(/Pelajaran populer|Popular lessons/i)).not.toBeInTheDocument();
    expect(screen.queryByText("Tryout 1")).not.toBeInTheDocument();
  });

  it("renders the pending banner when pending_order exists", async () => {
    setApi({
      "/students/dashboard": () => Promise.resolve({ ...baseData, pending_order: { id: "ord1", product: "Buku Soal SNBT", amount: 75000 } }),
    });
    renderPage();
    expect(await screen.findByText(/Pembayaran tertunda|Payment pending/)).toBeInTheDocument();
  });

  it("omits the pending banner without pending_order", async () => {
    renderPage();
    await screen.findByText("Lanjut belajar");
    expect(screen.queryByText(/Pembayaran tertunda|Payment pending/)).not.toBeInTheDocument();
  });

  it("renders the loading skeleton while the dashboard loads", () => {
    setApi({ "/students/dashboard": () => new Promise(() => {}) });
    renderPage();
    expect(screen.getByTestId("dashboard-skeleton")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  });

  it("shows the error card and recovers on retry", async () => {
    let calls = 0;
    setApi({ "/students/dashboard": () => (++calls === 1 ? Promise.reject(new Error("Network Error")) : Promise.resolve(baseData)) });
    renderPage();
    expect(await screen.findByText(/Gagal memuat dashboard/)).toBeInTheDocument();
    expect(screen.getByText(/Network Error/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Coba lagi|Retry/ }));
    expect(await screen.findByText("Lanjut belajar")).toBeInTheDocument();
    expect(screen.queryByText(/Gagal memuat dashboard/)).not.toBeInTheDocument();
  });
});

describe("greetingKey", () => {
  it.each([
    [0, "stu_home_greet_morning"],
    [10, "stu_home_greet_morning"],
    [11, "stu_home_greet_afternoon"],
    [14, "stu_home_greet_afternoon"],
    [15, "stu_home_greet_late_afternoon"],
    [17, "stu_home_greet_late_afternoon"],
    [18, "stu_home_greet_evening"],
    [23, "stu_home_greet_evening"],
  ])("hour %i -> %s", (hour, key) => {
    expect(greetingKey(hour)).toBe(key);
  });
});
