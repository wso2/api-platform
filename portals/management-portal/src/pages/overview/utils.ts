export const slugify = (s: string) =>
  s
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-");

export const twoLetters = (s: string) => {
  const letters = (s || "").replace(/[^A-Za-z]/g, "");
  if (!letters) return "GW";
  const first = letters[0]?.toUpperCase() ?? "";
  const second = letters[1]?.toLowerCase() ?? "";
  return `${first}${second}`;
};

export const relativeTime = (d: Date) => {
  const diff = Math.max(0, Date.now() - d.getTime());
  const sec = Math.floor(diff / 1000);
  const min = Math.floor(sec / 60);
  const hr = Math.floor(min / 60);
  const day = Math.floor(hr / 24);
  if (sec < 45) return "just now";
  if (min < 60) return `${min} min ago`;
  if (hr < 24) return `${hr} hr${hr > 1 ? "s" : ""} ago`;
  return `${day} day${day > 1 ? "s" : ""} ago`;
};

export const codeFor = (name: string, token: string) =>
  `curl -Ls https://bijira.dev/quick-start | bash -s -- -k $GATEWAY_KEY --name ${name}`;
