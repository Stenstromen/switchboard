import { fireEvent, screen, waitFor } from "@testing-library/react";
import { expect, test } from "vitest";
import { api, emit, failList, seedTunnel, tunnelByName } from "./backend";
import { renderApp } from "./render";

function tunnelItems() {
  return screen.getAllByRole("button").filter((el) => el.classList.contains("tunnel-item"));
}

function tunnelItem(name: string) {
  const item = tunnelItems().find((el) => el.textContent?.includes(name));
  if (!item) throw new Error(`no tunnel row named ${name}`);
  return item;
}

const localForward = {
  id: "fwd-1",
  type: "local" as const,
  localHost: "127.0.0.1",
  localPort: 8080,
  remoteHost: "10.0.0.5",
  remotePort: 80,
};

test("shows an empty state when there are no tunnels", async () => {
  renderApp();
  expect(await screen.findByText("No tunnels yet")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Create Tunnel" })).toBeInTheDocument();
  expect(screen.getByText("0 tunnels")).toBeInTheDocument();
});

test("shows the backend error when the tunnel list fails", async () => {
  renderApp({ prepare: () => failList("backend unavailable") });
  expect(await screen.findByText("backend unavailable")).toBeInTheDocument();
  expect(screen.getByText("No tunnels yet")).toBeInTheDocument();
});

test("lists tunnels and selects another row", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Alpha", hostName: "alpha.example", user: "ada" });
      seedTunnel({ name: "Beta", hostName: "beta.example", user: "bea", tags: ["work"] });
    },
  });

  expect(await screen.findByRole("heading", { name: "Alpha" })).toBeInTheDocument();
  expect(screen.getByText("ada@alpha.example")).toBeInTheDocument();
  expect(screen.getByText("2 tunnels")).toBeInTheDocument();
  expect(tunnelItem("Beta")).toHaveTextContent("work");

  await user.click(tunnelItem("Beta"));
  expect(screen.getByRole("heading", { name: "Beta" })).toBeInTheDocument();
  expect(screen.getByText("bea@beta.example")).toBeInTheDocument();
});

test("filters the sidebar by tag", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Alpha", tags: ["home"] });
      seedTunnel({ name: "Beta", tags: ["work"] });
    },
  });
  await screen.findByRole("heading", { name: "Alpha" });

  await user.click(screen.getByTitle("Filter by tag"));
  await user.click(screen.getByRole("button", { name: "work" }));

  expect(screen.getByText("1 tunnel filtered")).toBeInTheDocument();
  expect(screen.queryByText("Alpha")).not.toBeInTheDocument();
  expect(tunnelItem("Beta")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Beta" })).toBeInTheDocument();
});

test("hides forward sections that do not match the search", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Alpha", forwards: [localForward] });
    },
  });
  expect(await screen.findByText("No configured Dynamic Port Forwarding.")).toBeInTheDocument();
  expect(screen.getByText("10.0.0.5 : 80")).toBeInTheDocument();

  await user.type(screen.getByPlaceholderText("Search"), "10.0.0.5");

  expect(screen.queryByText("No configured Dynamic Port Forwarding.")).not.toBeInTheDocument();
  expect(screen.queryByText("No configured Remote Port Forwarding.")).not.toBeInTheDocument();
  expect(screen.getByText("Local Port Forwarding")).toBeInTheDocument();
  expect(screen.getByText("10.0.0.5 : 80")).toBeInTheDocument();
});

test("connects and disconnects the selected tunnel", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ id: "alpha", name: "Alpha" });
    },
  });
  await screen.findByRole("heading", { name: "Alpha" });

  await user.click(screen.getByRole("button", { name: "Connect" }));
  expect(api.connectTunnel).toHaveBeenCalledWith("alpha");
  expect(screen.getByRole("button", { name: "Disconnect" })).toBeInTheDocument();
  expect(screen.getAllByText("Connected").length).toBeGreaterThan(0);

  await user.click(screen.getByRole("button", { name: "Disconnect" }));
  expect(api.disconnectTunnel).toHaveBeenCalledWith("alpha");
  expect(screen.getByRole("button", { name: "Connect" })).toBeInTheDocument();
});

test("applies a live status event to the matching tunnel", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ id: "alpha", name: "Alpha" });
      seedTunnel({ id: "beta", name: "Beta" });
    },
  });
  await screen.findByRole("heading", { name: "Alpha" });

  emit("tunnel:status", {
    hostId: "beta",
    snap: { Status: "error", Err: "connection refused", PID: 0, Logs: "boom" },
  });

  expect(tunnelItem("Beta")).toHaveTextContent("connection refused");
  expect(screen.getByRole("button", { name: "Connect" })).toBeInTheDocument();

  await user.click(tunnelItem("Beta"));
  expect(screen.getAllByText("connection refused").length).toBeGreaterThan(0);
});

