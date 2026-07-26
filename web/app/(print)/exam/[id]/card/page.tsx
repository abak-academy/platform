"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { toast } from "sonner";

import { useRegistration } from "@/lib/hooks/exam";
import { useProfile, useSchools } from "@/lib/hooks/students";
import { API_BASE, ApiError, fileUrl } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { ExamCardPrintable } from "@/components/exam/ExamCardPrintable";
import styles from "@/components/exam/ExamCardPrintable.module.css";

const DASH = "—";

// downloadExamCardPDF hits the FR-30 endpoint (Gotenberg-rendered, cached
// under card_key server-side) and saves the response as a file — the
// endpoint requires a Bearer token, so this can't be a plain <a href>.
async function downloadExamCardPDF(id: string): Promise<void> {
  const token = useAuthStore.getState().token;
  const res = await fetch(
    `${API_BASE}/exam/registrations/${encodeURIComponent(id)}/card`,
    { headers: token ? { Authorization: `Bearer ${token}` } : {} }
  );
  if (!res.ok) {
    throw new ApiError(
      `HTTP_${res.status}`,
      `Failed to download card: ${res.status}`,
      res.status
    );
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function fmtDate(iso: string | null | undefined): string {
  if (!iso) return DASH;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return DASH;
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "Asia/Jakarta",
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(d);
}

function fmtTime(iso: string | null | undefined): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "Asia/Jakarta",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export default function ExamCardPrintPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const [downloading, setDownloading] = useState(false);

  const { data: reg, isLoading: regLoading, isError: regError } =
    useRegistration(id);
  const { data: student, isLoading: profileLoading } = useProfile();
  const { data: schools } = useSchools();

  const handleDownload = async () => {
    setDownloading(true);
    try {
      await downloadExamCardPDF(id);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Gagal mengunduh kartu.");
    } finally {
      setDownloading(false);
    }
  };

  if (regLoading || profileLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat kartu…
      </div>
    );
  }

  if (regError || !reg) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-3 text-ink-500">
        <p>Kartu peserta tidak ditemukan.</p>
        <Link href="/exam" className="text-sm font-semibold text-primary">
          Kembali ke daftar ujian
        </Link>
      </div>
    );
  }

  const exam = reg.exam;

  const schoolName =
    student?.unlisted_school_name?.trim() ||
    schools?.find((s) => s.id === student?.school_id)?.name ||
    DASH;

  const participantNumber = reg.participant_no || DASH;

  const start = fmtTime(exam.scheduled_at);
  const end = fmtTime(exam.scheduled_end_at);
  const timeRange = start ? `${start}${end ? ` – ${end}` : ""} WIB` : DASH;

  return (
    <div className={styles.screen}>
      <div className={styles.toolbar}>
        <Link className={styles.backLink} href={`/exam/${id}`}>
          ← Kembali
        </Link>
        <p>
          <strong>Kartu Peserta Ujian</strong> — ukuran cetak A6 (148×105 mm)
        </p>
        <button
          type="button"
          className={styles.printBtn}
          onClick={() => window.print()}
        >
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M6 9V2h12v7" />
            <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2" />
            <rect x="6" y="14" width="12" height="8" rx="1" />
          </svg>
          Cetak / Simpan PDF
        </button>
        <button
          type="button"
          className={styles.printBtn}
          onClick={handleDownload}
          disabled={downloading}
        >
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <path d="M7 10l5 5 5-5" />
            <path d="M12 15V3" />
          </svg>
          {downloading ? "Mengunduh…" : "Download PDF"}
        </button>
      </div>

      <ExamCardPrintable
        fullName={student?.name?.trim() || DASH}
        participantNumber={participantNumber}
        school={schoolName}
        grade={student?.grade != null ? String(student.grade) : DASH}
        dob={fmtDate(student?.dob)}
        photoUrl={fileUrl(student?.photo_url)}
        examName={exam.title || DASH}
        subject={reg.subject || DASH}
        date={fmtDate(exam.scheduled_at)}
        timeRange={timeRange}
        duration={
          exam.duration_minutes ? `${exam.duration_minutes} menit` : DASH
        }
        mode="Online (CBT)"
        platform={reg.platform || DASH}
        checkInCode={reg.token || DASH}
      />
    </div>
  );
}
