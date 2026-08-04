export type ProductType = "book" | "course" | "exam" | "merchandise" | "medal";

export type ProductStatus = "draft" | "published" | "hidden" | "archived";

export type OrderStatus =
  | "cart"
  | "payment_pending"
  | "paid"
  | "processing"
  | "shipped"
  | "completed"
  | "payment_expired"
  | "cancelled";

// "failed" is a *payment* failure (payment_expired); "shipment_failed" is a
// courier failure. They are different columns and must not be conflated.
export type AdminOrderFilterStatus = "all" | "pending" | "paid" | "processing" | "shipped" | "failed" | "refunded" | "shipment_failed";

export interface School {
  id: string;
  name: string;
  code?: string;
  npsn?: string;
  school_types?: string[];
  alamat?: string;
  status?: string;
  student_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface AdminSchoolInput {
  name: string;
  code: string;
  npsn?: string;
  school_types?: string[];
  alamat?: string;
}

export interface AdminSchoolUpdateInput {
  name?: string;
  code?: string;
  npsn?: string;
  school_types?: string[];
  alamat?: string;
}

export interface AdminStudent {
  id: string;
  name: string;
  /** Nullable: some accounts have no username on file. */
  username?: string | null;
  jenjang: string;
  email?: string;
  status: string;
  grade?: number;
  provinsi_id?: string;
  kota_id?: string;
  kecamatan_id?: string;
  kode_pos?: string;
  /** Linked school; absent when the registrant has no school on file. */
  school_name?: string | null;
  /** What a self-registering user typed when their school wasn't listed. */
  unlisted_school_name?: string | null;
  created_at: string;
}

export interface CrossSchoolStudent extends AdminStudent {
  school_id: string | null;
  school_name: string | null;
}

export interface StudentRegistrationInput {
  name: string;
  jenjang: string;
  email?: string;
  dob?: string;
  gender?: string;
  grade?: number;
  alamat_domisili?: string;
  target_exam?: string;
  provinsi_id?: string;
  kota_id?: string;
  kecamatan_id?: string;
  kode_pos?: string;
}

export interface StudentRegistrationResult extends AdminStudent {
  /** Registration always generates one, unlike an arbitrary existing account. */
  username: string;
  temp_password: string;
}

export interface StudentCredentials {
  username: string;
  temp_password: string;
}

export interface User {
  id: string;
  email?: string;
  username?: string;
  name?: string;
  role?: string;
  school_id?: string;
  unlisted_school_name?: string | null;
  auth_provider?: "password" | "google";
  status?: string;
  otp_enabled?: boolean;
  phone?: string;
  nis?: string;
  grade?: number;
  target_exam?: string;
  alamat_domisili?: string;
  dob?: string;
  gender?: string;
  jenjang?: string | null;
  provinsi_id?: string | null;
  kota_id?: string | null;
  kecamatan_id?: string | null;
  kode_pos?: string | null;
  photo_url?: string;
  created_at?: string;
  updated_at?: string;
}

export interface LoginResponse {
  access_token?: string;
  refresh_token?: string;
  user?: User;
  otp_required?: boolean;
  pending_token?: string;
}

export interface ProductSpec {
  key: string;
  label: string;
  value: string;
}

export interface Product {
  id: string;
  type: ProductType;
  name: string;
  description?: string;
  price: number;
  stock?: number;
  status?: ProductStatus;
  weight_grams?: number;
  image_url?: string;
  specs?: ProductSpec[];
  available_from?: string | null;
  available_until?: string | null;
  course_ids?: string[];
  exam_ids?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface AdminCreateProductInput {
  type: ProductType;
  name: string;
  description?: string;
  price: number;
  stock?: number;
  weight_grams?: number;
  image_url?: string;
  specs?: ProductSpec[];
  available_from?: string | null;
  available_until?: string | null;
  course_ids?: string[];
  exam_ids?: string[];
}

export interface AdminUpdateProductInput {
  name?: string;
  description?: string;
  price?: number;
  stock?: number;
  status?: ProductStatus;
  weight_grams?: number;
  image_url?: string;
  specs?: ProductSpec[];
  available_from?: string | null;
  available_until?: string | null;
  course_ids?: string[];
  exam_ids?: string[];
}

export interface OrderItem {
  id: string;
  order_id: string;
  product_id: string;
  product_type: string;
  name: string;
  unit_price: number;
  qty: number;
  jumlah: number;
  weight_grams?: number;
  fulfilled_at?: string;
  created_at?: string;
}

export interface CourierRate {
  courier: string;
  service: string;
  estimated_days: number;
  price: number;
  is_estimate?: boolean;
}

export interface OrderShipmentEvent {
  id: string;
  order_id: string;
  status: string;
  courier_waybill_id?: string | null;
  courier_driver_name?: string | null;
  occurred_at: string;
  created_at: string;
}

export interface Order {
  id: string;
  student_id: string;
  /** Resolved server-side (FR-33/FR-35); fallback text when the student row is missing. */
  student_name?: string;
  status: OrderStatus;
  subtotal: number;
  discount: number;
  shipping_cost: number;
  total: number;
  promo_code_id?: string;
  shipping_address?: Record<string, string> | null;
  selected_courier?: string;
  selected_service?: string;
  is_estimate?: boolean;
  tracking_number?: string;
  shipped_at?: string;
  biteship_order_id?: string | null;
  shipment_status?: string | null;
  tracking_url?: string | null;
  waybill_source?: "biteship" | "manual" | null;
  courier_code?: string | null;
  courier_service_code?: string | null;
  shipment_events?: OrderShipmentEvent[];
  gateway_ref?: string;
  payment_method?: string;
  payment_proof_url?: string | null;
  payment_expires_at?: string;
  paid_at?: string;
  invoice_url?: string;
  checked_out_at?: string;
  completed_at?: string;
  cancelled_at?: string;
  cancellation_reason?: string;
  created_at?: string;
  updated_at?: string;
  items?: OrderItem[];
}

export interface Course {
  id: string;
  title: string;
  level?: string;
  subject?: string;
  instructor_name?: string;
  created_at?: string;
  updated_at?: string;
}

export interface CourseSection {
  id: string;
  course_id: string;
  title: string;
  position?: number;
  lessons?: Lesson[];
  created_at?: string;
}

export interface Lesson {
  id: string;
  section_id: string;
  title: string;
  video_url?: string;
  duration_seconds?: number;
  position?: number;
  completed?: boolean;
  created_at?: string;
}

export interface AdminCourseDetail extends Course {
  section_count?: number;
  lesson_count?: number;
}

export interface AdminCreateCourseInput {
  title: string;
  level?: string;
  subject?: string;
  instructor_name?: string;
}

export interface AdminUpdateCourseInput {
  title?: string;
  level?: string;
  subject?: string;
  instructor_name?: string;
}

export interface AdminCreateSectionInput {
  title: string;
}

export interface AdminUpdateSectionInput {
  title: string;
}

export interface AdminCreateLessonInput {
  title: string;
  video_url?: string;
  duration?: number;
}

export interface AdminUpdateLessonInput {
  title?: string;
  video_url?: string;
  duration?: number;
}

export interface AdminReorderSectionsInput {
  section_ids: string[];
}

export interface AdminReorderLessonsInput {
  lesson_ids: string[];
}

export interface CourseSession {
  id: string;
  student_id: string;
  course_id: string;
  order_id?: string;
  status?: string;
  source?: string;
  enrolled_at?: string;
  revoked_at?: string;
  completed_lessons?: Record<string, string>;
}

export interface PromoCode {
  id: string;
  code: string;
  discount_percent?: number;
  discount_amount?: number;
  min_order_amount?: number;
  max_discount_amount?: number;
  max_uses?: number;
  used_count: number;
  expires_at?: string;
  created_at?: string;
  /** FR-13: shown to authenticated students via GET /promo-codes/active when true. */
  is_public?: boolean;
}

export interface AdminCreatePromoCodeInput {
  code: string;
  discount_percent?: number;
  discount_amount?: number;
  max_discount_amount?: number;
  min_order_amount?: number;
  max_uses?: number;
  expires_at?: string;
  is_public?: boolean;
}

export interface AdminUpdatePromoCodeInput {
  max_uses?: number;
  expires_at?: string;
  is_public?: boolean;
}

// GET /promo-codes/active wire shape (FR-11) — deliberately omits id,
// used_count, and max_uses; see backend/internal/handler/order.go
// activePublicPromoDTO.
export interface ActivePromoCode {
  code: string;
  discount_percent: number | null;
  discount_amount: number | null;
  min_order_amount: number | null;
  max_discount_amount: number | null;
  expires_at: string | null;
}

export interface RevenueByTypeItem {
  total: number;
  count: number;
}

export interface AdminRevenue {
  total: number;
  by_type: Record<string, RevenueByTypeItem>;
}

export interface PromoValidation {
  code: string;
  discount: number;
  final_total: number;
}

export interface CheckoutResult {
  snap_token: string;
  gateway_ref?: string;
  payment_url?: string;
  payment_expires_at?: string;
  // free is true when a zero-total order was settled without the payment gateway.
  free?: boolean;
}

export interface DashboardCourseSummary {
  id: string;
  title: string;
  progress: number;
  total_lessons: number;
  done_lessons: number;
  cover?: string;
}

export interface DashboardPendingOrder {
  id: string;
  product?: string;
  amount: number;
}

export interface DashboardStudySummary {
  visited_lectures: number;
  total_lectures: number;
  enrolled_courses_count: number;
  completed_courses: number;
  total_minutes: number;
}

export interface DashboardLeaderboardEntry {
  rank: number;
  name: string;
  points: number;
  is_me?: boolean;
}

export interface ExamProgressEntry {
  label: string;
  completed: number;
  in_progress: number;
}

export interface PopularLessonEntry {
  title: string;
  topics: number;
  students: number;
  duration: string;
  progress: number;
}

export interface DashboardRanking {
  position: number | null;
  points: number | null;
  leaderboard: DashboardLeaderboardEntry[];
}

export interface Dashboard {
  greeting?: string;
  enrolled_courses: DashboardCourseSummary[];
  pending_order?: DashboardPendingOrder;
  study_summary: DashboardStudySummary;
  ranking: DashboardRanking;
  exam_progress: ExamProgressEntry[];
  popular_lessons: PopularLessonEntry[];
}

export type AdminAccountRole = "super_admin" | "admin_store" | "admin_exam" | "admin_school";

export type AdminAccountStatus = "active" | "deactivated";

export interface AdminAccount {
  id: string;
  name: string;
  email?: string | null;
  role: AdminAccountRole;
  status: AdminAccountStatus;
  school_id?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AdminCreateAccountInput {
  email: string;
  name: string;
  role: AdminAccountRole;
  password: string;
  school_id?: string;
}

export interface AuditLogEntry {
  id: number;
  actor_id?: string | null;
  actor_name?: string | null;
  actor_email?: string | null;
  target_type: string;
  target_id: string;
  action: string;
  metadata?: Record<string, unknown> | null;
  created_at: string;
}

export type SystemConfig = Record<string, string>;

export type QuestionFormat = "mcq" | "multi_answer" | "short" | "fill_blank" | "essay" | "multi_blank" | "true_false";

export type SectionType = "listening" | "reading" | "writing";

export interface Test {
  id: string;
  title: string;
  subject: string;
  topic: string;
  duration_minutes: number;
  audio_url?: string | null;
  audio_play_limit?: number | null;
  section_type?: string | null;
  question_count?: number;
  created_at?: string;
}

export interface Question {
  id: string;
  format: QuestionFormat;
  body: string;
  correct_answer?: string | null;
  explanation?: string | null;
  difficulty?: string | null;
  image_url?: string | null;
  audio_url?: string | null;
  sort_order: number;
  point_correct: number;
  point_wrong: number;
  topic_id?: string | null;
  topic?: string | null;
  /** Server-assigned, monotonic, read-only. */
  question_number?: number;
  /** question-level set; `short` / `fill_blank` only. */
  accepted_answers?: string[];
  /** `true_false` only; admin payloads only. */
  statements?: { index: number; body: string; is_true: boolean; points?: number }[];
}

export interface ExamTopic {
  id: string;
  name: string;
  subject: string;
  question_count?: number;
  created_at?: string;
}

export interface BankQuestionListItem {
  question: Question;
  options: QuestionOption[];
  attached_count: number;
  blanks?: { index: number; correct_answer: string; accepted_answers?: string[]; points?: number }[];
  /** Wrapper-level, like `blanks` — the server emits it on BankQuestionListItem, never on Question. */
  in_live_exam?: boolean;
}

export interface BankQuestionListResponse {
  data: BankQuestionListItem[];
  next_cursor?: string;
  total?: number;
}

export interface QuestionOption {
  question_id: string;
  key: string;
  text: string;
  image_url?: string | null;
  is_correct: boolean;
  sort_order: number;
  /** Per-item worth when selected correctly; absent = question's point_correct. */
  points?: number;
}

export interface QuestionWithOptions {
  question: Question;
  options: QuestionOption[];
  blanks?: { index: number; correct_answer: string; accepted_answers?: string[]; points?: number }[];
}

export interface TestDetail {
  test: Test;
  questions: QuestionWithOptions[];
}

export interface AdminCreateTestInput {
  title: string;
  subject: string;
  topic: string;
  duration_minutes: number;
  audio_url?: string;
  audio_play_limit?: number;
  section_type?: string;
}

export interface AdminUpdateTestInput {
  title?: string;
  subject?: string;
  topic?: string;
  duration_minutes?: number;
  audio_url?: string;
  audio_play_limit?: number;
  section_type?: string;
}

export interface AdminQuestionOptionInput {
  key: string;
  text: string;
  image_url?: string;
  is_correct: boolean;
  sort_order: number;
  /** Per-item worth when selected correctly; absent = question's point_correct. */
  points?: number;
}

export interface AdminQuestionInput {
  format: QuestionFormat;
  body: string;
  difficulty?: string;
  explanation?: string;
  image_url?: string;
  audio_url?: string;
  correct_answer?: string;
  options?: AdminQuestionOptionInput[];
  blanks?: { index: number; correct_answer: string; accepted_answers?: string[]; points?: number }[];
  point_correct?: number;
  point_wrong?: number;
  topic_id?: string;
  accepted_answers?: string[];
  statements?: { index: number; body: string; is_true: boolean; points?: number }[];
}

export interface AdminQuestionImportResultRow {
  row_number: number;
  status: "inserted" | "error";
  question_id?: string;
  error?: string;
}

export interface AdminQuestionImportResponse {
  inserted: number;
  rows: AdminQuestionImportResultRow[];
}

export interface AdminAttachQuestionsInput {
  question_ids: string[];
}

export interface AdminReorderQuestionsInput {
  question_ids: string[];
}

export interface TestListResponse {
  data: Test[];
  next_cursor?: string;
}

export interface QuestionListResponse {
  data: QuestionWithOptions[];
  next_cursor?: string;
}

export interface Exam {
  id: string;
  title: string;
  is_free?: boolean;
  scheduled_at?: string | null;
  scheduled_end_at?: string | null;
  requires_checkin?: boolean;
  allow_leaderboard?: boolean;
  cdn_bundle?: boolean;
  bundle_url?: string | null;
  bundle_generated_at?: string | null;
  check_in_window_minutes?: number | null;
  grace_window_minutes?: number | null;
  max_attempts?: number | null;
  timer_mode?: string;
  duration_minutes?: number | null;
  randomize?: boolean;
  result_config?: string;
  result_release_at?: string | null;
  certificate_enabled?: boolean;
  certificate_template?: string;
  certificate_background_key?: string | null;
  certificate_layout?: CertificateLayout | null;
  certificate_design_updated_at?: string | null;
  status?: string;
  mode?: string;
  created_at?: string;
  // end_screen_image_url/end_screen_promo_text are the single admin-configured
  // post-submit image/promo block (FR-38/FR-39) — no templating, one of each.
  end_screen_image_url?: string | null;
  end_screen_promo_text?: string | null;
}

export interface ExamListItem extends Exam {
  has_published_product?: boolean;
}

export interface ExamTestEntry {
  id: string;
  exam_id: string;
  test_id: string;
  sort_order: number;
  test: {
    id: string;
    title: string;
    subject: string;
    topic?: string | null;
    duration_minutes?: number | null;
    question_count: number;
  };
}

export interface ExamDetail extends ExamListItem {
  tests: ExamTestEntry[];
}

export type ExamResultConfig = "hidden" | "score_only" | "score_pembahasan";

export interface CreateExamPayload {
  title: string;
  scheduled_at?: string | null;
  scheduled_end_at?: string | null;
  timer_mode?: string;
  duration_minutes?: number | null;
  is_free?: boolean;
  requires_checkin?: boolean;
  allow_leaderboard?: boolean;
  randomize?: boolean;
  mode?: string;
  result_config?: ExamResultConfig;
  result_release_at?: string | null;
  check_in_window_minutes?: number | null;
  grace_window_minutes?: number | null;
  max_attempts?: number | null;
}

export interface UpdateExamPayload {
  title?: string;
  scheduled_at?: string | null;
  scheduled_end_at?: string | null;
  timer_mode?: string;
  duration_minutes?: number | null;
  is_free?: boolean;
  requires_checkin?: boolean;
  allow_leaderboard?: boolean;
  randomize?: boolean;
  mode?: string;
  result_config?: ExamResultConfig;
  result_release_at?: string | null;
  check_in_window_minutes?: number | null;
  grace_window_minutes?: number | null;
  max_attempts?: number | null;
  end_screen_image_url?: string | null;
  end_screen_promo_text?: string | null;
}

// ── Certificate design (admin editor, FR-17/18/25) ───────────────────────
// Mirrors service.Layout / service.LayoutField / service.CertificateDesignResponse
// in the backend (internal/service/certificate_layout.go, internal/service/exam.go).

export interface CertificateLayoutField {
  id: string;
  kind?: "text" | "image";
  name?: string;
  content?: string;
  x_mm: number;
  y_mm: number;
  w_mm: number;
  align: string;
  font?: string;
  weight?: string;
  italic?: boolean;
  size_pt?: number;
  color?: string;
  visible: boolean;
  h_mm?: number;
  asset_key?: string | null;
}

export interface CertificateLayout {
  page: { width_mm: number; height_mm: number };
  background: { kind: string; ref: string };
  fields: CertificateLayoutField[];
  signature_key?: string | null;
}

export interface CertificateDesign {
  template: string;
  background_key: string | null;
  background_url: string | null;
  signature_url: string | null;
  layout: CertificateLayout;
  presets?: CertificatePreset[];
  asset_urls?: Record<string, string>;
}

export interface CertificatePreset {
  template: string;
  background_url: string;
  layout: CertificateLayout;
}

export interface CertificateDesignInput {
  template: string;
  background_key: string | null;
  layout: CertificateLayout;
  // FE-serialized self-contained HTML with {{token}} placeholders (async
  // redesign 2026-08-02), produced by POSTing layout to
  // /api/admin/certificate-template — the worker substitutes verified DB
  // values into it at generation time. Empty only means "not yet computed
  // for this save", never a deliberate clear.
  template_html?: string;
}

// ── Session engine types (FR26) ──────────────────────────────────────────
// These mirror the backend ExamSession / ExamSessionAnswer / SessionViolationLog
// models but strip is_correct/correct_answer fields the server keeps private.

export interface SessionQuestionOption {
  key: string;
  text: string;
  image_url?: string | null;
  sort_order: number;
}

export interface SessionQuestion {
  id: string;
  test_id: string;
  format: QuestionFormat;
  body: string;
  explanation?: string | null;
  difficulty?: string | null;
  image_url?: string | null;
  audio_url?: string | null;
  sort_order: number;
  options: SessionQuestionOption[];
  blanks?: number[];
  /** `true_false` only — bodies in index order, never truth values. */
  statements?: { index: number; body: string }[];
}

export interface SessionTest {
  id: string;
  title: string;
  subject: string;
  questions: SessionQuestion[];
  section_type?: string | null;
  duration_minutes?: number | null;
  audio_url?: string | null;
  audio_play_limit?: number | null;
  status?: string;
  remaining_seconds?: number;
}

export interface SessionStartPayload {
  session_id: string;
  remaining_seconds: number;
  timer_mode: string;
  duration_minutes?: number | null;
  tests: SessionTest[];
  mode?: string;
  active_test_id?: string | null;
}

export interface SessionAnswer {
  question_id: string;
  answer?: string | null;
  flagged_for_review?: boolean;
}

export interface SessionState extends SessionStartPayload {
  registration_id: string;
  status: string;
  started_at: string;
  submitted_at?: string | null;
  extended_until?: string | null;
  last_saved_at?: string | null;
  answers: SessionAnswer[];
  /** Last server-persisted question index (FR-35/FR-36). Never read from browser storage. */
  current_position?: number | null;
}

export interface SessionAnswerInput {
  question_id: string;
  answer: string;
  flagged_for_review?: boolean;
}

export interface SaveAnswersRequest {
  answers: SessionAnswerInput[];
  current_position?: number | null;
}

export interface SubmitResult {
  submitted: boolean;
  score?: number | null;
  total?: number;
}

export interface CheckInResult {
  checked_in: boolean;
  checked_in_at: string;
}

// ── Session monitor types (Slice 7) ────────────────────────────────────────

export interface AdvanceSectionResult {
  mode?: string;
  active_test_id?: string | null;
  completed: boolean;
  tests: SessionTest[];
}

export type SessionMonitorStatus =
  | "registered"
  | "checked_in"
  | "in_progress"
  | "overdue"
  | "submitted";

export interface SessionMonitorRow {
  registration_id: string;
  student_id: string;
  student_name: string;
  school_name: string | null;
  status: SessionMonitorStatus;
  answers_saved: number;
  total_questions: number;
  checked_in_at: string | null;
  last_saved_at: string | null;
  violation_count: number;
  session_id: string | null;
  admin_submitted: boolean;
  extended_until: string | null;
  active_section_test_id?: string | null;
  active_section_title?: string | null;
  active_section_started_at?: string | null;
  active_section_duration_minutes?: number | null;
  active_section_extended_until?: string | null;
  active_section_remaining_seconds?: number;
}

export interface SessionMonitorExam {
  id: string;
  title: string;
  scheduled_at: string | null;
  duration_minutes: number | null;
  grace_window_minutes: number | null;
  status: string;
}

export interface ViolationRecent {
  session_id: string;
  student_name: string;
  count: number;
  latest_type: string;
  latest_occurred_at: string;
}

export interface SessionMonitorResponse {
  exam: SessionMonitorExam;
  rows: SessionMonitorRow[];
  violations_recent: ViolationRecent[];
}

export interface SessionViolationLog {
  id: string;
  session_id: string;
  student_id: string;
  violation_type: string;
  occurred_at: string;
}

export interface RegistrationListItem {
  id: string;
  student_id: string;
  exam_id: string;
  token: string;
  card_key: string | null;
  checked_in_at: string | null;
  attempts_used: number;
  status: string;
  created_at: string;
  exam_title: string;
  scheduled_at: string | null;
  scheduled_end_at: string | null;
  is_free: boolean;
  requires_checkin: boolean;
  check_in_window_minutes: number | null;
  duration_minutes: number | null;
  session_id: string | null;
  max_attempts: number | null;
}

export interface RegistrationDetail {
  id: string;
  student_id: string;
  exam_id: string;
  token: string;
  card_key: string | null;
  checked_in_at: string | null;
  attempts_used: number;
  status: string;
  created_at: string;
  participant_number: number | null;
  participant_no: string;
  subject: string;
  platform: string;
  exam: {
    id: string;
    title: string;
    scheduled_at: string | null;
    scheduled_end_at: string | null;
    requires_checkin: boolean;
    check_in_window_minutes: number | null;
    timer_mode: string;
    duration_minutes: number | null;
    result_config: string;
  };
}

// ── Result & grading types (Slice 5) ─────────────────────────────────────

export interface ResultTopicRow {
  test_id: string;
  title: string;
  subject: string;
  topic: string;
  section_type?: string | null;
  earned: number;
  max: number;
}

export interface ResultPembahasanItem {
  question_id: string;
  body: string;
  format: QuestionFormat;
  your_answer?: string | null;
  correct_answer?: string | null;
  is_correct?: boolean | null;
  explanation?: string | null;
}

interface SessionResultCounts {
  score: number;
  correct_count: number;
  wrong_count: number;
  empty_count: number;
  rank: number;
}

// EndScreenFields carries the FR-38/FR-39 post-submit content — present on
// every gate state (like certificate_url) since it's shown regardless of
// result-visibility config; absent when the exam has none configured.
interface EndScreenFields {
  end_screen_image_url?: string | null;
  end_screen_promo_text?: string | null;
}

export type SessionResult =
  | ({ state: "hidden"; certificate_url?: string | null } & EndScreenFields)
  | ({ state: "grading"; certificate_url?: string | null } & EndScreenFields)
  | ({ state: "locked"; result_release_at: string; certificate_url?: string | null } & EndScreenFields)
  | ({ state: "result"; result_config: "score_only" } & SessionResultCounts & { certificate_url?: string | null } & EndScreenFields)
  | ({
      state: "result";
      result_config: "score_pembahasan";
      breakdown: ResultTopicRow[];
      pembahasan: ResultPembahasanItem[];
    } & SessionResultCounts & { certificate_url?: string | null } & EndScreenFields);

export interface GradingSessionItem {
  session_id: string;
  student_id: string;
  student_name: string;
  submitted_at?: string | null;
  ungraded_essay_count: number;
}

export interface GradingEssayItem {
  question_id: string;
  body: string;
  answer?: string | null;
  point_correct: number;
  score?: number | null;
  grader_comment?: string | null;
  graded_at?: string | null;
}

export interface ExamLeaderboardEntry {
  rank: number;
  session_id: string;
  student_id: string;
  student_name: string;
  score: number;
}

// ExamRosterEntry is one row of the admin participant roster (FR-32,
// GET /admin/exams/:id/registrations). participant_number/participant_no are
// empty/absent for registrations predating the FR-24 backfill.
export interface ExamRosterEntry {
  registration_id: string;
  student_id: string;
  student_name: string;
  student_username?: string | null;
  participant_number?: number | null;
  participant_no: string;
  status: string;
  checked_in_at?: string | null;
  // token is the exam check-in credential (FR-47/FR-48, NFR-S7) — masked by
  // default in the roster UI and revealed only via an explicit per-row toggle.
  token: string;
}

export interface ScoreBucket {
  label: string;
  count: number;
}

export interface ExamAnalytics {
  average_score: number;
  completion_rate: number;
  distribution: ScoreBucket[];
}

// ── Province/city/district reference types (Task 5/17) ──────────────────────

export interface Province {
  id: string;
  name: string;
}

export interface City {
  id: string;
  province_id: string;
  name: string;
}

export interface District {
  id: string;
  city_id: string;
  name: string;
}

// ── Admin Results (FR-SCHOOL-08) ───────────────────────────────────────────

export interface AdminResultRow {
  session_id: string;
  student_name: string;
  school_name?: string | null;
  username?: string | null;
  score: number;
  submitted_at: string;
}

export interface AdminResultDetail {
  session_id: string;
  student_name: string;
  username?: string | null;
  score: number;
  submitted_at: string;
  result_config: string;
  correct_count: number;
  wrong_count: number;
  empty_count: number;
  breakdown?: ResultTopicRow[];
  pembahasan?: ResultPembahasanItem[];
}

// Generic job row from the backend job table. Mirrors service.JobResponse.
// Terminal statuses observed in worker/student_bulk.go: "succeeded" and "failed".
export interface JobStatus {
  id: string;
  type: string;
  status: "queued" | "running" | "succeeded" | "failed" | string;
  progress: number;
  result_url: string | null;
  error: string | null;
  created_at: string;
  updated_at: string;
}

export interface OrderTrackingEntry {
  status: string;
  note?: string;
  occurred_at: string;
  driver_name?: string;
}

export interface OrderTracking {
  waybill: string;
  courier: string;
  service: string;
  status: string;
  // "courier" when the carrier's own scan log answered, "local" when it did
  // not and this fell back to the events our webhook recorded.
  source: "courier" | "local";
  history: OrderTrackingEntry[];
}
