import {
  type ActionFunctionArgs,
  createBrowserRouter,
  type LoaderFunctionArgs,
  redirect,
  replace,
} from "react-router";
import { getMe, login, loginWithLink, logout } from "./api/auth.ts";
import { Layout, RequireGM } from "./components/Layout.tsx";
import { ComingSoonPage } from "./pages/ComingSoonPage.tsx";
import { ErrorPage, LoadingPage } from "./pages/ErrorPage.tsx";
import { LinkLoginPage } from "./pages/LinkLoginPage.tsx";
import { LoginPage } from "./pages/LoginPage.tsx";
import { NotFoundPage } from "./pages/NotFoundPage.tsx";

// The table of the pages. A route runs its loader before it shows, and its
// action when one of its forms is sent; after an action, the loaders of the
// page run again. A page reads what they return with useLoaderData and
// useActionData. What they throw is shown by ErrorPage.
export const router = createBrowserRouter([
  {
    ErrorBoundary: ErrorPage,
    HydrateFallback: LoadingPage,
    children: [
      { path: "/login", loader: loginLoader, action: loginAction, Component: LoginPage },
      { path: "/login/:token", loader: linkLoginLoader, Component: LinkLoginPage },
      { path: "/logout", action: logoutAction },
      {
        id: "layout",
        loader: layoutLoader,
        Component: Layout,
        children: [
          { index: true, element: <ComingSoonPage title="Traduction" /> },
          { path: "dictionary", element: <ComingSoonPage title="Dictionnaire" /> },
          { path: "texts", element: <ComingSoonPage title="Textes" /> },
          {
            Component: RequireGM,
            children: [{ path: "players", element: <ComingSoonPage title="Joueurs" /> }],
          },
          { path: "*", Component: NotFoundPage },
        ],
      },
    ],
  },
]);

/** loginLoader sends a logged in user home. */
export async function loginLoader() {
  if (await getMe()) {
    return redirect("/");
  }
  return null;
}

/** loginAction logs the GM in with the form, then goes home. */
export async function loginAction({ request }: ActionFunctionArgs) {
  const form = await request.formData();
  try {
    await login(String(form.get("name")), String(form.get("password")));
  } catch (err) {
    return { error: (err as Error).message };
  }
  return redirect("/");
}

/**
 * linkLoginLoader logs the player in with the token of their magic link, then
 * goes home. replace, unlike redirect, swaps the history entry, which takes the
 * token out of the history.
 */
export async function linkLoginLoader({ params }: LoaderFunctionArgs) {
  try {
    await loginWithLink(params.token ?? "");
  } catch (err) {
    return { error: (err as Error).message };
  }
  return replace("/");
}

/** logoutAction closes the session, then goes to the login page. */
export async function logoutAction() {
  await logout();
  return redirect("/login");
}

/** layoutLoader reads the logged in user, and sends to the login page without a session. */
export async function layoutLoader() {
  const me = await getMe();
  if (!me) {
    throw redirect("/login");
  }
  return me;
}
