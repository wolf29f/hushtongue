import type { Role } from "./api/auth.ts";

export type MenuEntry = {
  label: string;
  path: string;
  gmOnly?: boolean;
};

const entries: MenuEntry[] = [
  { label: "Traduction", path: "/" },
  { label: "Dictionnaire", path: "/dictionary" },
  { label: "Textes", path: "/texts" },
  { label: "Joueurs", path: "/players", gmOnly: true },
];

/** menuFor returns the entries of the menu shown to a user of role. */
export function menuFor(role: Role): MenuEntry[] {
  return entries.filter((entry) => !entry.gmOnly || role === "gm");
}
