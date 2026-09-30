import { render, type RenderResult } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import App from "../App";
import { resetBackend } from "./backend";

export function renderApp(options?: {
  page?: "settings";
  prepare?: () => void;
}): RenderResult & { user: UserEvent } {
  resetBackend();
  options?.prepare?.();
  const search = options?.page === "settings" ? "?page=settings" : "";
  window.history.replaceState({}, "", `/${search}`);
  const user = userEvent.setup();
  return { user, ...render(<App />) };
}
