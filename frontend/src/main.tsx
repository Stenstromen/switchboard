import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

// Disable spellcheck on every text field (inherited from <html>, plus any
// dynamically created inputs that might reset it).
function disableSpellcheck(root: ParentNode = document) {
  root.querySelectorAll("input, textarea").forEach((el) => {
    el.setAttribute("spellcheck", "false");
    (el as HTMLInputElement).spellcheck = false;
  });
}

const rootEl = document.getElementById("root") as HTMLElement;
ReactDOM.createRoot(rootEl).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);

disableSpellcheck();
const observer = new MutationObserver((mutations) => {
  for (const m of mutations) {
    m.addedNodes.forEach((node) => {
      if (!(node instanceof HTMLElement)) return;
      if (node.matches?.("input, textarea")) {
        node.setAttribute("spellcheck", "false");
        (node as HTMLInputElement).spellcheck = false;
      }
      disableSpellcheck(node);
    });
  }
});
observer.observe(rootEl, { childList: true, subtree: true });
