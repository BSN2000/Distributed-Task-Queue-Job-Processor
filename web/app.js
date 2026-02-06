// API Base URL - Use environment variable or default to Docker port
const API_BASE = window.API_BASE_URL || 'http://localhost:8081';

// Auto-refresh interval (3 seconds)
const REFRESH_INTERVAL = 3000;

// State
let refreshTimer = null;

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    setupJobForm();
    loadMetrics();
    loadAllJobs();
    startAutoRefresh();
});

// Setup job submission form
function setupJobForm() {
    const form = document.getElementById('jobForm');
    const formMessage = document.getElementById('formMessage');

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const tenantId = document.getElementById('tenantId').value.trim();
        const payload = document.getElementById('payload').value.trim();

        if (!tenantId || !payload) {
            showMessage(formMessage, 'Please fill in all fields', 'error');
            return;
        }

        // Disable submit button
        const submitBtn = form.querySelector('button[type="submit"]');
        submitBtn.disabled = true;
        submitBtn.textContent = 'Submitting...';

        try {
            const response = await fetch(`${API_BASE}/jobs`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    tenant_id: tenantId,
                    payload: payload
                })
            });

            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(errorText || `HTTP ${response.status}`);
            }

            const job = await response.json();
            showMessage(formMessage, `Job submitted successfully! ID: ${job.id}`, 'success');
            form.reset();

            // Refresh metrics and jobs immediately
            await loadMetrics();
            loadAllJobs();
        } catch (error) {
            showMessage(formMessage, `Error: ${error.message}`, 'error');
        } finally {
            submitBtn.disabled = false;
            submitBtn.textContent = 'Submit Job';
        }
    });
}

// Show message
function showMessage(element, message, type) {
    element.textContent = message;
    element.className = `message ${type}`;
    
    // Clear success messages after 5 seconds
    if (type === 'success') {
        setTimeout(() => {
            element.className = 'message';
            element.textContent = '';
        }, 5000);
    }
}

// Load metrics
async function loadMetrics() {
    try {
        // FIRST: Load metrics from /metrics endpoint (includes accurate total from database)
        // This endpoint returns total_jobs that includes DLQ jobs
        let totalJobsFromAPI = null;
        let retriedJobs = 0;
        
        try {
            const metricsResponse = await fetch(`${API_BASE}/metrics`);
            if (metricsResponse.ok) {
                const metrics = await metricsResponse.json();
                totalJobsFromAPI = metrics.total_jobs || 0;
                retriedJobs = metrics.retried_jobs || 0;
                // Set total jobs immediately from API (this includes DLQ)
                document.getElementById('metricTotalJobs').textContent = totalJobsFromAPI;
                document.getElementById('metricRetried').textContent = retriedJobs;
            }
        } catch (error) {
            console.error('Error loading metrics endpoint:', error);
        }

        // Load current status counts from API (shows current state)
        const statusCounts = {
            PENDING: 0,
            RUNNING: 0,
            DONE: 0,
            FAILED: 0
        };

        const statusPromises = [
            { status: 'PENDING', elementId: 'metricPending' },
            { status: 'RUNNING', elementId: 'metricRunning' },
            { status: 'DONE', elementId: 'metricCompleted' },
            { status: 'FAILED', elementId: 'metricFailed' }
        ].map(async ({ status, elementId }) => {
            try {
                const response = await fetch(`${API_BASE}/jobs?status=${status}`);
                if (response.ok) {
                    const jobs = await response.json();
                    const count = Array.isArray(jobs) ? jobs.length : 0;
                    statusCounts[status] = count;
                    document.getElementById(elementId).textContent = count;
                } else {
                    document.getElementById(elementId).textContent = '0';
                }
            } catch (error) {
                document.getElementById(elementId).textContent = '0';
            }
        });

        await Promise.all(statusPromises);

        // Load DLQ count
        let dlqCount = 0;
        try {
            const dlqResponse = await fetch(`${API_BASE}/dlq`);
            if (dlqResponse.ok) {
                const dlqJobs = await dlqResponse.json();
                dlqCount = Array.isArray(dlqJobs) ? dlqJobs.length : 0;
                document.getElementById('metricDLQ').textContent = dlqCount;
            } else {
                document.getElementById('metricDLQ').textContent = '0';
            }
        } catch (error) {
            document.getElementById('metricDLQ').textContent = '0';
        }

        // If metrics API didn't work, use fallback calculation
        if (totalJobsFromAPI === null) {
            const totalJobs = statusCounts.PENDING + statusCounts.RUNNING + statusCounts.DONE + statusCounts.FAILED + dlqCount;
            document.getElementById('metricTotalJobs').textContent = totalJobs;
        }
        
        // Set retried if not already set
        if (retriedJobs === 0) {
            document.getElementById('metricRetried').textContent = '0';
        }
    } catch (error) {
        // Silently fail - metrics will show "-" on error
        console.error('Error loading metrics:', error);
    }
}

