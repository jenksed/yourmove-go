export function coverLetterStatusText(letterStatus: string, ownsPosting: boolean): string {
  switch (letterStatus) {
    case "complete":
      return "A letter was selected and verified in the employer browser earlier in this session. Check the browser copy; it may differ from the current saved letter.";
    case "attention":
      return "Transfer attempted, but its result is uncertain. Inspect it in the employer browser.";
    case "waiting":
    case "checking":
      return "Transfer is pending; no result has been verified yet.";
    case "review":
      return "No verified transfer. Review the employer browser result below before retrying.";
    default:
      return ownsPosting
        ? "Saved locally; no transfer to the employer browser has occurred."
        : "Automatic handling is unavailable on this site. Attach the saved letter manually in the employer application.";
  }
}