test("reloads the list when the backend reports a config change", async () => {
  renderApp({
    prepare: () => {
      seedTunnel({ name: "Alpha" });
    },
  });
  await screen.findByRole("heading", { name: "Alpha" });

  seedTunnel({ name: "Beta" });
  emit("config:changed", {});

  expect(await screen.findByText("2 tunnels")).toBeInTheDocument();
  expect(tunnelItem("Beta")).toBeInTheDocument();
});

test("creates a tunnel from the empty state", async () => {
  const { user } = renderApp();
  await screen.findByText("No tunnels yet");

  await user.click(screen.getByRole("button", { name: "Create Tunnel" }));
  expect(await screen.findByText("New Tunnel — Unnamed")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Create" })).toBeDisabled();

  const name = screen.getByDisplayValue("Unnamed");
  await user.clear(name);
  await user.type(name, "Office");
  await user.type(screen.getByPlaceholderText("Hostname or IP address : 22"), "office.example");
  await user.type(screen.getByPlaceholderText("Username"), "ada");
  expect(screen.getByRole("button", { name: "Create" })).toBeEnabled();

  await user.click(screen.getByRole("button", { name: "Create" }));

  expect(await screen.findByRole("heading", { name: "Office" })).toBeInTheDocument();
  expect(screen.getByText("ada@office.example")).toBeInTheDocument();
  expect(tunnelByName("Office").profile.hostName).toBe("office.example");
  expect(api.saveTunnel).toHaveBeenCalled();
});

test("edits connection options and saves them", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office", hostName: "office.example", user: "ada" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });

  await user.click(screen.getByRole("button", { name: "Edit" }));
  expect(await screen.findByText("Edit Tunnel — Office")).toBeInTheDocument();

  const name = screen.getByDisplayValue("Office");
  await user.clear(name);
  await user.type(name, "Office VPN");
  await user.click(screen.getByRole("button", { name: "Connection" }));
  await user.click(screen.getByRole("button", { name: "DEBUG1" }));
  await user.type(screen.getByPlaceholderText("Jump hosts"), "jump.example");
  await user.click(screen.getByRole("button", { name: "Save" }));

  expect(await screen.findByRole("heading", { name: "Office VPN" })).toBeInTheDocument();
  expect(tunnelByName("Office VPN").profile.logLevel).toBe("DEBUG1");
  expect(tunnelByName("Office VPN").profile.proxyJump).toBe("jump.example");

  await user.click(screen.getByRole("button", { name: "Edit" }));
  await user.click(await screen.findByRole("button", { name: "Connection" }));
  expect(screen.getByRole("button", { name: "DEBUG1" })).toHaveClass("is-active");
  expect(screen.getByPlaceholderText("Jump hosts")).toHaveValue("jump.example");
});

test("discards editor changes on cancel", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office", hostName: "office.example" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });
  await user.click(screen.getByRole("button", { name: "Edit" }));
  const name = await screen.findByDisplayValue("Office");
  await user.clear(name);
  await user.type(name, "Renamed");
  await user.click(screen.getByRole("button", { name: "Cancel" }));

  expect(screen.getByRole("heading", { name: "Office" })).toBeInTheDocument();
  expect(api.saveTunnel).not.toHaveBeenCalled();
});

test("duplicates a tunnel into a new draft", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office", hostName: "office.example", user: "ada" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });
  await user.click(screen.getByRole("button", { name: "Duplicate" }));

  expect(await screen.findByText("New Tunnel — Office Copy")).toBeInTheDocument();
  expect(screen.getByDisplayValue("office.example")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Create" }));

  expect(await screen.findByRole("heading", { name: "Office Copy" })).toBeInTheDocument();
  expect(tunnelItem("Office")).toBeInTheDocument();
  expect(tunnelByName("Office Copy").profile.hostName).toBe("office.example");
  expect(tunnelByName("Office Copy").profile.id).not.toBe(tunnelByName("Office").profile.id);
});

test("asks before deleting and can cancel", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });

  fireEvent.contextMenu(tunnelItem("Office"));
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("menu")).not.toBeInTheDocument();

  fireEvent.contextMenu(tunnelItem("Office"));
  await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
  expect(await screen.findByRole("alertdialog")).toHaveTextContent("delete “Office”");
  await user.click(screen.getByRole("button", { name: "Cancel" }));

  expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  expect(api.deleteTunnel).not.toHaveBeenCalled();
  expect(screen.getByRole("heading", { name: "Office" })).toBeInTheDocument();
});

test("deletes a tunnel after confirmation", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ id: "office", name: "Office" });
      seedTunnel({ name: "Home" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });

  fireEvent.contextMenu(tunnelItem("Office"));
  await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
  await user.click(await screen.findByRole("button", { name: "Delete" }));

  expect(api.deleteTunnel).toHaveBeenCalledWith("office");
  expect(await screen.findByRole("heading", { name: "Home" })).toBeInTheDocument();
  expect(screen.queryByText("Office")).not.toBeInTheDocument();
});

