"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Save, Settings, Bell, Plug, Mail } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAdminSystemConfig,
  useUpdateSystemConfig,
} from "@/lib/hooks/admin-config";
import {
  useProvinces,
  useCitiesByProvince,
  useDistrictsByCity,
} from "@/lib/hooks/regions";

interface ToggleProps {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  description?: string;
}

function Toggle({ checked, onChange, label, description }: ToggleProps) {
  return (
    <div className="flex items-start justify-between gap-4 py-3">
      <div className="space-y-0.5">
        <div className="text-sm font-medium text-ink-900">{label}</div>
        {description && (
          <div className="text-xs text-ink-500">{description}</div>
        )}
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cn(
          "relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
          checked ? "bg-brand-600" : "bg-line"
        )}
      >
        <span
          className={cn(
            "pointer-events-none block size-5 rounded-full bg-white shadow ring-0 transition-transform",
            checked ? "translate-x-5" : "translate-x-0"
          )}
        />
      </button>
    </div>
  );
}

// A quiet small-caps label that turns a flat field list into readable groups.
function Eyebrow({ children }: { children: ReactNode }) {
  return (
    <div className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-ink-400">
      {children}
    </div>
  );
}

const INITIAL_APP = {
  app_name: "",
  app_address: "",
  app_logo_url: "",
  app_contact_email: "",
  app_contact_phone: "",
  app_help_url: "",
  app_social_handle: "",
  app_province_id: "",
  app_city_id: "",
  app_district_id: "",
  app_kode_pos: "",
};

const INITIAL_NOTIF = {
  notify_on_purchase_admin_store: "false",
};

const INITIAL_PAYMENT = {
  midtrans_server_key: "",
  midtrans_client_key: "",
  midtrans_env: "sandbox",
  shipping_fallback_flat_rate: "",
  biteship_api_key: "",
  biteship_webhook_secret: "",
};

