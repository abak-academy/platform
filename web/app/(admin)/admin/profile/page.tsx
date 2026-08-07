"use client";

import { useRef, useState } from "react";
import { useMe } from "@/lib/hooks/auth";
import { usePresignUpload } from "@/lib/hooks/students";
import { useUpdateOwnPhoto } from "@/lib/hooks/profile";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { roleLabelKey } from "@/lib/nav-config";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ChangePasswordForm } from "@/components/profile/ChangePasswordForm";
import { toast } from "sonner";

function initials(name?: string): string {
  if (!name) return "?";
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? "";
  const second = parts[1]?.[0] ?? "";
  return (first + second).toUpperCase() || name.trim()[0].toUpperCase();
}

function Field({
  label,
  value,
  isLoading,
}: {
  label: string;
  value?: string;
  isLoading?: boolean;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-ink-600">{label}</span>
      {isLoading ? (
        <Skeleton className="h-5 w-32" />
      ) : (
        <span className="text-sm text-ink-900">{value || "—"}</span>
      )}
    </div>
  );
}

export default function AdminProfilePage() {
  const { t } = useTranslation();
  const { data: me, isLoading } = useMe();
  const presign = usePresignUpload();
  const updatePhoto = useUpdateOwnPhoto();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);

  const roleKey = roleLabelKey(me?.role);
  const roleLabel = roleKey ? t(roleKey) : "—";
  const signInMethodLabel =
    me?.auth_provider === "google"
      ? t("admin_profile_signin_google")
      : t("admin_profile_signin_password");

  async function handlePhotoSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const presigned = await presign.mutateAsync({
        filename: file.name,
        content_type: file.type,
      });
      const uploadRes = await fetch(presigned.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": file.type },
      });
      if (!uploadRes.ok) {
        throw new Error(`Upload failed: ${uploadRes.status}`);
      }
      await updatePhoto.mutateAsync(presigned.key);
      toast.success(t("photo_uploaded"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("photo_upload_failed"));
    } finally {
      setUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  }

  return (
    <div className="fade-in flex max-w-2xl flex-col gap-5">
      <header>
        <h1 className="font-serif text-3xl font-bold text-ink-900 md:text-4xl">
          {t("profile_title")}
        </h1>
      </header>

      <Card className="rounded-2xl border-0 p-6 shadow-md">
        <h2 className="mb-4 text-[15px] font-semibold text-ink-900">
          {t("admin_profile_identity_card")}
        </h2>

        <div className="mb-6 flex items-center gap-4">
          {isLoading ? (
            <Skeleton className="size-20 rounded-full" />
          ) : (
            <Avatar
              size="lg"
              className="size-20 rounded-full bg-gradient-to-br from-brand-100 to-brand-200 text-brand-700 ring-4 ring-surface shadow-sm"
            >
              {me?.photo_url ? (
                <AvatarImage
                  src={fileUrl(me.photo_url)}
                  alt={me?.name ?? ""}
                  className="object-cover"
                />
              ) : null}
              <AvatarFallback className="rounded-full bg-transparent text-2xl font-semibold">
                {initials(me?.name)}
              </AvatarFallback>
            </Avatar>
          )}
          <div className="flex flex-col gap-2">
            {isLoading ? (
              <Skeleton className="h-5 w-40" />
            ) : (
              <div className="font-serif text-xl font-semibold text-ink-900">
                {me?.name}
              </div>
            )}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => fileInputRef.current?.click()}
              disabled={uploading}
              className="w-fit"
            >
              {uploading ? t("saving") : t("upload_photo")}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={handlePhotoSelect}
            />
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label={t("admin_profile_role_label")} value={roleLabel} isLoading={isLoading} />
          <Field label={t("email")} value={me?.email || me?.username} isLoading={isLoading} />
          <Field label={t("admin_profile_status")} value={me?.status} isLoading={isLoading} />
          <Field
            label={t("admin_profile_signin_method")}
            value={signInMethodLabel}
            isLoading={isLoading}
          />
        </div>

        <p className="mt-4 text-xs text-ink-600">{t("admin_profile_edit_hint")}</p>
      </Card>

      <Card className="rounded-2xl border-0 p-6 shadow-md">
        <h2 className="mb-4 text-[15px] font-semibold text-ink-900">
          {t("admin_profile_security_card")}
        </h2>
        <ChangePasswordForm />
      </Card>
    </div>
  );
}
