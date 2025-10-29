// API Base URL
const API_BASE = '/api';

// Check if user is logged in
const token = localStorage.getItem('token');
if (!token) {
    window.location.href = '/login';
}

// Display username
const username = localStorage.getItem('username');
const usernameDisplay = document.getElementById('usernameDisplay');
if (usernameDisplay && username) {
    usernameDisplay.textContent = `Hi, ${username}`;
}

// Logout handler
const logoutBtn = document.getElementById('logoutBtn');
if (logoutBtn) {
    logoutBtn.addEventListener('click', () => {
        localStorage.clear();
        window.location.href = '/';
    });
}

// Fetch webhook info
async function loadWebhookInfo() {
    try {
        const response = await fetch(`${API_BASE}/user/webhook-info`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (response.status === 401) {
            localStorage.clear();
            window.location.href = '/login';
            return;
        }

        const data = await response.json();

        if (response.ok) {
            // Display webhook URL
            const webhookUrlInput = document.getElementById('webhookUrl');
            const webhookTokenEl = document.getElementById('webhookToken');
            const exampleUrlEl = document.getElementById('exampleUrl');

            if (webhookUrlInput) {
                webhookUrlInput.value = data.webhook_url;
            }

            if (webhookTokenEl) {
                webhookTokenEl.textContent = data.webhook_token;
            }

            if (exampleUrlEl) {
                exampleUrlEl.textContent = data.webhook_url;
            }

            // Display recent logs
            displayLogs(data.recent_logs);
        }
    } catch (error) {
        console.error('Error loading webhook info:', error);
    }
}

// Copy webhook URL to clipboard
const copyBtn = document.getElementById('copyBtn');
if (copyBtn) {
    copyBtn.addEventListener('click', () => {
        const webhookUrl = document.getElementById('webhookUrl');
        webhookUrl.select();
        document.execCommand('copy');

        copyBtn.textContent = 'Copied!';
        setTimeout(() => {
            copyBtn.textContent = 'Copy';
        }, 2000);
    });
}

// Display webhook logs
function displayLogs(logs) {
    const loadingLogs = document.getElementById('loadingLogs');
    const logsContainer = document.getElementById('logsContainer');
    const logsList = document.getElementById('logsList');
    const noLogs = document.getElementById('noLogs');

    loadingLogs.style.display = 'none';

    if (!logs || logs.length === 0) {
        noLogs.style.display = 'block';
        return;
    }

    logsContainer.style.display = 'block';
    logsList.innerHTML = '';

    // Calculate stats
    const stats = logs.reduce((acc, log) => {
        acc[log.status] = (acc[log.status] || 0) + 1;
        return acc;
    }, {});

    // Update stats display
    const activityStats = document.getElementById('activityStats');
    if (activityStats && logs.length > 0) {
        document.getElementById('successCount').textContent = stats.success || 0;
        document.getElementById('failedCount').textContent = stats.failed || 0;
        document.getElementById('filteredCount').textContent = stats.filtered || 0;
        activityStats.style.display = 'flex';
    }

    logs.forEach(log => {
        const logItem = document.createElement('div');
        logItem.className = `log-item ${log.status}`;

        const date = new Date(log.sent_at);
        const formattedDate = date.toLocaleString();

        let payload;
        try {
            payload = JSON.parse(log.payload);
        } catch (e) {
            payload = log.payload;
        }

        // Get status icon
        const statusIcon = {
            'success': '✅',
            'failed': '❌',
            'filtered': '🚫',
            'pending': '⏳'
        }[log.status] || '•';

        // Extract message preview (first 100 chars)
        const messagePreview = payload.message ?
            (payload.message.length > 100 ? payload.message.substring(0, 100) + '...' : payload.message) :
            'N/A';

        // Format telegram response/reason
        let reasonHTML = '';
        if (log.telegram_response && log.status !== 'success') {
            const reason = log.telegram_response.includes('message_id') ?
                'Message sent successfully' :
                log.telegram_response;
            reasonHTML = `
                <div class="log-reason ${log.status}">
                    <strong>Reason:</strong> <span class="reason-text">${reason}</span>
                </div>
            `;
        } else if (log.status === 'success' && log.telegram_response) {
            try {
                const response = JSON.parse(log.telegram_response);
                reasonHTML = `
                    <div class="log-reason success">
                        <strong>Delivered:</strong> Message ID ${response.message_id || 'N/A'}
                    </div>
                `;
            } catch (e) {
                // If not JSON, skip
            }
        }

        logItem.innerHTML = `
            <div class="log-header">
                <span class="log-status ${log.status}">${statusIcon} ${log.status.toUpperCase()}</span>
                <span class="log-date">${formattedDate}</span>
            </div>
            <div class="log-payload">
                <strong>Message:</strong> ${messagePreview}
            </div>
            ${reasonHTML}
        `;

        logsList.appendChild(logItem);
    });
}

// Load channels for test webhook dropdown
async function loadChannelsForTest() {
    try {
        const response = await fetch(`${API_BASE}/user/channels`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!response.ok) return;

        const data = await response.json();
        const channels = data.channels || [];

        const select = document.getElementById('testChannelIdentifier');
        if (select) {
            select.innerHTML = '<option value="">(Optional) Send to default channel</option>';

            if (channels.length === 0) {
                const option = document.createElement('option');
                option.value = '';
                option.textContent = 'No channels configured - Add one in Channels tab';
                option.disabled = true;
                select.appendChild(option);
            } else {
                channels.forEach(channel => {
                    if (channel.is_active) {
                        const option = document.createElement('option');
                        option.value = channel.identifier;
                        option.textContent = `${channel.identifier} - ${channel.channel_name || channel.channel_id}`;
                        select.appendChild(option);
                    }
                });
            }
        }
    } catch (error) {
        console.error('Error loading channels for test:', error);
    }
}

// Convert Telegram Markdown to HTML for preview
function renderTelegramMarkdown(text) {
    if (!text) return 'Type a message to see preview...';

    // Escape HTML first
    let html = text
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');

    // Code blocks (```)
    html = html.replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>');

    // Links [text](url)
    html = html.replace(/\[([^\]]+)\]\(([^\)]+)\)/g, '<a href="$2" target="_blank">$1</a>');

    // Bold *text*
    html = html.replace(/\*([^\*]+)\*/g, '<strong>$1</strong>');

    // Italic _text_
    html = html.replace(/_([^_]+)_/g, '<em>$1</em>');

    // Inline code `text`
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

    // Convert line breaks to <br>
    html = html.replace(/\n/g, '<br>');

    return html;
}

