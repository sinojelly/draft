import AddAccountModal from '../components/AddAccountModal.js';
import SuccessModal from '../components/SuccessModal.js';
import AddTaskModal from '../components/AddTaskModal.js';
import { EventBus } from '../../utils/EventBus.js';

export default class Onboarding {
  constructor(app) {
    this.app = app;
    this.currentStep = 'welcome';
    this.modals = {
      addAccount: null,
      success: null,
      addTask: null
    };
  }

  render() {
    return `
      <div class="onboarding-container">
        <div class="welcome-screen">
          <div class="welcome-icon">
            <svg width="120" height="120" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
            </svg>
          </div>
          <h1 class="welcome-title">欢迎使用 SyncGhost</h1>
          <p class="welcome-subtitle">一个支持多网盘/多账号/多目录的自动同步工具</p>
          <button class="btn btn-primary btn-large" id="startOnboardingBtn">
            开始使用
          </button>
        </div>
      </div>
      <div id="modalContainer"></div>
    `;
  }

  attachEventListeners() {
    const startBtn = document.getElementById('startOnboardingBtn');
    if (startBtn) {
      startBtn.addEventListener('click', () => this.startAccountSetup());
    }
  }

  startAccountSetup() {
    // Show add account modal
    this.modals.addAccount = new AddAccountModal(
      () => this.closeModal('addAccount'),
      (accountData) => this.handleAccountAdded(accountData)
    );
    
    const modalContainer = document.getElementById('modalContainer');
    if (modalContainer) {
      modalContainer.innerHTML = this.modals.addAccount.render();
      this.modals.addAccount.attachEventListeners();
    }
  }

  handleAccountAdded(accountData) {
    // Close add account modal
    this.closeModal('addAccount');
    
    // Show success modal
    setTimeout(() => {
      this.modals.success = new SuccessModal(
        () => this.closeModal('success'),
        () => this.handleConfigureTask()
      );
      
      const modalContainer = document.getElementById('modalContainer');
      if (modalContainer) {
        modalContainer.innerHTML = this.modals.success.render();
        this.modals.success.attachEventListeners();
      }
    }, 300);
  }

  handleConfigureTask() {
    // Close success modal
    this.closeModal('success');
    
    // Show add task modal
    setTimeout(() => {
      this.modals.addTask = new AddTaskModal(
        () => this.closeModal('addTask'),
        (taskData) => this.handleTaskAdded(taskData)
      );
      
      const modalContainer = document.getElementById('modalContainer');
      if (modalContainer) {
        modalContainer.innerHTML = this.modals.addTask.render();
        this.modals.addTask.attachEventListeners();
      }
    }, 300);
  }

  handleTaskAdded(taskData) {
    // Close add task modal
    this.closeModal('addTask');
    
    // Complete onboarding
    EventBus.emit('ONBOARDING_COMPLETE');
  }

  closeModal(modalName) {
    const modalContainer = document.getElementById('modalContainer');
    if (modalContainer) {
      modalContainer.innerHTML = '';
    }
    this.modals[modalName] = null;
  }
}
