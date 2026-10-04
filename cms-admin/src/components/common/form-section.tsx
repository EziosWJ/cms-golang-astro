import type { PropsWithChildren } from "react";
import { ContentCard } from "@/components/common/content-card";

type FormSectionProps = PropsWithChildren<{
  title: string;
  description?: string;
  columns?: 1 | 2;
}>;

export function FormSection({ title, description, children, columns = 2 }: FormSectionProps) {
  return (
    <ContentCard title={title} description={description}>
      <div className={`grid gap-space-4 ${columns === 2 ? "md:grid-cols-2" : ""}`}>{children}</div>
    </ContentCard>
  );
}
