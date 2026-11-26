"use client";

import { useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { authFetch } from "@/lib/api";
import { groupingErrorParts } from "@/lib/hooks/admin-product-groupings";
import { putFileToPresignedURL } from "@/lib/hooks/admin-students-bulk";
import {
  useApplyExternalRound,
  useDeleteExternalRound,
  useExternalRoundResults,
  useExternalRounds,
  usePresignExternalRound,
  usePreviewExternalRound,
  useSetExternalRoundPublished,
} from "@/lib/hooks/admin-external-rounds";
import type { ExamDetail, ExternalRoundPreview } from "@/lib/types";

const REASON_COPY: Record<string, string> = {
  unreadable_participant_no: "Nomor peserta tidak terbaca",
  other_exam_participant_no: "Nomor peserta milik ujian lain",
  unknown_participant: "Nomor peserta tidak dikenal",
  revoked_participant: "Peserta sudah dicabut aksesnya",
  duplicate_participant: "Nomor peserta muncul lebih dari sekali",
  bad_score: "Skor tidak valid",
  bad_rank: "Peringkat tidak valid",
  bad_certificate_url: "URL sertifikat tidak valid",
  award_too_long: "Penghargaan terlalu panjang",
  bad_row: "Baris tidak valid",
};

const dateTime = (iso: string) => new Date(iso).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });

function RoundResults({ roundId }: { roundId: string }) {
  const { data, isLoading } = useExternalRoundResults(roundId);
  if (isLoading) return <p className="text-sm text-muted-foreground">Memuat…</p>;
  return (
    <table className="w-full text-left text-xs" aria-label="Hasil ronde">
      <thead>
        <tr><th>No. peserta</th><th>Nama</th><th>Skor</th><th>Peringkat</th><th>Penghargaan</th></tr>
      </thead>
      <tbody>
        {(data?.data ?? []).map((r) => (
          <tr key={r.registration_id}>
            <td>{r.participant_no}</td><td>{r.student_name}</td><td>{r.score}</td><td>{r.rank ?? "-"}</td><td>{r.award ?? "-"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function ExternalRoundsDialog({
  open,
  onOpenChange,
  productName,
  examIds,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  productName: string;
  examIds: string[];
}) {
  const [examId, setExamId] = useState(examIds[0] ?? "");
  const [label, setLabel] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [key, setKey] = useState("");
  const [preview, setPreview] = useState<ExternalRoundPreview | null>(null);
  const [fileError, setFileError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [viewing, setViewing] = useState<string | null>(null);

  const titles = useQueries({
    queries: examIds.map((id) => ({
      queryKey: ["admin", "exams", "detail", id],
      queryFn: () => authFetch<ExamDetail>(`/admin/exams/${encodeURIComponent(id)}`),
      enabled: examIds.length > 1,
    })),
  });

  const rounds = useExternalRounds(examId).data?.data ?? [];
  const presign = usePresignExternalRound(examId);
  const previewMut = usePreviewExternalRound(examId);
  const apply = useApplyExternalRound(examId);
  const setPublished = useSetExternalRoundPublished();
  const del = useDeleteExternalRound();

  const busy = presign.isPending || previewMut.isPending || apply.isPending;
  const trimmed = label.trim();

  function resetPreview() {
    setPreview(null);
    setFileError("");
  }

  async function handlePreview() {
    if (!file || !trimmed) return;
    resetPreview();
    try {
      const p = await presign.mutateAsync(file.name);
      await putFileToPresignedURL(p.url, file, file.type || "text/csv");
      setKey(p.key);
      setPreview(await previewMut.mutateAsync({ key: p.key, round_label: trimmed }));
    } catch (e) {
      setFileError(groupingErrorParts(e).message);
    }
  }

  async function handleApply() {
    try {
      await apply.mutateAsync({ key, round_label: trimmed });
      toast.success(`Ronde "${trimmed}" disimpan.`);
      setFile(null);
      setLabel("");
      resetPreview();
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  async function run(fn: () => Promise<unknown>, ok: string) {
    try {
      await fn();
      toast.success(ok);
    } catch (e) {
      toast.error(groupingErrorParts(e).message);
    }
  }

  const canConfirm = preview !== null && preview.errors.length === 0 && preview.matched > 0 && !busy;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="font-serif">Hasil ronde luar — {productName}</DialogTitle>
          <DialogDescription>Unggah CSV hasil ronde yang dinilai di luar platform, periksa pratinjau, lalu terapkan.</DialogDescription>
        </DialogHeader>

        {examIds.length === 0 && <p className="text-sm text-muted-foreground">Produk ini belum punya ujian.</p>}

        {examIds.length > 1 && (
          <div className="space-y-1">
            <Label htmlFor="ext-exam">Ujian</Label>
            <select
              id="ext-exam"
              className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              value={examId}
              onChange={(e) => {
                setExamId(e.target.value);
                setViewing(null);
                resetPreview();
              }}
            >
              {examIds.map((id, i) => (
                <option key={id} value={id}>{titles[i]?.data?.title ?? `Ujian ${i + 1} (${id.slice(0, 8)})`}</option>
              ))}
            </select>
          </div>
        )}

        {examId && (
          <>
            <section className="space-y-2" aria-label="Ronde tersimpan">
              <h3 className="text-sm font-semibold">Ronde tersimpan</h3>
              {rounds.length === 0 && <p className="text-sm text-muted-foreground">Belum ada ronde.</p>}
              <ul className="divide-y rounded-md border">
                {rounds.map((r) => (
                  <li key={r.id} className="space-y-2 px-3 py-2">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div>
                        <span className="font-medium">{r.label}</span>{" "}
                        <span className="text-xs text-muted-foreground">{r.row_count} baris · diperbarui {dateTime(r.updated_at)}</span>{" "}
                        <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${r.published_at ? "bg-teal-50 text-teal-700" : "bg-muted text-muted-foreground"}`}>
                          {r.published_at ? "Tayang" : "Draf"}
                        </span>
                      </div>
                      <div className="flex gap-1">
                        <Button type="button" size="sm" variant="ghost" onClick={() => setViewing(viewing === r.id ? null : r.id)}>
                          {viewing === r.id ? "Tutup" : "Lihat"}
                        </Button>
                        <Button type="button" size="sm" variant="outline" disabled={setPublished.isPending} onClick={() => run(() => setPublished.mutateAsync({ roundId: r.id, publish: !r.published_at }), r.published_at ? "Ronde ditarik." : "Ronde ditayangkan.")}>
                          {r.published_at ? "Tarik" : "Tayangkan"}
                        </Button>
                        {confirmDelete === r.id ? (
                          <>
                            <Button type="button" size="sm" variant="destructive" onClick={() => { setConfirmDelete(null); run(() => del.mutateAsync(r.id), "Ronde dihapus."); }}>
                              Ya, hapus
                            </Button>
                            <Button type="button" size="sm" variant="ghost" onClick={() => setConfirmDelete(null)}>Batal</Button>
                          </>
                        ) : (
                          <Button type="button" size="sm" variant="ghost" aria-label={`Hapus ${r.label}`} onClick={() => setConfirmDelete(r.id)}>Hapus</Button>
                        )}
                      </div>
                    </div>
                    {confirmDelete === r.id && <p role="alert" className="text-sm text-destructive">Hapus ronde &quot;{r.label}&quot; beserta {r.row_count} barisnya? Tindakan ini tidak bisa dibatalkan.</p>}
                    {viewing === r.id && <RoundResults roundId={r.id} />}
                  </li>
                ))}
              </ul>
            </section>

            <section className="space-y-3" aria-label="Unggah ronde">
              <h3 className="text-sm font-semibold">Unggah hasil</h3>
              <div className="space-y-1">
                <Label htmlFor="ext-round">Ronde</Label>
                <select
                  id="ext-round"
                  className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                  value={rounds.find((r) => r.label === label)?.id ?? ""}
                  onChange={(e) => {
                    setLabel(rounds.find((r) => r.id === e.target.value)?.label ?? "");
                    resetPreview();
                  }}
                >
                  <option value="">Ronde baru</option>
                  {rounds.map((r) => <option key={r.id} value={r.id}>Ganti: {r.label}</option>)}
                </select>
              </div>
              <div className="space-y-1">
                <Label htmlFor="ext-label">Label ronde</Label>
                <Input id="ext-label" value={label} onChange={(e) => { setLabel(e.target.value); resetPreview(); }} placeholder="Babak Semifinal" />
              </div>
              <div className="space-y-1">
                <Label htmlFor="ext-file">Berkas CSV</Label>
                <input id="ext-file" type="file" accept=".csv,text/csv" onChange={(e) => { setFile(e.target.files?.[0] ?? null); resetPreview(); }} />
              </div>
              <Button type="button" variant="outline" disabled={!file || !trimmed || busy} onClick={handlePreview}>Pratinjau</Button>

              {fileError && <p role="alert" className="text-sm text-destructive">{fileError}</p>}

              {preview && (
                <div className="space-y-2" aria-label="Pratinjau">
                  <p className="text-sm"><strong>{preview.matched}</strong> baris cocok, <strong>{preview.errors.length}</strong> bermasalah.</p>
                  {preview.round_published && (
                    <p role="alert" className="rounded-md bg-gold-50 px-3 py-2 text-sm text-gold-700">Ronde ini sudah tayang. Menerapkan akan langsung mengganti hasil yang dilihat peserta.</p>
                  )}
                  {preview.errors.length > 0 && (
                    <table className="w-full text-left text-xs" aria-label="Galat pratinjau">
                      <thead><tr><th>Baris</th><th>No. peserta</th><th>Masalah</th></tr></thead>
                      <tbody>
                        {preview.errors.map((e, i) => (
                          <tr key={`${e.line}-${i}`}><td>{e.line}</td><td>{e.participant_no}</td><td>{REASON_COPY[e.reason] ?? e.reason}</td></tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                  <Button type="button" disabled={!canConfirm} onClick={handleApply}>Terapkan</Button>
                </div>
              )}
            </section>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
