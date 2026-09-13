"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useProfile, useUpdateProfile } from "@/lib/hooks/students";
import { studentsKeys } from "@/lib/hooks/students";
import { isProfileComplete } from "@/lib/profile";
import { useAuthStore } from "@/stores/auth";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SchoolPicker } from "@/components/SchoolPicker";
import type { SchoolOption } from "@/lib/types";
import { toast } from "sonner";

const GRADES = ["7", "8", "9", "10", "11", "12"];
export default function CompleteProfilePage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const token = useAuthStore((s) => s.token);
  const user = useAuthStore((s) => s.user);
  const { data: profile, isLoading } = useProfile();
  const updateProfile = useUpdateProfile();

  const [schoolId, setSchoolId] = useState("");
  const [unlistedSchoolName, setUnlistedSchoolName] = useState("");
  const [selectedSchool, setSelectedSchool] = useState<SchoolOption | null>(null);
  const [grade, setGrade] = useState("");
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Prefill name from the stored user (Google-provided).
  useEffect(() => {
    if (!name && user?.name) {
      setName(user.name);
    }
  }, [user?.name]); // eslint-disable-line react-hooks/exhaustive-deps

  // Redirect guards — evaluate once data arrives.
  useEffect(() => {
    if (!token) {
      router.replace("/login");
      return;
    }
    if (isLoading) return;
    if (!profile) return; // still resolving
    if (profile.auth_provider !== "google" || isProfileComplete(profile)) {
      router.replace("/");
    }
  }, [token, isLoading, profile, router]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmedUnlisted = unlistedSchoolName.trim();
    if (!grade || (!schoolId && !trimmedUnlisted)) {
      toast.error("Silakan lengkapi sekolah dan kelas.");
      return;
    }
    setSubmitting(true);
    try {
      await updateProfile.mutateAsync({
        name: name || undefined,
        school_id: schoolId || "",
        unlisted_school_name: schoolId ? "" : trimmedUnlisted,
        grade: parseInt(grade, 10),
      });
      // Invalidate the profile query so the gate re-evaluates with fresh data.
      await queryClient.invalidateQueries({ queryKey: studentsKeys.profile() });
      toast.success("Profil berhasil dilengkapi!");
      router.replace("/");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Gagal menyimpan profil."
      );
    } finally {
      setSubmitting(false);
    }
  }

  // Guard loading states — no token check, incomplete data check.
  if (!token) return null;
  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loader2 className="size-8 animate-spin text-brand-500" />
      </div>
    );
  }
  // Don't render the form if the user shouldn't be here.
  if (!profile || profile.auth_provider !== "google" || isProfileComplete(profile)) {
    return null;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-2 px-4">
      <Card className="w-full max-w-md rounded-2xl border-0 p-6 shadow-lg">
        <h1 className="mb-2 font-serif text-2xl font-bold text-ink-900">
          Lengkapi Profil
        </h1>
        <p className="mb-6 text-sm text-ink-500">
          Akun Google kamu belum memiliki data sekolah dan kelas. Silakan lengkapi
          untuk melanjutkan.
        </p>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="onboard-name" className="text-xs font-semibold text-ink-600">
              Nama
            </Label>
            <Input
              id="onboard-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Nama lengkap"
              className="h-11 rounded-md"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="onboard-school" className="text-xs font-semibold text-ink-600">
              Sekolah
            </Label>
            <SchoolPicker
              id="onboard-school"
              value={schoolId}
              selectedSchool={selectedSchool}
              onChange={(school) => {
                setSelectedSchool(school);
                setSchoolId(school?.id ?? "");
              }}
              allowUnlisted
              unlistedName={unlistedSchoolName}
              onUnlistedNameChange={setUnlistedSchoolName}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="onboard-grade" className="text-xs font-semibold text-ink-600">
              Kelas
            </Label>
            <Select value={grade || "_empty_"} onValueChange={(v) => setGrade(v === "_empty_" ? "" : v)}>
              <SelectTrigger id="onboard-grade" className="h-11 rounded-md">
                <SelectValue placeholder="Pilih kelas" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_empty_">Pilih kelas</SelectItem>
                {GRADES.map((g) => (
                  <SelectItem key={g} value={g}>
                    {g}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <Button type="submit" className="mt-2" disabled={submitting}>
            {submitting ? (
              <Loader2 className="mr-2 size-4 animate-spin" />
            ) : null}
            {submitting ? "Menyimpan…" : "Lanjutkan"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
