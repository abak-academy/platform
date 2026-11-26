import type { LucideIcon } from "lucide-react";
import { PageHead } from "@/components/shell/PageHead";

interface AdminPageHeaderProps {
  /** Kept so existing callers compile; the redesigned head has no icon tile. */
  icon?: LucideIcon;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  /** Kept so existing callers compile; actions are always right-aligned. */
  actionsAlign?: "start" | "end";
  back?: { href: string; label: string };
}

export function AdminPageHeader({ title, description, actions, back }: AdminPageHeaderProps) {
  return <PageHead title={title} sub={description} actions={actions} back={back} />;
}
