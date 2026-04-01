import Sidebar from './components/Sidebar.js';
import Dashboard from './pages/Dashboard.js';
import Tasks from './pages/Tasks.js';
import Accounts from './pages/Accounts.js';
import Settings from './pages/Settings.js';
import Onboarding from './pages/Onboarding.js';
import { EventBus } from '../utils/EventBus.js';

export default class App {
  constructor() {
    this.currentPage = this.getInitialPage();
    this.pages = {
      onboarding: new Onboarding(this),
      dashboard: new Dashboard(this),
      tasks: new Tasks(this),
      accounts: new Accounts(this),
      settings: new Settings(this)
    };
    this.isOnboarded = this.checkOnboardingStatus();
  }

  getInitialPage() {
    // Check URL hash for initial page
    const hash = window.location.hash.slice(1);
    return hash || 'dashboard';
  }

  checkOnboardingStatus() {
    // Check if user has completed onboarding
    return localStorage.getItem('syncghost_onboarded') === 'true';
  }

  init() {
    // If not onboarded, show onboarding page
    if (!this.isOnboarded) {
      this.showOnboarding();
    } else {
      this.render();
      this.setupEventListeners();
    }

    // Listen to hash changes for browser back/forward
    window.addEventListener('hashchange', () => {
      const page = window.location.hash.slice(1) || 'dashboard';
      if (page !== this.currentPage) {
        this.navigate(page);
      }
    });

    // Listen to global events
    EventBus.on('ONBOARDING_COMPLETE', () => {
      this.completeOnboarding();
    });

    EventBus.on('TASK_CREATED', () => {
      this.navigate('dashboard');
    });
  }

  showOnboarding() {
    const appContainer = document.getElementById('app');
    appContainer.innerHTML = this.pages.onboarding.render();
    this.pages.onboarding.attachEventListeners();
  }

  completeOnboarding() {
    localStorage.setItem('syncghost_onboarded', 'true');
    this.isOnboarded = true;
    this.render();
    this.setupEventListeners();
    window.showToast('配置完成，开始同步', 'success');
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
    if (page) {
      content.innerHTML = page.render();
      page.attachEventListeners?.();
    }
  }

  navigate(page) {
    // Don't navigate to onboarding if already onboarded
    if (page === 'onboarding' && this.isOnboarded) {
      return;
    }

    this.currentPage = page;
    
    // Update URL hash
    window.location.hash = page;
    
    // Update sidebar
    const sidebar = new Sidebar(this.currentPage, (p) => this.navigate(p));
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

  // Trigger onboarding manually
  startOnboarding() {
    this.showOnboarding();
  }
}
