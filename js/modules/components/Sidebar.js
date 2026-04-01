export default class Sidebar {
  constructor(activePage, onNavigate) {
    this.activePage = activePage;
    this.onNavigate = onNavigate;
  }

  render() {
    return `
      <aside class="sidebar">
        <div class="logo">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
          </svg>
          SyncGhost
        </div>
        
        <nav class="nav-menu">
          <div class="nav-item ${this.activePage === 'dashboard' ? 'active' : ''}" data-page="dashboard">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="7" height="7"/>
              <rect x="14" y="3" width="7" height="7"/>
              <rect x="14" y="14" width="7" height="7"/>
              <rect x="3" y="14" width="7" height="7"/>
            </svg>
            仪表盘
          </div>
          <div class="nav-item ${this.activePage === 'tasks' ? 'active' : ''}" data-page="tasks">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
            同步任务
          </div>
          <div class="nav-item ${this.activePage === 'accounts' ? 'active' : ''}" data-page="accounts">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/>
              <line x1="7" y1="7" x2="7.01" y2="7"/>
            </svg>
            网盘账号
          </div>
          <div class="nav-item ${this.activePage === 'settings' ? 'active' : ''}" data-page="settings">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="3"/>
              <path d="M12 1v6m0 6v6m4.22-13.22l-4.24 4.24m-2.83 2.83l-4.24 4.24m14.14-2.83l-4.24-4.24m-2.83-2.83l-4.24-4.24"/>
            </svg>
            系统设置
          </div>
        </nav>
        
        <div class="connection-status">
          <span class="status-dot"></span>
          已连接
        </div>
      </aside>
    `;
  }
}
