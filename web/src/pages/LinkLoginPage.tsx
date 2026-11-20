import { useLoaderData } from "react-router";
import type { linkLoginLoader } from "../routes.tsx";

/** LinkLoginPage shows why the magic link didn't log the player in. */
export function LinkLoginPage() {
  const { error } = useLoaderData<typeof linkLoginLoader>();

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-4 p-6">
      <h1 className="text-3xl font-semibold">hushtongue</h1>
      <p className="text-red-700">{error}</p>
    </main>
  );
}