// Load all jobs by status
async function loadAllJobs() {
    const statuses = ['PENDING', 'RUNNING', 'DONE', 'FAILED'];
    
    for (const status of statuses) {
        await loadJobsByStatus(status);
    }
    
    await loadDeadLetterQueue();
    // Also refresh metrics when loading jobs
    await loadMetrics();
}

// Load jobs by status
async function loadJobsByStatus(status) {
    const containerId = `${status.toLowerCase()}Jobs`;
    const container = document.getElementById(containerId);

    try {
        const response = await fetch(`${API_BASE}/jobs?status=${status}`);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const jobs = await response.json();
        renderJobsTable(container, jobs, status);
    } catch (error) {
        container.innerHTML = `<p class="error">Error loading ${status} jobs: ${error.message}</p>`;
    }
}

// Render jobs table
function renderJobsTable(container, jobs, status) {
    if (!jobs || jobs.length === 0) {
        container.innerHTML = '<p class="empty">No jobs found</p>';
        return;
    }

    let html = '<table><thead><tr>';
    html += '<th>Job ID</th>';
    html += '<th>Tenant ID</th>';
    html += '<th>Status</th>';
    html += '<th>Retries</th>';
    html += '<th>Created At</th>';
    html += '</tr></thead><tbody>';

    jobs.forEach(job => {
        html += '<tr>';
        html += `<td class="job-id">${escapeHtml(job.id)}</td>`;
        html += `<td>${escapeHtml(job.tenant_id)}</td>`;
        html += `<td>${escapeHtml(job.status)}</td>`;
        html += `<td>${job.retry_count || 0} / ${job.max_retries || 3}</td>`;
        html += `<td>${formatDate(job.created_at)}</td>`;
        html += '</tr>';
    });

    html += '</tbody></table>';
    container.innerHTML = html;
}

// Load Dead Letter Queue
async function loadDeadLetterQueue() {
    const container = document.getElementById('dlqJobs');

    try {
        const response = await fetch(`${API_BASE}/dlq`);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const dlqJobs = await response.json();
        renderDLQTable(container, dlqJobs);
    } catch (error) {
        container.innerHTML = `<p class="error">Error loading DLQ: ${error.message}</p>`;
    }
}

// Render Dead Letter Queue table
function renderDLQTable(container, dlqJobs) {
    if (!dlqJobs || dlqJobs.length === 0) {
        container.innerHTML = '<p class="empty">No jobs in Dead Letter Queue</p>';
        return;
    }

    let html = '<table><thead><tr>';
    html += '<th>Job ID</th>';
    html += '<th>Tenant ID</th>';
    html += '<th>Failure Reason</th>';
    html += '<th>Failed At</th>';
    html += '</tr></thead><tbody>';

    dlqJobs.forEach(job => {
        html += '<tr>';
        html += `<td class="job-id">${escapeHtml(job.job_id)}</td>`;
        html += `<td>${escapeHtml(job.tenant_id)}</td>`;
        html += `<td>${escapeHtml(job.failure_reason)}</td>`;
        html += `<td>${formatDate(job.failed_at)}</td>`;
        html += '</tr>';
    });

    html += '</tbody></table>';
    container.innerHTML = html;
}

// Format date
function formatDate(dateString) {
    if (!dateString) return 'N/A';
    
    try {
        const date = new Date(dateString);
        return date.toLocaleString();
    } catch (e) {
        return dateString;
    }
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    if (text == null) return '';
    
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Start auto-refresh
function startAutoRefresh() {
    if (refreshTimer) {
        clearInterval(refreshTimer);
    }
    
    refreshTimer = setInterval(() => {
        loadMetrics();
        loadAllJobs();
    }, REFRESH_INTERVAL);
}

// Stop auto-refresh (if needed)
function stopAutoRefresh() {
    if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = null;
    }
}
