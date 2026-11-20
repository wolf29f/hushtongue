import { Link } from "react-router";

export function NotFoundPage() {
  return (
    <>
      <h1 className="text-2xl font-semibold">Page introuvable</h1>
      <Link to="/" className="mt-2 inline-block text-blue-700 underline">
        Retour à l'accueil
      </Link>
    </>
  );
}
