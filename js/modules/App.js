import Sidebar from './components/Sidebar.js';
import Dashboard from './pages/Dashboard.js';
import Tasks from './pages/Tasks.js';
import Accounts from './pages/Accounts.js';
import Settings from './pages/Settings.js';

export default class App {
  constructor() {
    this.currentPage = 'dashboard';
    this.pages = {
      dashboard: new Dashboard(),
      tasks: new Tasks(),
      accounts: new Accounts(),
      settings: new Settings()
    };
  }

  init() {
    this.render();
    this.setupEventListeners();
  }

  render() {
    const appContainer = document.getElementById('app');
    appContainer.innerHTML = `
      <div class="app-container">
        <div id="sidebar"></div>
        <main class="main-content" id="main-content"></main>
      </div>
    `;

    // Render sidebar
    const sidebar = new Sidebar(this.currentPage, (page) => this.navigate(page));
    document.getElementById('sidebar').innerHTML = sidebar.render();

    // Render current page
    this.renderPage();
  }

  renderPage() {
    const content = document.getElementById('main-content');
    const page = this.pages[this.currentPage];
    content.innerHTML = page.render();
    page.attachEventListeners?.();
  }

  navigate(page) {
    this.currentPage = page;
    
    // Update sidebar
    const sidebar = new Sidebar(this.currentPage, (page) => this.navigate(page));
    document.getElementById('sidebar').innerHTML = sidebar.render();
    
    // Render new page
    this.renderPage();
    
    // Re-attach navigation listeners
    this.setupEventListeners();
  }

  setupEventListeners() {
    const navItems = document.querySelectorAll('.nav-item');
    navItems.forEach(item => {
      item.addEventListener('click', () => {
        const page = item.getAttribute('data-page');
        this.navigate(page);
      });
    });
  }
}
