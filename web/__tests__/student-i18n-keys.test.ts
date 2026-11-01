import { describe, it, expect } from "vitest";
import { DICT } from "@/lib/i18n";

type Dict = Record<string, string>;
const idDict = DICT.id as Dict;
const enDict = DICT.en as Dict;

const NEW_KEYS: { key: string; id: string; en: string }[] = [
  // Dashboard (page.tsx)
  { key: "greeting_morning", id: "Selamat pagi", en: "Good morning" },
  { key: "greeting_afternoon", id: "Selamat siang", en: "Good afternoon" },
  { key: "greeting_evening", id: "Selamat malam", en: "Good evening" },
  { key: "hello", id: "Halo", en: "Hello" },
  { key: "add_course", id: "Tambah Kursus", en: "Add Course" },
  { key: "dash_load_failed", id: "Gagal memuat dashboard.", en: "Failed to load dashboard." },
  { key: "dash_my_ranking", id: "Peringkat saya", en: "My Ranking" },
  { key: "dash_ranking_placeholder", id: "Total peringkat akan tersedia setelah ujian pertama.", en: "Overall ranking will be available after your first exam." },
  { key: "dash_study_hours", id: "Total jam belajar", en: "Total Study Hours" },
  { key: "dash_hours_placeholder", id: "Ringkasan waktu belajar akan hadir di sini.", en: "Study time summary will appear here." },
  { key: "dash_exam_progress", id: "Progress ujian", en: "Exam Progress" },
  { key: "dash_progress_placeholder", id: "Statistik progress ujian sedang disiapkan.", en: "Exam progress statistics are being prepared." },
  { key: "dash_tryout", id: "Tryout", en: "Tryout" },
  { key: "dash_view_all", id: "Lihat semua", en: "View all" },
  { key: "dash_explore_catalog", id: "Jelajahi katalog", en: "Explore catalog" },
  { key: "dash_catalog_desc", id: "Temukan buku, kursus, dan paket kompetisi baru.", en: "Discover new books, courses, and competition packages." },
  { key: "dash_open_catalog", id: "Buka katalog", en: "Open catalog" },
  { key: "dash_no_courses", id: "Belum ada kursus", en: "No courses yet" },
  { key: "dash_no_courses_desc", id: "Mulai belajar dengan menjelajahi katalog kami.", en: "Start learning by exploring our catalog." },
  { key: "dash_coming_soon", id: "Akan datang", en: "Coming soon" },

  // Catalog (catalog/page.tsx)
  { key: "catalog_tab_all", id: "Semua", en: "All" },
  { key: "catalog_tab_book", id: "Buku", en: "Book" },
  { key: "catalog_tab_course", id: "Kursus", en: "Course" },
  { key: "catalog_tab_competition", id: "Kompetisi", en: "Competition" },
  { key: "catalog_empty", id: "Belum ada produk pada kategori ini.", en: "No products in this category yet." },
  { key: "catalog_title", id: "Katalog", en: "Catalog" },
  { key: "catalog_subtitle", id: "Jelajahi buku, kursus, dan paket kompetisi.", en: "Browse books, courses, and competition packages." },
  { key: "catalog_load_failed", id: "Gagal memuat katalog.", en: "Failed to load catalog." },

  // Product detail (catalog/[id]/page.tsx)
  { key: "product_type_book", id: "Buku", en: "Book" },
  { key: "product_type_course", id: "Kursus", en: "Course" },
  { key: "product_type_competition", id: "Kompetisi", en: "Competition" },
  { key: "product_no_description", id: "Tidak ada deskripsi.", en: "No description available." },
  { key: "product_stock_label", id: "Stok", en: "Stock" },
  { key: "product_shipped_to_address", id: "dikirim ke alamat Anda", en: "shipped to your address" },
  { key: "product_back_catalog", id: "Katalog", en: "Catalog" },
  { key: "product_add_cart", id: "Tambah ke Keranjang", en: "Add to Cart" },
  { key: "product_buy_now", id: "Beli Sekarang", en: "Buy Now" },
  { key: "product_load_failed", id: "Gagal memuat produk.", en: "Failed to load product." },
  { key: "product_added_toast", id: "Ditambahkan ke keranjang", en: "Added to cart" },
  { key: "product_add_failed_desc", id: "Gagal menambahkan ke keranjang.", en: "Failed to add to cart." },
  { key: "product_add_failed_title", id: "Gagal menambahkan", en: "Failed to add" },

  // Courses list (courses/page.tsx)
  { key: "course_subtitle", id: "Lanjutkan belajar dari kursus yang sudah terdaftar.", en: "Continue learning from your enrolled courses." },
  { key: "course_load_failed", id: "Gagal memuat kursus.", en: "Failed to load courses." },

  // Course detail (courses/[id]/page.tsx)
  { key: "course_lesson_undone", id: "Pelajaran dibatalkan.", en: "Lesson undone." },
  { key: "course_lesson_done", id: "Pelajaran selesai.", en: "Lesson completed." },
  { key: "course_update_failed", id: "Gagal memperbarui status.", en: "Failed to update status." },
  { key: "course_not_found", id: "Kursus tidak ditemukan.", en: "Course not found." },
  { key: "course_back", id: "Kembali ke kursus", en: "Back to courses" },
  { key: "course_instructor", id: "Pengajar", en: "Instructor" },
  { key: "course_select_lesson", id: "Pilih pelajaran", en: "Select a lesson" },
  { key: "course_duration", id: "Durasi {n} menit", en: "Duration {n} min" },
  { key: "course_complete", id: "Selesai", en: "Done" },
  { key: "course_mark_complete", id: "Tandai selesai", en: "Mark complete" },

  // Cart (cart/page.tsx)
  { key: "cart_continue", id: "Lanjutkan belanja", en: "Continue shopping" },
  { key: "cart_title", id: "Keranjang", en: "Cart" },
  { key: "cart_item_count", id: "{n} item", en: "{n} item" },
  { key: "cart_load_failed", id: "Gagal memuat keranjang", en: "Failed to load cart" },
  { key: "cart_order_summary", id: "Ringkasan Pesanan", en: "Order Summary" },
  { key: "cart_promo_invalid", id: "Kode promo tidak valid", en: "Invalid promo code" },
  { key: "cart_subtotal", id: "Subtotal", en: "Subtotal" },
  { key: "cart_discount", id: "Diskon", en: "Discount" },
  { key: "cart_total", id: "Total", en: "Total" },
  { key: "cart_secure_payment", id: "Midtrans · pembayaran aman terenkripsi", en: "Midtrans · secure encrypted payment" },
  { key: "cart_empty_title", id: "Keranjang masih kosong", en: "Cart is still empty" },
  { key: "cart_empty_desc", id: "Yuk jelajahi katalog dan tambahkan buku atau kursus favoritmu.", en: "Browse the catalog and add your favorite books or courses." },
  { key: "cart_view_catalog", id: "Lihat Katalog", en: "View Catalog" },

  // Orders list (orders/page.tsx)
  { key: "orders_title", id: "Pesanan", en: "Orders" },
  { key: "orders_empty", id: "Belum ada pesanan.", en: "No orders yet." },
  { key: "orders_empty_desc", id: "Pesanan Anda akan muncul di sini setelah checkout.", en: "Your orders will appear here after checkout." },
  { key: "orders_start_shopping", id: "Mulai belanja", en: "Start shopping" },
  { key: "orders_load_failed", id: "Gagal memuat pesanan", en: "Failed to load orders" },

  // Order detail (orders/[id]/page.tsx)
  { key: "order_tl_created", id: "Pesanan dibuat", en: "Order created" },
  { key: "order_tl_checkout", id: "Checkout dimulai", en: "Checkout started" },
  { key: "order_tl_paid", id: "Pembayaran diterima", en: "Payment received" },
  { key: "order_tl_shipped", id: "Pesanan dikirim", en: "Order shipped" },
  { key: "order_tl_completed", id: "Pesanan selesai", en: "Order completed" },
  { key: "order_tl_cancelled", id: "Pesanan dibatalkan", en: "Order cancelled" },
  { key: "order_no_items", id: "Tidak ada item pada pesanan ini.", en: "No items in this order." },
  { key: "order_title", id: "Pesanan #{id}", en: "Order #{id}" },
  { key: "order_all_orders", id: "Semua pesanan", en: "All orders" },
  { key: "order_created_at", id: "Dibuat {date}", en: "Created {date}" },
  { key: "order_payment_pending_title", id: "Pembayaran tertunda", en: "Payment pending" },
  { key: "order_payment_pending_desc", id: "Selesaikan pembayaran{deadline}.", en: "Complete payment{deadline}." },
  { key: "order_pay_before", id: " sebelum ", en: " before " },
  { key: "order_continue_payment", id: "Lanjutkan Pembayaran", en: "Continue Payment" },
  { key: "order_payment_pending_countdown", id: "Sisa waktu sebelum pesanan kedaluwarsa", en: "Time left before the order expires" },
  { key: "order_payment_pending_expired", id: "Waktu pembayaran habis. Menunggu pesanan ditutup.", en: "Payment time is up. Waiting for the order to close." },
  { key: "order_pay_success_toast", id: "Pembayaran berhasil", en: "Payment successful" },
  { key: "order_pay_pending_toast", id: "Pembayaran masih tertunda", en: "Payment still pending" },
  { key: "order_pay_failed_toast", id: "Pembayaran gagal", en: "Payment failed" },
  { key: "order_pay_try_again", id: "Silakan coba lagi.", en: "Please try again." },
  { key: "order_pay_closed_toast", id: "Pembayaran ditutup", en: "Payment closed" },
  { key: "order_pay_continue_later", id: "Anda dapat melanjutkan kapan saja.", en: "You can continue anytime." },
  { key: "order_snap_unavailable", id: "Snap tidak tersedia", en: "Snap not available" },
  { key: "order_retry_failed_desc", id: "Gagal memulai ulang pembayaran.", en: "Failed to restart payment." },
  { key: "order_retry_failed_title", id: "Gagal melanjutkan pembayaran", en: "Failed to continue payment" },
  { key: "order_items_section", id: "Item pesanan", en: "Order items" },
  { key: "order_status_history", id: "Riwayat status", en: "Status history" },
  { key: "order_summary", id: "Ringkasan", en: "Summary" },
  { key: "order_subtotal", id: "Subtotal", en: "Subtotal" },
  { key: "order_discount", id: "Diskon", en: "Discount" },
  { key: "order_shipping", id: "Ongkos kirim", en: "Shipping cost" },
  { key: "order_total", id: "Total", en: "Total" },
  { key: "order_payment_info", id: "Info pembayaran", en: "Payment info" },
  { key: "order_payment_method", id: "Metode pembayaran", en: "Payment method" },
  { key: "order_gateway_ref", id: "Referensi gateway", en: "Gateway reference" },
  { key: "order_valid_until", id: "Berlaku sampai", en: "Valid until" },
  { key: "order_invoice", id: "Invoice", en: "Invoice" },
  { key: "order_tracking", id: "No. resi", en: "Tracking no." },
  { key: "order_view_invoice", id: "Lihat invoice", en: "View invoice" },

  // Competition / Registration (competition/page.tsx + competition/[id]/page.tsx)
  { key: "competition_list_subtitle", id: "Paket ujian yang Anda ikuti", en: "Exam packages you are enrolled in" },
  { key: "competition_status_upcoming", id: "Akan Datang", en: "Upcoming" },
  { key: "competition_status_ongoing", id: "Berlangsung", en: "Ongoing" },
  { key: "competition_status_completed", id: "Selesai", en: "Completed" },
  { key: "competition_view_detail", id: "Lihat Detail", en: "View Detail" },
  { key: "competition_empty", id: "Anda belum terdaftar pada ujian apapun", en: "You are not enrolled in any exams" },
  { key: "competition_error", id: "Gagal memuat data ujian", en: "Failed to load exam data" },
  { key: "competition_detail_back", id: "Kembali", en: "Back" },
  { key: "competition_detail_exam_info", id: "Informasi Ujian", en: "Exam Information" },
  { key: "competition_detail_scheduled_at", id: "Jadwal", en: "Scheduled" },
  { key: "competition_detail_token_label", id: "Token", en: "Token" },
  { key: "competition_detail_show_token", id: "Tampilkan Token", en: "Show Token" },
  { key: "competition_detail_hide_token", id: "Sembunyikan Token", en: "Hide Token" },
  { key: "competition_detail_download_card", id: "Unduh Kartu Peserta", en: "Download Participant Card" },
  { key: "competition_detail_checkin_heading", id: "Petunjuk Check-in", en: "Check-in Instructions" },
  { key: "competition_detail_checkin_body", id: "Harap check-in dalam waktu {N} menit sebelum ujian dimulai.", en: "Please check in within {N} minutes before the exam starts." },
  { key: "competition_detail_not_found", id: "Pendaftaran tidak ditemukan", en: "Registration not found" },
];

describe("i18n keys for student page migration", () => {
  it.each(NEW_KEYS)("DICT contains key '$key' in both id and en", ({ key, id, en }) => {
    expect(idDict[key]).toBe(id);
    expect(enDict[key]).toBe(en);
  });
});
