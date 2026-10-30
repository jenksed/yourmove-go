import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { OpportunitiesPage } from "./features/opportunities/OpportunitiesPage";
import "./features/opportunities/opportunities.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("Opportunities root element missing");
}

createRoot(root).render(
  <StrictMode>
    <OpportunitiesPage />
  </StrictMode>,
);
