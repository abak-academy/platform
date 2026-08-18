import type { LucideIcon } from "lucide-react";

interface AdminPageHeaderProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  actionsAlign?: "start" | "end";
}

export function AdminPageHeader({
  icon: Icon,
  title,
  description,
  actions,
  actionsAlign = "start",
}: AdminPageHeaderProps) {
  return (
    <div className="mb-8">
      <div className={actionsAlign === "end" ? "flex flex-wrap items-end justify-between gap-4" : "flex flex-wrap items-start justify-between gap-4"}>
        <div>
          <div className="flex items-center gap-5">
            <div
              className="flex size-16 items-center justify-center rounded-[16px]"
              style={{
                backgroundColor: "var(--md-sys-color-primary-container)",
                color: "var(--md-sys-color-primary)",
              }}
            >
              <Icon size={32} />
            </div>
            <h1 className="text-headline">{title}</h1>
          </div>
          {description ? (
            <p className="text-title-medium color-on-surface-variant" style={{ marginLeft: "84px" }}>
              {description}
            </p>
          ) : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
    </div>
  );
}
