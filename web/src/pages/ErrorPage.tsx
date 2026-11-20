import { Navigate, useRouteError } from "react-router";
import { isUnauthenticated } from "../api/auth.ts";

/**
 * ErrorPage shows an error thrown by a loader, an action or a page. A session
 * that ended while the app was open, expired or closed by the GM, sends back
 * to the login page.
 */
export function ErrorPage() {
  const error = useRouteError();
  if (isUnauthenticated(error)) {
    return <Navigate to="/login" replace />;
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 p-6">
      <h1 className="text-2xl font-semibold">Erreur</h1>
      <p className="text-red-700">
        {error instanceof Error ? error.message : "Erreur inattendue."}
      </p>
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

/** LoadingPage is shown while the loaders of the first page run. */
export function LoadingPage() {
  return <p className="p-6 text-gray-500">Chargement…</p>;
}
