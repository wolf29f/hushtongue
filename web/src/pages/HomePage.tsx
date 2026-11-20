import { useQuery } from "@tanstack/react-query";
import { getSettings } from "../api/settings.ts";

export function HomePage() {
  const settings = useQuery({ queryKey: ["settings"], queryFn: getSettings });

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 p-6">
      <h1 className="text-3xl font-semibold">hushtongue</h1>
      {settings.isPending && <p className="text-gray-500">Chargement…</p>}
      {settings.isError && <p className="text-red-700">{settings.error.message}</p>}
      {settings.isSuccess && (
        <p className="text-lg">
          {settings.data.languages.source} ↔ {settings.data.languages.con}
        </p>
      )}
    </main>
  );
}
