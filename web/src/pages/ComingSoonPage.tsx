/** ComingSoonPage holds the place of a page of the menu not built yet. */
export function ComingSoonPage({ title }: { title: string }) {
  return (
    <>
      <h1 className="text-2xl font-semibold">{title}</h1>
      <p className="mt-2 text-gray-600">Cette page arrive bientôt.</p>
    </>
  );
}
