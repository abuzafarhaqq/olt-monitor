import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app/App";

// Load order matters: tokens define the variables, the rest consume them.
import "./styles/tokens.css";
import "./styles/global.css";
import "./styles/layout.css";
import "./styles/components.css";
import "./styles/dashboard.css";

const container = document.getElementById("root");
if (container === null) {
  throw new Error("index.html is missing #root");
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
