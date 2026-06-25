export function formatDate(date: number | Date | string): string {
  let d: Date;
  if (typeof date === "number") {
    d = new Date(date);
  } else if (typeof date === "string") {
    d = new Date(date);
  } else {
    d = date;
  }
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
  });
}

export function formatTime(date: number | Date | string): string {
  let d: Date;
  if (typeof date === "number") {
    d = new Date(date);
  } else if (typeof date === "string") {
    d = new Date(date);
  } else {
    d = date;
  }
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
