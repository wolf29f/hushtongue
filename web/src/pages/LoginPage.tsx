import { Form, useActionData, useNavigation } from "react-router";
import type { loginAction } from "../routes.tsx";
import { useSettings } from "../settings.ts";

export function LoginPage() {
  const settings = useSettings();
  const result = useActionData<typeof loginAction>();
  const navigation = useNavigation();

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 p-6">
      <div>
        <h1 className="text-3xl font-semibold">hushtongue</h1>
        <p className="text-gray-600">
          {settings.languages.source} ↔ {settings.languages.con}
        </p>
      </div>
      <Form method="post" className="flex flex-col gap-4">
        <label className="flex flex-col gap-1">
          Nom
          <input
            name="name"
            autoComplete="username"
            required
            className="rounded border border-gray-300 px-3 py-2"
          />
        </label>
        <label className="flex flex-col gap-1">
          Mot de passe
          <input
            name="password"
            type="password"
            autoComplete="current-password"
            required
            className="rounded border border-gray-300 px-3 py-2"
          />
        </label>
        {result?.error && <p className="text-red-700">{result.error}</p>}
        <button
          type="submit"
          disabled={navigation.state === "submitting"}
          className="rounded bg-blue-700 px-3 py-2 font-medium text-white hover:bg-blue-800 disabled:opacity-50"
        >
          Se connecter
        </button>
      </Form>
      <p className="text-sm text-gray-600">
        Joueurs : connectez-vous avec le lien envoyé par le MJ.
      </p>
    </main>
  );
}
