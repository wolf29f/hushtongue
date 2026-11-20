import { createContext, useContext } from "react";
import type { Settings } from "./api/settings.ts";

/** SettingsContext holds the settings of the instance, set by SettingsProvider. */
export const SettingsContext = createContext<Settings | null>(null);

/** useSettings returns the settings of the instance. It throws outside SettingsProvider. */
export function useSettings(): Settings {
  const settings = useContext(SettingsContext);
  if (!settings) {
    throw new Error("useSettings called outside SettingsProvider");
  }
  return settings;
}