export default function SystemConfigPage() {
  const { t } = useTranslation();
  const [tab, setTab] = useState("general");
  const configLoaded = useRef(false);

  const [appFields, setAppFields] = useState(INITIAL_APP);
  const [notifFields, setNotifFields] = useState(INITIAL_NOTIF);
  const [paymentFields, setPaymentFields] = useState(INITIAL_PAYMENT);

  const { data: config, isLoading, error } = useAdminSystemConfig();
  const updateConfig = useUpdateSystemConfig();

  const { data: provinces } = useProvinces();
  const { data: cities } = useCitiesByProvince(appFields.app_province_id);
  const { data: districts } = useDistrictsByCity(appFields.app_city_id);

  useEffect(() => {
    if (config && !configLoaded.current) {
      configLoaded.current = true;
      setAppFields({
        app_name: config.app_name ?? "",
        app_address: config.app_address ?? "",
        app_logo_url: config.app_logo_url ?? "",
        app_contact_email: config.app_contact_email ?? "",
        app_contact_phone: config.app_contact_phone ?? "",
        app_help_url: config.app_help_url ?? "",
        app_social_handle: config.app_social_handle ?? "",
        app_province_id: config.app_province_id ?? "",
        app_city_id: config.app_city_id ?? "",
        app_district_id: config.app_district_id ?? "",
        app_kode_pos: config.app_kode_pos ?? "",
      });
      setNotifFields({
        notify_on_purchase_admin_store:
          config.notify_on_purchase_admin_store ?? "false",
      });
      setPaymentFields({
        midtrans_server_key: config.midtrans_server_key ?? "",
        midtrans_client_key: config.midtrans_client_key ?? "",
        midtrans_env: (config.midtrans_env as "sandbox" | "production") ?? "sandbox",
        shipping_fallback_flat_rate: config.shipping_fallback_flat_rate ?? "",
        biteship_api_key: config.biteship_api_key ?? "",
        biteship_webhook_secret: config.biteship_webhook_secret ?? "",
      });
    }
  }, [config]);

  const handleSaveGeneral = async () => {
    try {
      await updateConfig.mutateAsync(appFields);
      toast.success(t("config_toast_general_saved"));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("sys_save_failed");
      toast.error(msg);
    }
  };

  const handleSaveNotif = async () => {
    try {
      await updateConfig.mutateAsync(notifFields);
      toast.success(t("config_toast_notif_saved"));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("sys_save_failed");
      toast.error(msg);
    }
  };

  const handleSavePayment = async () => {
    try {
      await updateConfig.mutateAsync(paymentFields);
      toast.success(t("config_toast_integrations_saved"));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("sys_save_failed");
      toast.error(msg);
    }
  };

  if (isLoading) {
    return (
      <div className="mx-auto max-w-4xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={Settings}
          title={t("config_title")}
          description={t("sys_loading")}
        />
        <div className="py-12 text-center text-ink-500">{t("sys_loading_data")}</div>
      </div>
    );
  }

  if (error) {
    const msg =
      (error as { code?: string })?.code === "forbidden"
        ? t("sys_error_forbidden")
        : t("sys_error_load");
    return (
      <div className="mx-auto max-w-4xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={Settings}
          title={t("config_title")}
          description={t("sys_error_title")}
        />
        <div className="py-12 text-center text-ink-500">{msg}</div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl px-4 py-8 md:px-6 md:py-10 fade-in">
      <AdminPageHeader
        icon={Settings}
        title={t("config_title")}
        description={t("config_subtitle")}
      />

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className="mb-6">
          <TabsTrigger value="general" className="text-xs">
            <Settings className="mr-1 size-4" />
            {t("config_tab_general")}
          </TabsTrigger>
          <TabsTrigger value="notifications" className="text-xs">
            <Bell className="mr-1 size-4" />
            {t("config_tab_notifications")}
          </TabsTrigger>
          <TabsTrigger value="integrations" className="text-xs">
            <Plug className="mr-1 size-4" />
            {t("config_tab_integrations")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="general">
          <div className="md-card-outlined">
            <div className="space-y-8">
              <div>
                <Eyebrow>{t("config_group_identity")}</Eyebrow>
                <div className="space-y-4">
                  <div>
                    <Label>{t("config_general_app_name")}</Label>
                    <Input
                      value={appFields.app_name}
                      onChange={(e) =>
                        setAppFields((f) => ({ ...f, app_name: e.target.value }))
                      }
                    />
                  </div>
                  <div>
                    <Label>{t("config_general_logo_url")}</Label>
                    <Input
                      value={appFields.app_logo_url}
                      onChange={(e) =>
                        setAppFields((f) => ({
                          ...f,
                          app_logo_url: e.target.value,
                        }))
                      }
                    />
                  </div>
                </div>
              </div>

              <div>
                <Eyebrow>{t("config_group_contact")}</Eyebrow>
                <div className="grid gap-4 sm:grid-cols-2">
                  <div>
                    <Label>{t("config_general_contact_email")}</Label>
                    <div className="flex items-center gap-2">
                      <Mail className="size-4 shrink-0 text-ink-400" />
                      <Input
                        type="email"
                        value={appFields.app_contact_email}
                        onChange={(e) =>
                          setAppFields((f) => ({
                            ...f,
                            app_contact_email: e.target.value,
                          }))
                        }
                      />
                    </div>
                  </div>
                  <div>
                    <Label>{t("config_general_contact_phone")}</Label>
                    <Input
                      value={appFields.app_contact_phone}
                      onChange={(e) =>
                        setAppFields((f) => ({
                          ...f,
                          app_contact_phone: e.target.value,
                        }))
                      }
                    />
                  </div>
                  <div>
                    <Label>{t("config_general_help_url")}</Label>
                    <Input
                      value={appFields.app_help_url}
                      onChange={(e) =>
                        setAppFields((f) => ({
                          ...f,
                          app_help_url: e.target.value,
                        }))
                      }
                    />
                  </div>
                  <div>
                    <Label>{t("config_general_social_handle")}</Label>
                    <Input
                      value={appFields.app_social_handle}
                      onChange={(e) =>
                        setAppFields((f) => ({
                          ...f,
                          app_social_handle: e.target.value,
                        }))
                      }
                    />
                  </div>
                </div>
              </div>

              <div>
                <Eyebrow>{t("config_group_address")}</Eyebrow>
                <div className="space-y-4">
                  <div>
                    <Label>{t("config_general_address")}</Label>
                    <Input
                      value={appFields.app_address}
                      onChange={(e) =>
                        setAppFields((f) => ({
                          ...f,
                          app_address: e.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div>
                      <Label>{t("students_field_provinsi")}</Label>
                      <Select
                        value={appFields.app_province_id}
                        onValueChange={(v) =>
                          setAppFields((f) => ({
                            ...f,
                            app_province_id: v,
                            app_city_id: "",
                            app_district_id: "",
                          }))
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {(provinces ?? []).map((p) => (
                            <SelectItem key={p.id} value={p.id}>
                              {p.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div>
                      <Label>{t("students_field_kota")}</Label>
                      <Select
                        value={appFields.app_city_id}
                        onValueChange={(v) =>
                          setAppFields((f) => ({
                            ...f,
                            app_city_id: v,
                            app_district_id: "",
                          }))
                        }
                        disabled={!appFields.app_province_id}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {(cities ?? []).map((c) => (
                            <SelectItem key={c.id} value={c.id}>
                              {c.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div>
                      <Label>{t("students_field_kecamatan")}</Label>
                      <Select
                        value={appFields.app_district_id}
                        onValueChange={(v) =>
                          setAppFields((f) => ({
                            ...f,
                            app_district_id: v,
                          }))
                        }
                        disabled={!appFields.app_city_id}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {(districts ?? []).map((d) => (
                            <SelectItem key={d.id} value={d.id}>
                              {d.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div>
                      <Label>{t("students_field_kode_pos")}</Label>
                      <Input
                        value={appFields.app_kode_pos}
                        onChange={(e) =>
                          setAppFields((f) => ({
                            ...f,
                            app_kode_pos: e.target.value,
                          }))
                        }
                        placeholder={t("config_general_kode_pos_placeholder")}
                      />
                    </div>
                  </div>
                  <p className="text-xs text-ink-500">
                    {t("config_address_shipping_hint")}
                  </p>
                </div>
              </div>

              <div className="flex justify-end">
                <Button
                  onClick={handleSaveGeneral}
                  disabled={updateConfig.isPending}
                >
                  <Save className="mr-1 size-4" />
                  {t("save")}
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="notifications">
          <div className="md-card-outlined">
            <Toggle
              checked={notifFields.notify_on_purchase_admin_store === "true"}
              onChange={(v) =>
                setNotifFields((f) => ({
                  ...f,
                  notify_on_purchase_admin_store: v ? "true" : "false",
                }))
              }
              label={t("config_notif_store_label")}
              description={t("config_notif_store_desc")}
            />
            <div className="flex justify-end pt-4">
              <Button
                onClick={handleSaveNotif}
                disabled={updateConfig.isPending}
              >
                <Save className="mr-1 size-4" />
                {t("save")}
              </Button>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="integrations">
          <div className="md-card-outlined">
            <div className="space-y-8">
              <div>
                <Eyebrow>{t("config_group_payment")}</Eyebrow>
                <div className="space-y-4">
                  <div>
                    <Label>{t("config_payment_server_key")}</Label>
                    <Input
                      type="password"
                      value={paymentFields.midtrans_server_key}
                      onChange={(e) =>
                        setPaymentFields((f) => ({
                          ...f,
                          midtrans_server_key: e.target.value,
                        }))
                      }
                      placeholder={
                        paymentFields.midtrans_server_key === "***"
                          ? "***"
                          : t("config_payment_placeholder_server")
                      }
                    />
                    {paymentFields.midtrans_server_key === "***" && (
                      <div className="mt-1 text-xs text-ink-500">
                        {t("config_payment_mask_hint")}
                      </div>
                    )}
                  </div>
                  <div>
                    <Label>{t("config_payment_client_key")}</Label>
                    <Input
                      type="password"
                      value={paymentFields.midtrans_client_key}
                      onChange={(e) =>
                        setPaymentFields((f) => ({
                          ...f,
                          midtrans_client_key: e.target.value,
                        }))
                      }
                      placeholder={
                        paymentFields.midtrans_client_key === "***"
                          ? "***"
                          : t("config_payment_placeholder_client")
                      }
                    />
                    {paymentFields.midtrans_client_key === "***" && (
                      <div className="mt-1 text-xs text-ink-500">
                        {t("config_payment_mask_hint")}
                      </div>
                    )}
                  </div>
                  <div>
                    <Label>{t("config_payment_env")}</Label>
                    <Select
                      value={paymentFields.midtrans_env}
                      onValueChange={(v) =>
                        setPaymentFields((f) => ({
                          ...f,
                          midtrans_env: v,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="sandbox">Sandbox</SelectItem>
                        <SelectItem value="production">Production</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>

              <div>
                <Eyebrow>{t("config_group_shipping")}</Eyebrow>
                <div className="space-y-4">
                  <div>
                    <Label>{t("config_shipping_fallback_rate")}</Label>
                    <Input
                      type="number"
                      value={paymentFields.shipping_fallback_flat_rate}
                      onChange={(e) =>
                        setPaymentFields((f) => ({
                          ...f,
                          shipping_fallback_flat_rate: e.target.value,
                        }))
                      }
                      placeholder={t("config_shipping_rate_placeholder")}
                    />
                  </div>
                  <div>
                    <Label>{t("config_shipping_biteship_key")}</Label>
                    <Input
                      type="password"
                      value={paymentFields.biteship_api_key}
                      onChange={(e) =>
                        setPaymentFields((f) => ({
                          ...f,
                          biteship_api_key: e.target.value,
                        }))
                      }
                      placeholder={
                        paymentFields.biteship_api_key === "***"
                          ? "***"
                          : t("config_shipping_key_placeholder")
                      }
                    />
                    {paymentFields.biteship_api_key === "***" && (
                      <div className="mt-1 text-xs text-ink-500">
                        {t("config_payment_mask_hint")}
                      </div>
                    )}
                  </div>
                  <div>
                    <Label>{t("config_shipping_webhook_secret")}</Label>
                    <Input
                      type="password"
                      value={paymentFields.biteship_webhook_secret}
                      onChange={(e) =>
                        setPaymentFields((f) => ({
                          ...f,
                          biteship_webhook_secret: e.target.value,
                        }))
                      }
                      placeholder={
                        paymentFields.biteship_webhook_secret === "***"
                          ? "***"
                          : t("config_shipping_webhook_secret_placeholder")
                      }
                    />
                    {paymentFields.biteship_webhook_secret === "***" && (
                      <div className="mt-1 text-xs text-ink-500">
                        {t("config_payment_mask_hint")}
                      </div>
                    )}
                  </div>
                </div>
              </div>

              <div className="flex justify-end">
                <Button
                  onClick={handleSavePayment}
                  disabled={updateConfig.isPending}
                >
                  <Save className="mr-1 size-4" />
                  {t("save")}
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
