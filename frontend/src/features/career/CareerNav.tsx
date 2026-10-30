// Feature-local navigation shared by the career discovery and studio pages.

export function CareerNav(props: { active: "discovery" | "studio" }): React.JSX.Element {
  const { active } = props;
  return (
    <aside className="sidebar" aria-label="Primary navigation">
      <a className="brand" href="/jobs" aria-label="YourMove — Opportunities">
        <img
          className="brand-wordmark brand-light"
          src="/static/yourmove-wordmark-primary.svg"
          alt="YourMove"
          width="166"
          height="49"
        />
        <img
          className="brand-wordmark brand-dark"
          src="/static/yourmove-wordmark-reverse.svg"
          alt=""
          width="166"
          height="49"
        />
      </a>
      <nav className="primary-nav" aria-label="Workspace">
        <div className="nav-group">
          <p className="nav-caption">Work</p>
          <a className="nav-item" href="/jobs">
            Opportunities
          </a>
          <a className="nav-item" href="/applications">
            Submissions
          </a>
        </div>
        <div className="nav-group">
          <p className="nav-caption">Your materials</p>
          <a className="nav-item" href="/resumes">
            Resumes
          </a>
          <a
            className={active === "discovery" ? "nav-item is-active" : "nav-item"}
            aria-current={active === "discovery" ? "page" : undefined}
            href="/career"
          >
            Career discovery
          </a>
          <a
            className={active === "studio" ? "nav-item is-active" : "nav-item"}
            aria-current={active === "studio" ? "page" : undefined}
            href="/career/studio"
          >
            Career Studio
          </a>
          <a className="nav-item" href="/candidate">
            Candidate
          </a>
          <a className="nav-item" href="/answer-library">
            Answer library
          </a>
          <a className="nav-item" href="/career-evidence">
            Career evidence
          </a>
        </div>
        <div className="nav-group">
          <p className="nav-caption">Preferences and tools</p>
          <a className="nav-item" href="/ai">
            AI connections
          </a>
          <a className="nav-item" href="/utilities">
            Utilities
          </a>
        </div>
      </nav>
      <div className="sidebar-bottom">
        <span className="local-dot" aria-hidden="true" />
        <span>Local-first workspace</span>
      </div>
    </aside>
  );
}
