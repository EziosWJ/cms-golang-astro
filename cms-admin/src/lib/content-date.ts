// Working dates use the configured IANA timezone and persist UTC instants.
export function formatContentDate(iso: string | null, timezone: string) {
  if (!iso) return "";
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23" }).formatToParts(new Date(iso));
  const values = Object.fromEntries(parts.map(({ type, value }) => [type, value]));
  return `${values.year.padStart(4, "0")}-${values.month}-${values.day}T${values.hour}:${values.minute}:${values.second}`;
}
export function parseContentDate(local: string, timezone: string): string | null {
  if (!local) return null;
  const normalized = local.length === 16 ? `${local}:00` : local;
  const wall = new Date(`${normalized}Z`).getTime();
  if (!Number.isFinite(wall)) throw new Error("展示日期无效");
  let candidate = wall;
  for (let step = 0; step < 4; step++) {
    const rendered = formatContentDate(new Date(candidate).toISOString(), timezone);
    if (rendered === normalized) return new Date(candidate).toISOString();
    candidate += wall - new Date(`${rendered}Z`).getTime();
  }
  throw new Error(`该时间在 ${timezone} 不存在，请调整展示日期`);
}