// Update message preview
function updateMessagePreview() {
    const message = document.getElementById('testMessage').value;
    const identifier = document.getElementById('testChannelIdentifier').value;
    const preview = document.getElementById('previewContent');

    if (message) {
        // Show what will actually be sent to Telegram (without identifier)
        const cleanMessage = message;
        preview.innerHTML = renderTelegramMarkdown(cleanMessage);
    } else {
        preview.innerHTML = 'Type a message to see preview...';
    }
}

// Add event listeners for preview update
const testMessageInput = document.getElementById('testMessage');
const testIdentifierSelect = document.getElementById('testChannelIdentifier');

if (testMessageInput) {
    testMessageInput.addEventListener('input', updateMessagePreview);
}

if (testIdentifierSelect) {
    testIdentifierSelect.addEventListener('change', updateMessagePreview);
}

// Refresh channels button
const refreshChannelsBtn = document.getElementById('refreshChannelsBtn');
if (refreshChannelsBtn) {
    refreshChannelsBtn.addEventListener('click', () => {
        loadChannelsForTest();
    });
}

// Test webhook form handler
const testWebhookForm = document.getElementById('testWebhookForm');
if (testWebhookForm) {
    testWebhookForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const message = document.getElementById('testMessage').value;
        const identifier = document.getElementById('testChannelIdentifier').value;
        const priority = parseInt(document.getElementById('testPriority').value);
        const errorMessage = document.getElementById('testErrorMessage');
        const successMessage = document.getElementById('testSuccessMessage');

        errorMessage.style.display = 'none';
        successMessage.style.display = 'none';

        // Format message with identifier (only if provided)
        const formattedMessage = identifier ? `${message}\n----\n${identifier}` : message;

        const payload = {
            message: formattedMessage,
            priority: priority
        };

        try {
            const webhookUrl = document.getElementById('webhookUrl').value;
            const response = await fetch(webhookUrl, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(payload)
            });

            const result = await response.json();

            if (response.ok) {
                const channelName = result.channel || 'your channel';
                const identifierMsg = identifier ? ` (identifier: ${result.identifier || identifier})` : ' (default channel)';
                successMessage.textContent = `✅ Message sent successfully to "${channelName}"${identifierMsg}! Check your Telegram channel.`;
                successMessage.style.display = 'block';

                // Reload webhook info to show new log
                setTimeout(() => {
                    loadWebhookInfo();
                }, 1000);
            } else {
                errorMessage.textContent = result.error || 'Failed to send message';
                if (result.hint) {
                    errorMessage.textContent += ` - ${result.hint}`;
                }
                errorMessage.style.display = 'block';
            }
        } catch (error) {
            errorMessage.textContent = 'Network error. Please try again.';
            errorMessage.style.display = 'block';
        }
    });
}

// Load webhook info and channels on page load
loadWebhookInfo();
loadChannelsForTest();
