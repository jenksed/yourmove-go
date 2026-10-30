import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { CareerPage } from "./features/career/CareerPage";
import { StudioPage } from "./features/career/StudioPage";
import { OpportunitiesPage } from "./features/opportunities/OpportunitiesPage";
import { ResumesPage } from "./features/resumes/ResumesPage";
import { applyStoredTheme } from "./shared/theme";
import "./shared/quiet-ink.css";
import "./features/opportunities/opportunities.css";
import "./features/resumes/resumes.css";
import "./features/career/career.css";

applyStoredTheme();

const root = document.getElementById("root");
if (!root) {
  throw new Error("Application root element missing");
}

// One bundle serves the migrated screens; Go routes (/jobs, /resumes,
// /career, /career/studio) select the entry so full-page navigation keeps
// working in Wails and the local browser. Unmigrated paths fall back to
// Opportunities. The /career match must not swallow the Go-rendered
// /career-evidence route.
const path = window.location.pathname;
const Page =
  path === "/career/studio" || path.startsWith("/career/studio/")
    ? StudioPage
    : path === "/career" || path.startsWith("/career/")
      ? CareerPage
      : path.startsWith("/resumes")
        ? ResumesPage
        : OpportunitiesPage;

createRoot(root).render(
  <StrictMode>
    <Page />
  </StrictMode>,
);
