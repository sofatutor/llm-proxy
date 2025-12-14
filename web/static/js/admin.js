// Admin UI JavaScript functionality

// Localize timestamps to viewer's timezone
function localizeTimestamps() {
    const elements = document.querySelectorAll('[data-local-time="true"][data-ts]');
    elements.forEach(function(el) {
        const tsStr = el.getAttribute('data-ts');
        if (!tsStr) return;
        
        try {
            const date = new Date(tsStr);
            if (isNaN(date.getTime())) return;
            
            const format = el.getAttribute('data-format') || 'ymd_hm';
            let formatted;
            
            switch(format) {
                case 'ymd_hm':
                    // YYYY-MM-DD HH:mm
                    formatted = formatDateTime(date, false);
                    break;
                case 'ymd_hms':
                    // YYYY-MM-DD HH:mm:ss
                    formatted = formatDateTime(date, true);
                    break;
                case 'ymd_hms_tz':
                    // YYYY-MM-DD HH:mm:ss TZ
                    formatted = formatDateTime(date, true) + ' ' + getTimezoneName(date);
                    break;
                case 'long':
                    // Monday, January 2, 2006 at 3:04 PM
                    formatted = formatLongDate(date);
                    break;
                case 'date_only':
                    // January 2, 2006
                    formatted = formatDateOnly(date);
                    break;
                default:
                    formatted = formatDateTime(date, false);
            }
            
            el.textContent = formatted;
        } catch (e) {
            console.error('Failed to localize timestamp:', e);
        }
    });
}

function formatDateTime(date, includeSeconds) {
    const pad = n => n.toString().padStart(2, '0');
    const year = date.getFullYear();
    const month = pad(date.getMonth() + 1);
    const day = pad(date.getDate());
    const hours = pad(date.getHours());
    const minutes = pad(date.getMinutes());
    
    let result = `${year}-${month}-${day} ${hours}:${minutes}`;
    if (includeSeconds) {
        const seconds = pad(date.getSeconds());
        result += `:${seconds}`;
    }
    return result;
}

function formatLongDate(date) {
    // Use a fixed locale to keep the admin UI stable across browser languages.
    const locale = 'en-US';
    const options = {
        weekday: 'long',
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        hour12: true
    };

    // Intl doesn't let us inject the literal " at " via options, so build it from parts.
    const parts = new Intl.DateTimeFormat(locale, options).formatToParts(date);
    const byType = {};
    parts.forEach(function(p) {
        if (!byType[p.type]) {
            byType[p.type] = p.value;
        }
    });

    if (!byType.weekday || !byType.month || !byType.day || !byType.year || !byType.hour || !byType.minute) {
        return new Intl.DateTimeFormat(locale, options).format(date);
    }

    const dayPeriod = byType.dayPeriod ? ` ${byType.dayPeriod}` : '';
    return `${byType.weekday}, ${byType.month} ${byType.day}, ${byType.year} at ${byType.hour}:${byType.minute}${dayPeriod}`;
}

function formatDateOnly(date) {
    const options = { 
        year: 'numeric', 
        month: 'long', 
        day: 'numeric'
    };
    return date.toLocaleDateString('en-US', options);
}

function getTimezoneName(date) {
    const tzString = date.toLocaleTimeString('en-US', { timeZoneName: 'short' });
    const parts = tzString.split(' ');
    return parts[parts.length - 1];
}

document.addEventListener('DOMContentLoaded', function() {
    // Localize all timestamps on page load
    localizeTimestamps();
    
    // Initialize tooltips
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });

    // Auto-dismiss alerts after 5 seconds
    const alerts = document.querySelectorAll('.alert-dismissible');
    alerts.forEach(function(alert) {
        setTimeout(function() {
            const bsAlert = new bootstrap.Alert(alert);
            bsAlert.close();
        }, 5000);
    });

    // Add loading state to forms
    const forms = document.querySelectorAll('form');
    forms.forEach(function(form) {
        form.addEventListener('submit', function() {
            const submitBtn = form.querySelector('button[type="submit"]');
            if (submitBtn) {
                submitBtn.disabled = true;
                submitBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>Processing...';
            }
        });
    });

    // Confirmation dialogs for dangerous actions
    const dangerousButtons = document.querySelectorAll('[data-confirm]');
    dangerousButtons.forEach(function(button) {
        button.addEventListener('click', function(e) {
            const message = this.getAttribute('data-confirm');
            if (!confirm(message)) {
                e.preventDefault();
                return false;
            }
        });
    });

    // Auto-refresh dashboard data every 30 seconds
    if (window.location.pathname === '/dashboard') {
        setInterval(refreshDashboard, 30000);
    }

    // System Status: Auto-update server time and backend status
    const serverTimeSpan = document.getElementById('server-time');
    const backendStatus = document.getElementById('backend-status');
    let serverTime = null;
    function updateBackendStatus() {
        fetch('/health', {cache: 'no-store'})
            .then(r => r.json())
            .then(data => {
                const backend = data.backend || {};
                if (backend.status && backend.status.toLowerCase() === 'ok') {
                    backendStatus.classList.remove('text-danger');
                    backendStatus.classList.add('text-success');
                    backendStatus.innerHTML = '<i class="bi bi-check-circle"></i> Backend: Online';
                } else {
                    backendStatus.classList.remove('text-success');
                    backendStatus.classList.add('text-danger');
                    backendStatus.innerHTML = '<i class="bi bi-x-circle"></i> Backend: Offline';
                }
                if (backend.timestamp && serverTimeSpan) {
                    serverTime = new Date(backend.timestamp);
                    updateServerTimeDisplay(true);
                }
            })
            .catch(() => {
                backendStatus.classList.remove('text-success');
                backendStatus.classList.add('text-danger');
                backendStatus.innerHTML = '<i class="bi bi-x-circle"></i> Backend: Offline';
            });
    }
    function updateServerTimeDisplay(updateAttributes) {
        if (!serverTimeSpan || !serverTime) return;
        // Format as YYYY-MM-DD HH:mm:ss
        const pad = n => n.toString().padStart(2, '0');
        const formatted = `${serverTime.getFullYear()}-${pad(serverTime.getMonth()+1)}-${pad(serverTime.getDate())} ${pad(serverTime.getHours())}:${pad(serverTime.getMinutes())}:${pad(serverTime.getSeconds())}`;
        serverTimeSpan.textContent = formatted;

        if (updateAttributes) {
            // Keep canonical UTC ISO timestamp in both attributes.
            const iso = serverTime.toISOString();
            serverTimeSpan.title = iso;
            serverTimeSpan.setAttribute('data-ts', iso);
        }
    }
    if (backendStatus) {
        updateBackendStatus();
        setInterval(updateBackendStatus, 10000);
    }
    if (serverTimeSpan) {
        setInterval(() => {
            if (serverTime) {
                serverTime.setSeconds(serverTime.getSeconds() + 1);
                updateServerTimeDisplay(false);
            }
        }, 1000);
    }
});

