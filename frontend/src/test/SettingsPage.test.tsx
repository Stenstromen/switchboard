import { screen, waitFor } from "@testing-library/react";
import { expect, test } from "vitest";
import { api, dialogsApi } from "./backend";
import { renderApp } from "./render";

test("saves how the app is shown and whether it opens at login", async () => {
  const { user } = renderApp({ page: "settings" });

  const dock = await screen.findByRole("radio", { name: "Dock icon" });
  await waitFor(() => expect(dock).toBeEnabled());
  expect(screen.getByRole("radio", { name: "Both" })).toBeChecked();
  await user.click(dock);
  await waitFor(() => expect(screen.getByRole("radio", { name: "Dock icon" })).toBeChecked());
  expect(api.setPreferences).toHaveBeenCalledWith(expect.objectContaining({ showAs: "dock" }));

  const login = screen.getByRole("checkbox", { name: "Open Switchboard automatically when you log in" });
  await waitFor(() => expect(login).toBeEnabled());
  await user.click(login);
  expect(api.setPreferences).toHaveBeenCalledWith(
    expect.objectContaining({ showAs: "dock", openAtLogin: true })
  );
});

test("exports configuration to the chosen file", async () => {
  const { user } = renderApp({ page: "settings" });
  await screen.findByRole("radio", { name: "Both" });
  dialogsApi.SaveFile.mockResolvedValue("/tmp/switchboard-config.json");

  await user.click(screen.getByRole("button", { name: "Export…" }));

  expect(await screen.findByText("Exported 0 tunnels.")).toBeInTheDocument();
  expect(api.exportConfiguration).toHaveBeenCalledWith("/tmp/switchboard-config.json");
});

test("merges an imported configuration without a replace prompt", async () => {
  const { user } = renderApp({ page: "settings" });
  await screen.findByRole("button", { name: "Import & Merge…" });
  dialogsApi.OpenFile.mockResolvedValue("/tmp/in.json");

  await user.click(screen.getByRole("button", { name: "Import & Merge…" }));

  expect(await screen.findByText("Merged configuration.")).toBeInTheDocument();
  expect(dialogsApi.Question).not.toHaveBeenCalled();
  expect(api.importConfiguration).toHaveBeenCalledWith("/tmp/in.json", false);
});

test("asks before replacing configuration and stops when cancelled", async () => {
  const { user } = renderApp({ page: "settings" });
  await screen.findByRole("button", { name: "Import & Replace…" });
  dialogsApi.Question.mockResolvedValue("Cancel");

  await user.click(screen.getByRole("button", { name: "Import & Replace…" }));

  expect(dialogsApi.Question).toHaveBeenCalled();
  expect(dialogsApi.OpenFile).not.toHaveBeenCalled();
  expect(api.importConfiguration).not.toHaveBeenCalled();
});

test("replaces configuration after confirmation", async () => {
  const { user } = renderApp({ page: "settings" });
  await screen.findByRole("button", { name: "Import & Replace…" });
  dialogsApi.Question.mockResolvedValue("Replace");
  dialogsApi.OpenFile.mockResolvedValue("/tmp/replace.json");

  await user.click(screen.getByRole("button", { name: "Import & Replace…" }));

  expect(await screen.findByText("Replaced configuration.")).toBeInTheDocument();
  expect(api.importConfiguration).toHaveBeenCalledWith("/tmp/replace.json", true);
});

test("requests notification permission and sends a test", async () => {
  const { user } = renderApp({ page: "settings" });
  await user.click(await screen.findByRole("button", { name: "Notifications" }));

  expect(await screen.findByText("Notifications are turned off.")).toBeInTheDocument();
  expect(screen.getByText("Not allowed")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send Test Notification" })).toBeDisabled();

  await user.click(screen.getByRole("button", { name: "Enable Notifications…" }));
  expect(await screen.findByText("Permission granted.")).toBeInTheDocument();
  expect(screen.getByText("Allowed")).toBeInTheDocument();
  expect(api.requestNotificationPermission).toHaveBeenCalledOnce();

  await user.click(screen.getByRole("button", { name: "Send Test Notification" }));
  expect(await screen.findByText("Test notification sent.")).toBeInTheDocument();
  expect(api.sendTestNotification).toHaveBeenCalledOnce();
});

test("adds and removes a known host", async () => {
  const { user } = renderApp({ page: "settings" });
  await user.click(await screen.findByRole("button", { name: "SSH" }));
  expect(await screen.findByText("No known hosts yet")).toBeInTheDocument();
  expect(screen.getByText("/Users/me/.ssh/known_hosts")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Add" }));
  await user.type(
    screen.getByPlaceholderText("Enter known host entry"),
    "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFake"
  );
  const addButtons = screen.getAllByRole("button", { name: "Add" });
  await user.click(addButtons.find((button) => button.textContent === "Add")!);

  const row = await screen.findByRole("button", { name: /github.com ssh-ed25519/ });
  expect(api.addKnownHost).toHaveBeenCalledWith(
    "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFake"
  );

  await user.click(row);
  await user.click(screen.getByRole("button", { name: "Remove" }));
  expect(api.removeKnownHost).toHaveBeenCalledWith(0);
  expect(await screen.findByText("No known hosts yet")).toBeInTheDocument();
});
