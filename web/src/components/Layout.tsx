import { Form, NavLink, Outlet, useLoaderData, useRouteLoaderData } from "react-router";
import type { Me } from "../api/auth.ts";
import { menuFor } from "../menu.ts";
import { NotFoundPage } from "../pages/NotFoundPage.tsx";
import type { layoutLoader } from "../routes.tsx";

/** Layout is the frame of the pages that need a session: the menu, then the page. */
export function Layout() {
  const me = useLoaderData<typeof layoutLoader>();

  return (
    <div className="min-h-screen">
      <header className="border-b border-gray-200">
        <div className="mx-auto flex max-w-3xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
          <span className="font-semibold">hushtongue</span>
          <nav className="flex flex-wrap gap-x-4 gap-y-1">
            {menuFor(me.role).map((entry) => (
              <NavLink
                key={entry.path}
                to={entry.path}
                end
                className={({ isActive }) =>
                  isActive ? "font-medium text-blue-700" : "text-gray-700 hover:text-blue-700"
                }
              >
                {entry.label}
              </NavLink>
            ))}
          </nav>
          <Form method="post" action="/logout" className="ml-auto flex items-center gap-3 text-sm">
            <span className="text-gray-600">{me.name}</span>
            <button
              type="submit"
              className="rounded border border-gray-300 px-2 py-1 hover:bg-gray-100"
            >
              Déconnexion
            </button>
          </Form>
        </div>
      </header>
      <main className="mx-auto max-w-3xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}

/** RequireGM shows the pages under it to the GM only, as missing pages to a player. */
export function RequireGM() {
  const me = useRouteLoaderData("layout") as Me;
  return me.role === "gm" ? <Outlet /> : <NotFoundPage />;
}
