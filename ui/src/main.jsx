import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./styles.css";

// mountApp bootstraps the React UI into the root DOM node.
function mountApp() {
  const container = document.getElementById("root");
  if (!container) {
    console.error("ui startup failed: #root container was not found");
    return;
  }

  createRoot(container).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>
  );
}

mountApp();
