import { type ReactNode, useEffect, useState } from "react";
import { getSettings, type Settings } from "../api/settings.ts";
import { LoadingPage } from "../pages/ErrorPage.tsx";
import { SettingsContext } from "../settings.ts";

/**
 * SettingsProvider reads the settings of the instance once, then shows its
 * children, which read them with useSettings.
 */
export function SettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getSettings().then(setSettings, (err: Error) => setError(err.message));
  }, []);

  if (error) {
    return (
      <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 p-6">
        <p className="text-red-700">{error}</p>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="self-start rounded border border-gray-300 px-3 py-2 hover:bg-gray-100"
        >
          Réessayer
        </button>
      </main>
    );
  }
  if (!settings) {
    return <LoadingPage />;
  }
  return <SettingsContext value={settings}>{children}</SettingsContext>;
}
