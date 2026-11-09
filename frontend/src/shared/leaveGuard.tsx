// Leave-guard for ordinary full-page navigation.
//
// Anchors and windows cannot ask the applicant what to do with unsaved
// drafts, and the Wails host presents no beforeunload panel. Components
// wrap their content with the returned onNavigate handler; any same-window
// link out while the dirty registry is set is held with an inline choice
// instead: stay and keep editing, or discard the page and leave.
// Tab switches and in-app state keep their drafts and need no guard.
import { useCallback, useState } from "react";
import { hasUnsaved } from "./unsaved";

export function useLeaveGuard(): {
  intercept: (event: React.MouseEvent<HTMLElement>) => void;
  banner: React.JSX.Element | null;
} {
  const [leaveTo, setLeaveTo] = useState<string | null>(null);

  const intercept = useCallback(
    (event: React.MouseEvent<HTMLElement>) => {
      if (leaveTo !== null) return;
      const target = event.target as HTMLElement | null;
      const anchor = target?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!anchor) return;
      if (anchor.getAttribute("target") === "_blank") return;
      const href = anchor.getAttribute("href") ?? "";
      // Same-window application navigation only: in-page fragments,
      // external employer URLs, and new-tab views never unload drafts.
      if (!href.startsWith("/")) return;
      if (!hasUnsaved()) return;
      event.preventDefault();
      setLeaveTo(anchor.href);
    },
    [leaveTo],
  );

  const banner =
    leaveTo === null ? null : (
      <div role="alert" aria-label="Unsaved edits">
        <p className="writing-notice">
          You have unsaved edits. Leaving now will discard them.
        </p>
        <div className="form-actions">
          <button
            className="button button-secondary"
            type="button"
            onClick={() => {
              setLeaveTo(null);
            }}
          >
            Stay and keep editing
          </button>
          <button
            className="button button-secondary"
            type="button"
            onClick={() => {
              window.location.href = leaveTo;
            }}
          >
            Discard and leave
          </button>
        </div>
      </div>
    );

  return { intercept, banner };
}
