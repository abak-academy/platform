import type { PublicEdition, PublicEvent } from "@/lib/types";

export const edition = (over: Partial<PublicEdition> = {}): PublicEdition => ({
  product_id: "p1",
  exam_ids: ["x1"],
  title: "ESOLIO SD 2026",
  stages: ["SD"],
  grade_min: 1,
  grade_max: 6,
  price: 150000,
  image_url: "",
  phase: "open",
  scheduled_at: "2026-12-05T02:00:00Z",
  scheduled_end_at: null,
  available_until: null,
  ...over,
});

export const event = (over: Partial<PublicEvent> = {}): PublicEvent => ({
  slug: "esolio",
  name: "ESOLIO",
  short_name: null,
  scope: "national",
  subjects: ["science"],
  tone: "brand",
  illustration: "science",
  logo_url: "",
  official_url: null,
  full_name: "English Science Olympiad",
  tagline: "Sains dalam bahasa Inggris",
  levels_label: "SD–SMA",
  story: ["Paragraf satu."],
  rounds: ["Penyisihan", "Final"],
  categories: [{ name: "Sains", detail: "Fisika dan biologi" }],
  facts: [{ label: "Format", value: "Online" }],
  schedule: [{ label: "Pendaftaran", starts_on: "2026-11-01", ends_on: "2026-11-30", state: "ongoing" }],
  open_editions: [],
  ...over,
});
