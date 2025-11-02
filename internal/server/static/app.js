// State
let currentStatus = null;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    loadStatus();
    loadAutostartStatus();

    // Poll for status updates every 3 seconds
    setInterval(loadStatus, 3000);
});

// Setup event listeners
function setupEventListeners() {
    // Launch at login toggle
    document.getElementById('launch-at-login').addEventListener('change', handleLaunchAtLoginToggle);

    // Open folder button
    document.getElementById('open-folder-btn').addEventListener('click', handleOpenFolder);

    // Service buttons
    document.getElementById('claude-btn').addEventListener('click', () => handleServiceAction('claude'));
    document.getElementById('codex-btn').addEventListener('click', () => handleServiceAction('codex'));
    document.getElementById('gemini-btn').addEventListener('click', () => handleServiceAction('gemini'));
    document.getElementById('qwen-btn').addEventListener('click', () => handleServiceAction('qwen'));

    // Qwen modal
    document.getElementById('qwen-cancel-btn').addEventListener('click', hideQwenModal);
    document.getElementById('qwen-continue-btn').addEventListener('click', handleQwenContinue);

    // Close modal on background click
    document.getElementById('qwen-modal').addEventListener('click', (e) => {
        if (e.target.id === 'qwen-modal') {
            hideQwenModal();
        }
    });
}

// Load status from API
async function loadStatus() {
    try {
        const response = await fetch('/api/status');
        if (!response.ok) throw new Error('Failed to fetch status');

        currentStatus = await response.json();
        updateUI();
    } catch (error) {
        console.error('Error loading status:', error);
        showToast('Failed to load status', 'error');
    }
}

// Load autostart status
async function loadAutostartStatus() {
    try {
        const response = await fetch('/api/autostart/status');
        if (!response.ok) throw new Error('Failed to fetch autostart status');

        const data = await response.json();
        document.getElementById('launch-at-login').checked = data.enabled;
    } catch (error) {
        console.error('Error loading autostart status:', error);
    }
}

// Update UI based on current status
function updateUI() {
    if (!currentStatus) return;

    // Update server status
    const serverStatusDot = document.getElementById('server-status-dot');
    const serverStatusText = document.getElementById('server-status-text');

    if (currentStatus.server.running) {
        serverStatusDot.classList.add('running');
        serverStatusText.textContent = 'Running';
    } else {
        serverStatusDot.classList.remove('running');
        serverStatusText.textContent = 'Stopped';
    }

    // Update service statuses
    updateServiceUI('claude', currentStatus.services.claude);
    updateServiceUI('codex', currentStatus.services.codex);
    updateServiceUI('gemini', currentStatus.services.gemini);
    updateServiceUI('qwen', currentStatus.services.qwen);
}

// Update individual service UI
function updateServiceUI(serviceName, serviceData) {
    const statusEl = document.getElementById(`${serviceName}-status`);
    const btn = document.getElementById(`${serviceName}-btn`);

    if (!serviceData || !serviceData.isAuthenticated) {
        statusEl.textContent = 'Not Connected';
        statusEl.className = 'service-status';
        btn.textContent = 'Connect';
        btn.className = 'btn';
    } else if (serviceData.expired && new Date(serviceData.expired) < new Date()) {
        statusEl.textContent = 'Expired - Reconnect Required';
        statusEl.className = 'service-status expired';
        btn.textContent = 'Reconnect';
        btn.className = 'btn reconnect';
    } else {
        const email = serviceData.email || 'Connected';
        statusEl.textContent = `Connected as ${email}`;
        statusEl.className = 'service-status connected';
        btn.textContent = 'Disconnect';
        btn.className = 'btn disconnect';
    }
}

// Handle service action (connect/disconnect/reconnect)
async function handleServiceAction(service) {
    const btn = document.getElementById(`${service}-btn`);
    const isConnected = currentStatus?.services[service]?.isAuthenticated;

    if (isConnected) {
        // Disconnect
        await handleDisconnect(service, btn);
    } else {
        // Connect or Reconnect
        if (service === 'qwen') {
            showQwenModal();
        } else {
            await handleConnect(service, btn);
        }
    }
}

