import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { OpportunitiesPage } from "./features/opportunities/OpportunitiesPage";
import { ResumesPage } from "./features/resumes/ResumesPage";
import { applyStoredTheme } from "./shared/theme";
import "./shared/quiet-ink.css";
import "./features/opportunities/opportunities.css";
import "./features/resumes/resumes.css";

applyStoredTheme();

const root = document.getElementById("root");
if (!root) {
  throw new Error("Application root element missing");
}

// One bundle serves both migrated screens; Go routes (/jobs, /resumes)
// select the entry so full-page navigation keeps working in Wails and the
// local browser. Unmigrated paths fall back to Opportunities.
const path = window.location.pathname;
const Page = path.startsWith("/resumes") ? ResumesPage : OpportunitiesPage;

createRoot(root).render(
  <StrictMode>
    <Page />
  </StrictMode>,
);
