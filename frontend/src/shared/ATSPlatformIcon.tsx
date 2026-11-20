import { atsIconSrc, atsPlatformLabel, normalizeATSProvider } from "./atsPlatforms";

export interface ATSPlatformIconProps {
  // Go-owned provider key from the API ("" when unrecognized), identical in
  // the Opportunities list and the workspace snapshot. Rendered verbatim;
  // never re-derived here.
  provider?: string;
  // Go-owned accessible name for the platform.
  providerLabel?: string;
  className?: string;
}

// ATSPlatformIcon shows the locally bundled platform mark for a saved
// posting, or a neutral fallback where no reliable platform can be shown.
// The icon identifies the posting platform only; it never implies support
// for question discovery, browser automation, assisted Fill, or submission.
export function ATSPlatformIcon(props: ATSPlatformIconProps): React.JSX.Element {
  const platform = normalizeATSProvider(props.provider);
  const label = props.providerLabel?.trim() !== "" && props.providerLabel !== undefined
    ? props.providerLabel.trim()
    : atsPlatformLabel(platform);
  const name = label !== "" ? label : "Unknown source";
  const src = atsIconSrc(platform);
  const className = props.className !== undefined && props.className !== ""
    ? `ats-icon ${props.className}`
    : "ats-icon";

  if (src === null) {
    return (
      <span
        className={`${className} ats-icon-fallback`}
        role="img"
        aria-label={name}
        title={name}
      >
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true" focusable="false">
          <circle cx="8" cy="8" r="6.2" stroke="currentColor" strokeWidth="1.4" />
          <path
            d="M1.8 8h12.4M8 1.8c-3.6 3.7-3.6 8.7 0 12.4M8 1.8c3.6 3.7 3.6 8.7 0 12.4"
            stroke="currentColor"
            strokeWidth="1.1"
          />
        </svg>
      </span>
    );
  }

  return (
    <span className={className} title={name}>
      <img src={src} alt={name} draggable={false} />
    </span>
  );
}