test("pins a tunnel from the context menu so it sorts first", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Alpha" });
      seedTunnel({ name: "Beta" });
    },
  });
  await screen.findByRole("heading", { name: "Alpha" });

  fireEvent.contextMenu(tunnelItem("Beta"));
  expect(screen.getByRole("menuitem", { name: "Disconnect" })).toBeDisabled();
  expect(screen.getByRole("menuitem", { name: "Connect" })).toBeEnabled();
  await user.click(screen.getByRole("menuitem", { name: "Pin" }));

  await waitFor(() => {
    const names = tunnelItems().map((el) => el.querySelector(".tunnel-item-name-text")?.textContent);
    expect(names).toEqual(["Beta", "Alpha"]);
  });
  expect(tunnelByName("Beta").profile.pinned).toBe(true);
});

test("submits a password prompt and can dismiss it without saving", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ id: "alpha", name: "Alpha" });
    },
  });
  await screen.findByRole("heading", { name: "Alpha" });

  emit("tunnel:password", {
    requestId: "req-1",
    hostId: "alpha",
    prompt: "Password for root@alpha.example",
    kind: "password",
  });

  const dialog = await screen.findByRole("dialog");
  expect(dialog).toHaveTextContent("Password for root@alpha.example");
  expect(screen.getByRole("checkbox", { name: "Save password in Keychain" })).toBeChecked();
  const secret = dialog.querySelector("input[type='password']") as HTMLInputElement;
  await user.type(secret, "s3cret");
  await user.click(screen.getByRole("checkbox", { name: "Save password in Keychain" }));
  await user.click(screen.getByRole("button", { name: "OK" }));

  expect(api.submitPassword).toHaveBeenCalledWith("req-1", "s3cret", false);
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

  emit("tunnel:password", {
    requestId: "req-2",
    hostId: "alpha",
    prompt: "Enter passphrase",
    kind: "passphrase",
  });
  const again = await screen.findByRole("dialog");
  expect(screen.getByRole("checkbox", { name: "Save passphrase in Keychain" })).toBeChecked();
  await user.type(again.querySelector("input[type='password']") as HTMLInputElement, "{Escape}");
  expect(api.submitPassword).toHaveBeenCalledWith("req-2", "", false);
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

test("opens connection details, the ssh command, and closes on escape", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({
        name: "Office",
        hostName: "office.example",
        user: "ada",
        port: 2222,
        logs: "hello from ssh",
        status: "connected",
        pid: 99,
      });
    },
  });
  await screen.findByRole("heading", { name: "Office" });
  await user.click(screen.getByRole("button", { name: "Details…" }));

  const dialog = await screen.findByRole("dialog");
  expect(dialog).toHaveTextContent("ada@office.example");
  expect(dialog).toHaveTextContent("2222");
  expect(api.previewArgs).toHaveBeenCalled();

  await user.click(screen.getByRole("button", { name: "Logs" }));
  expect(screen.getByText("hello from ssh")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Command" }));
  expect(screen.getByText(/\/usr\/bin\/ssh/)).toHaveTextContent("ada@office.example");

  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

test("toggles auto-connect for the selected tunnel", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });
  const toggle = screen.getByRole("button", { name: "Toggle auto-connect" });
  expect(toggle).not.toHaveClass("is-on");

  await user.click(toggle);
  await waitFor(() => expect(screen.getByRole("button", { name: "Toggle auto-connect" })).toHaveClass("is-on"));
  expect(tunnelByName("Office").profile.autoConnect).toBe(true);
});

test("adds a tag from the tag menu", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office", tags: ["home"] });
    },
  });
  await screen.findByRole("heading", { name: "Office" });

  emit("menu:command", { command: "newTag" });
  const input = await screen.findByPlaceholderText("New tag…");
  await user.type(input, "work{Enter}");

  expect(await screen.findByRole("button", { name: "Remove work" })).toBeInTheDocument();
  expect(tunnelByName("Office").profile.tags).toEqual(["home", "work"]);
});

test("opens a new tunnel and hides the sidebar from menu commands", async () => {
  renderApp({
    prepare: () => {
      seedTunnel({ name: "Office" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });
  expect(screen.getByRole("separator", { name: "Resize tunnels sidebar" })).toBeInTheDocument();

  emit("menu:command", { command: "setSidebar", visible: false });
  expect(screen.queryByRole("separator", { name: "Resize tunnels sidebar" })).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Office" })).toBeInTheDocument();

  emit("menu:command", { command: "newTunnel" });
  expect(await screen.findByText(/New Tunnel —/)).toBeInTheDocument();
});

test("opens settings from the sidebar", async () => {
  const { user } = renderApp({
    prepare: () => {
      seedTunnel({ name: "Office" });
    },
  });
  await screen.findByRole("heading", { name: "Office" });
  await user.click(screen.getByRole("button", { name: "Settings" }));
  expect(api.openSettings).toHaveBeenCalledOnce();
});
