import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { CareerPage } from "./features/career/CareerPage";
import { StudioPage } from "./features/career/StudioPage";
import { WorkspacePage } from "./features/application-workspace/WorkspacePage";
import { OpportunitiesPage } from "./features/opportunities/OpportunitiesPage";
import { ResumesPage } from "./features/resumes/ResumesPage";
import { applyStoredTheme } from "./shared/theme";
import "./shared/quiet-ink.css";
import "./features/opportunities/opportunities.css";
import "./features/application-workspace/workspace.css";
import "./features/resumes/resumes.css";
import "./features/career/career.css";

applyStoredTheme();

const root = document.getElementById("root");
if (!root) {
  throw new Error("Application root element missing");
}

// Career Discovery and Career Studio are parked for this version: their
// components, styles, and Go handlers stay intact and covered by existing
// tests, but no route selects them. Set careerDiscoveryEnabled to true to
// restore both screens (and their AppNavigation entries) in a future version.
const careerDiscoveryEnabled = false;

// One bundle serves the migrated screens; Go routes (/jobs, /jobs/:id,
// /resumes) select the entry so full-page navigation keeps working in Wails and the local browser. Unmigrated paths fall back to
// Opportunities. /jobs/:id serves the React workspace; ?legacy=1 (or
// draft/cover/question state) forces the Go-rendered legacy job page, and the
// workspace links there for writing, employer, and history work.
// /career-evidence is Go-rendered and never matched here; parked /career
// paths fall through to Opportunities while the flag above is false.
const path = window.location.pathname;
const search = window.location.search;
const legacyJobPage =
  search.includes("legacy=") ||
  search.includes("draft=") ||
  search.includes("cover=") ||
  search.includes("question=") ||
  search.includes("early_question=");
function isJobWorkspace(p: string): boolean {
  return /^\/jobs\/\d+(?:\/)?$/.test(p);
}
const Page =
  careerDiscoveryEnabled && (path === "/career/studio" || path.startsWith("/career/studio/"))
    ? StudioPage
    : careerDiscoveryEnabled && (path === "/career" || path.startsWith("/career/"))
      ? CareerPage
      : path.startsWith("/resumes")
        ? ResumesPage
        : isJobWorkspace(path) && !legacyJobPage
          ? WorkspacePage
          : OpportunitiesPage;

createRoot(root).render(
  <StrictMode>
    <Page />
  </StrictMode>,
);
