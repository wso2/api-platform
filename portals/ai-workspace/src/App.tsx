import "./App.css";

const sidebarItems = [
  "Overview",
  "Applications",
  "LLM Proxies",
  "LLM Service Providers",
  "External MCP Servers",
  "MCP Registries",
  "Insights",
];

function App() {
  return (
    <div className="app-root">
      <header className="app-header">
        <div className="app-header-title">AI Workspace</div>
        <div className="app-header-user">Admin</div>
      </header>

      <div className="app-body">
        <aside className="sidebar">
          <div className="sidebar-title">AI Workspace</div>
          <nav className="sidebar-nav">
            {sidebarItems.map((item) => (
              <div
                key={item}
                className={
                  "sidebar-item" + (item === "Overview" ? " active" : "")
                }
              >
                {item}
              </div>
            ))}
          </nav>
        </aside>

        <main className="main-content">
          <section className="cards-row">
            <div className="panel-card">LLM Service Providers</div>
            <div className="panel-card">External MCP Servers</div>
          </section>

          <section className="insights-panel">
            <div className="insights-chart">
              <div className="insights-axis" />
              <div className="insights-line" />
            </div>
            <div>
              <div className="insights-label">Admin Insights</div>
              <div className="insights-pie" />
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

export default App;


