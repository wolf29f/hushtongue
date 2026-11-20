import { Link } from "react-router";

export function NotFoundPage() {
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 p-6">
      <h1 className="text-2xl font-semibold">Page introuvable</h1>
      <Link to="/" className="text-blue-700 underline">
        Retour à l'accueil
      </Link>
    </main>
  );
}
