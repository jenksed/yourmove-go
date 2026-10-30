import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { OpportunitiesPage } from "./features/opportunities/OpportunitiesPage";
import { applyStoredTheme } from "./shared/theme";
import "./shared/quiet-ink.css";
import "./features/opportunities/opportunities.css";

applyStoredTheme();

const root = document.getElementById("root");
if (!root) {
  throw new Error("Opportunities root element missing");
}

createRoot(root).render(
  <StrictMode>
    <OpportunitiesPage />
  </StrictMode>,
);