// Handle connect action
async function handleConnect(service, btn) {
    const originalText = btn.textContent;
    btn.textContent = 'Connecting...';
    btn.classList.add('loading');
    btn.disabled = true;

    try {
        const response = await fetch('/api/auth/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ service })
        });

        if (!response.ok) throw new Error('Connection failed');

        const data = await response.json();

        if (data.success) {
            showToast(data.message, 'success');
            // Status will update via polling
        } else {
            showToast(data.message || 'Connection failed', 'error');
        }
    } catch (error) {
        console.error('Error connecting:', error);
        showToast('Connection failed', 'error');
    } finally {
        btn.textContent = originalText;
        btn.classList.remove('loading');
        btn.disabled = false;
    }
}

// Handle disconnect action
async function handleDisconnect(service, btn) {
    if (!confirm(`Are you sure you want to disconnect ${service}?`)) {
        return;
    }

    const originalText = btn.textContent;
    btn.textContent = 'Disconnecting...';
    btn.disabled = true;

    try {
        const response = await fetch('/api/auth/disconnect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ service })
        });

        if (!response.ok) throw new Error('Disconnection failed');

        const data = await response.json();
        showToast(data.message, 'success');

        // Refresh status
        await loadStatus();
    } catch (error) {
        console.error('Error disconnecting:', error);
        showToast('Disconnection failed', 'error');
    } finally {
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

// Handle launch at login toggle
async function handleLaunchAtLoginToggle(e) {
    const enabled = e.target.checked;
    const endpoint = enabled ? '/api/autostart/enable' : '/api/autostart/disable';

    try {
        const response = await fetch(endpoint, { method: 'POST' });
        if (!response.ok) throw new Error('Failed to update autostart');

        showToast(`Autostart ${enabled ? 'enabled' : 'disabled'}`, 'success');
    } catch (error) {
        console.error('Error updating autostart:', error);
        showToast('Failed to update autostart', 'error');
        // Revert toggle
        e.target.checked = !enabled;
    }
}

// Handle open folder
function handleOpenFolder() {
    showToast('Open your file manager and navigate to ~/.cli-proxy-api/', 'success');
}

// Show Qwen email modal
function showQwenModal() {
    document.getElementById('qwen-modal').classList.add('show');
    document.getElementById('qwen-email-input').value = '';
    document.getElementById('qwen-email-input').focus();
}

// Hide Qwen email modal
function hideQwenModal() {
    document.getElementById('qwen-modal').classList.remove('show');
}

// Handle Qwen continue
async function handleQwenContinue() {
    const email = document.getElementById('qwen-email-input').value.trim();

    if (!email) {
        showToast('Please enter an email address', 'error');
        return;
    }

    // Basic email validation
    if (!email.includes('@') || !email.includes('.')) {
        showToast('Please enter a valid email address', 'error');
        return;
    }

    hideQwenModal();

    const btn = document.getElementById('qwen-btn');
    const originalText = btn.textContent;
    btn.textContent = 'Connecting...';
    btn.classList.add('loading');
    btn.disabled = true;

    try {
        const response = await fetch('/api/auth/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ service: 'qwen', email })
        });

        if (!response.ok) throw new Error('Connection failed');

        const data = await response.json();

        if (data.success) {
            showToast(data.message, 'success');
        } else {
            showToast(data.message || 'Connection failed', 'error');
        }
    } catch (error) {
        console.error('Error connecting:', error);
        showToast('Connection failed', 'error');
    } finally {
        btn.textContent = originalText;
        btn.classList.remove('loading');
        btn.disabled = false;
    }
}

// Show toast notification
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast show ${type}`;

    setTimeout(() => {
        toast.classList.remove('show');
    }, 5000);
}