// Dashboard refresh functionality
function refreshDashboard() {
    fetch('/dashboard', {
        headers: {
            'X-Requested-With': 'XMLHttpRequest'
        }
    })
    .then(response => response.text())
    .then(html => {
        // Extract dashboard cards from response
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const newCards = doc.querySelectorAll('.card');
        const currentCards = document.querySelectorAll('.card');
        
        // Update card contents if they exist
        newCards.forEach((newCard, index) => {
            if (currentCards[index]) {
                const cardBody = newCard.querySelector('.card-body');
                const currentCardBody = currentCards[index].querySelector('.card-body');
                if (cardBody && currentCardBody) {
                    currentCardBody.innerHTML = cardBody.innerHTML;
                }
            }
        });
    })
    .catch(error => {
        console.log('Dashboard refresh failed:', error);
    });
}

// Utility functions
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        showToast('Copied to clipboard!', 'success');
    }, function(err) {
        showToast('Failed to copy to clipboard', 'error');
    });
}

function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toast-container') || createToastContainer();
    
    const toast = document.createElement('div');
    toast.className = `toast align-items-center text-white bg-${type === 'success' ? 'success' : type === 'error' ? 'danger' : 'primary'} border-0`;
    toast.setAttribute('role', 'alert');
    toast.innerHTML = `
        <div class="d-flex">
            <div class="toast-body">${message}</div>
            <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
        </div>
    `;
    
    toastContainer.appendChild(toast);
    
    const bsToast = new bootstrap.Toast(toast);
    bsToast.show();
    
    // Remove toast element after it's hidden
    toast.addEventListener('hidden.bs.toast', function() {
        toast.remove();
    });
}

function createToastContainer() {
    const container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'toast-container position-fixed top-0 end-0 p-3';
    container.style.zIndex = '1055';
    document.body.appendChild(container);
    return container;
}

// Form validation helpers
function validateForm(form) {
    const inputs = form.querySelectorAll('input[required], select[required], textarea[required]');
    let isValid = true;
    
    inputs.forEach(function(input) {
        if (!input.value.trim()) {
            input.classList.add('is-invalid');
            isValid = false;
        } else {
            input.classList.remove('is-invalid');
        }
    });
    
    return isValid;
}

// API request helper
async function apiRequest(url, options = {}) {
    const defaultOptions = {
        headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest'
        }
    };
    
    const mergedOptions = {
        ...defaultOptions,
        ...options,
        headers: {
            ...defaultOptions.headers,
            ...options.headers
        }
    };
    
    try {
        const response = await fetch(url, mergedOptions);
        
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
        }
        
        return await response.json();
    } catch (error) {
        console.error('API request failed:', error);
        throw error;
    }
}

// Table sorting (if needed)
function sortTable(table, column, direction = 'asc') {
    const tbody = table.querySelector('tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));
    
    rows.sort((a, b) => {
        const aVal = a.children[column].textContent.trim();
        const bVal = b.children[column].textContent.trim();
        
        if (direction === 'asc') {
            return aVal.localeCompare(bVal);
        } else {
            return bVal.localeCompare(aVal);
        }
    });
    
    rows.forEach(row => tbody.appendChild(row));
}

// Search functionality
function filterTable(input, tableId) {
    const filter = input.value.toUpperCase();
    const table = document.getElementById(tableId);
    const rows = table.querySelectorAll('tbody tr');
    
    rows.forEach(function(row) {
        const text = row.textContent.toUpperCase();
        if (text.indexOf(filter) > -1) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
}