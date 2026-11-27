"use client";

import { useRef, useState } from "react";
import { KeyRound } from "lucide-react";
import { useMe } from "@/lib/hooks/auth";
import { usePresignUpload } from "@/lib/hooks/students";
import { useUpdateOwnPhoto } from "@/lib/hooks/profile";
import { fileUrl } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import { roleLabelKey } from "@/lib/nav-config";
import { PageHead } from "@/components/shell/PageHead";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import { AvatarPicker } from "@/components/profile/AvatarPicker";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ChangePasswordForm } from "@/components/profile/ChangePasswordForm";
import { toast } from "sonner";

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
  const [changingPassword, setChangingPassword] = useState(false);

  const roleKey = roleLabelKey(me?.role);
  const roleLabel = roleKey ? t(roleKey) : "—";
  // Google sign-in is student-only, so an admin's usable method is always the
  // password — naming the Google origin keeps that honest for migrated accounts.
  const signInMethodLabel =
    me?.auth_provider === "google"
      ? t("admin_profile_signin_password_google_origin")
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
    <>
      <PageHead title={t("profile_title")} />

      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
        <Card className="items-center gap-0 bg-[radial-gradient(100%_70%_at_50%_0%,var(--color-brand-50),var(--color-surface)_70%)] p-6 text-center">
          {isLoading ? (
            <Skeleton className="size-[104px] rounded-full" />
          ) : (
            <AbakAvatar
              name={me?.name ?? ""}
              index={me?.avatar_index}
              size="xl"
              src={me?.photo_url ? fileUrl(me.photo_url) : undefined}
              className={uploading ? "animate-pulse" : undefined}
            />
          )}
          <div className="mt-4 w-full min-w-0">
            {isLoading ? (
              <Skeleton className="mx-auto h-5 w-40" />
            ) : (
              <>
                <h2 className="type-h2 text-ink-900 [overflow-wrap:anywhere]">{me?.name}</h2>
                <div className="truncate text-sm text-ink-600">{roleLabel}</div>
              </>
            )}
          </div>
          {!isLoading && (
            <div className="mt-4 flex w-full flex-col gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
              >
                {uploading ? t("saving") : t("upload_photo")}
              </Button>
              <AvatarPicker name={me?.name ?? ""} current={me?.avatar_index} />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setChangingPassword((open) => !open)}
                aria-expanded={changingPassword}
                aria-controls={changingPassword ? "change-password-panel" : undefined}
              >
                <KeyRound />
                {changingPassword ? t("cancel") : t("admin_profile_change_password")}
              </Button>
            </div>
          )}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={handlePhotoSelect}
          />
        </Card>

        <div className="flex flex-col gap-4">
          <Card className="gap-0 p-6">
            <h2 className="mb-4 type-h2 text-ink-900">{t("admin_profile_identity_card")}</h2>
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

          <Card className="gap-0 p-6">
            <h2 className="type-h2 text-ink-900">{t("admin_profile_security_card")}</h2>
            <p className="mt-1 text-xs text-ink-600">{t("admin_profile_password_hint")}</p>
            {changingPassword && (
              <div id="change-password-panel" className="mt-5 border-t border-line pt-5">
                <ChangePasswordForm onDone={() => setChangingPassword(false)} />
              </div>
            )}
          </Card>
        </div>
      </div>
    </>
  );
}
