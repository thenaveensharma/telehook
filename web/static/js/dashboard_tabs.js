// ============================================================================
// Tab Management
// ============================================================================

document.querySelectorAll('.tab-btn').forEach(button => {
    button.addEventListener('click', () => {
        const tabName = button.getAttribute('data-tab');

        // Remove active class from all tabs and buttons
        document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));

        // Add active class to clicked button and corresponding content
        button.classList.add('active');
        document.getElementById(`${tabName}-tab`).classList.add('active');

        // Load data for the tab
        if (tabName === 'bots') {
            loadBots();
        } else if (tabName === 'channels') {
            loadChannels();
        } else if (tabName === 'activity') {
            loadWebhookInfo(); // Reload activity logs
        } else if (tabName === 'analytics') {
            // Load analytics (defined in analytics.js)
            console.log('Analytics tab clicked');
            console.log('window.loadAnalytics function exists?', typeof window.loadAnalytics);
            if (typeof window.loadAnalytics === 'function') {
                console.log('Calling window.loadAnalytics...');
                window.loadAnalytics('24h');
            } else {
                console.error('window.loadAnalytics function not found!');
            }
        }
    });
});

// ============================================================================
// Bots Management
// ============================================================================

let allBots = [];

async function loadBots() {
    try {
        const response = await fetch(`${API_BASE}/user/bots`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!response.ok) throw new Error('Failed to load bots');

        const data = await response.json();
        allBots = data.bots || [];

        displayBots(allBots);
    } catch (error) {
        console.error('Error loading bots:', error);
        document.getElementById('loadingBots').textContent = 'Error loading bots';
    }
}

function displayBots(bots) {
    const loadingBots = document.getElementById('loadingBots');
    const botsList = document.getElementById('botsList');
    const noBots = document.getElementById('noBots');

    loadingBots.style.display = 'none';

    if (!bots || bots.length === 0) {
        noBots.style.display = 'block';
        botsList.innerHTML = '';
        return;
    }

    noBots.style.display = 'none';
    botsList.innerHTML = '';

    bots.forEach(bot => {
        const botItem = document.createElement('div');
        botItem.className = 'bot-item';

        const botUsername = bot.bot_username || 'Bot';
        const defaultBadge = bot.is_default ? '<span class="bot-badge badge-default">Default</span>' : '';
        const tokenPreview = bot.bot_token.substring(0, 20) + '...';
        const addedDate = new Date(bot.created_at).toLocaleDateString();

        botItem.innerHTML = `
            <div class="bot-info">
                <h4>
                    🤖 ${botUsername}
                    ${defaultBadge}
                </h4>
                <p><strong>ID:</strong> ${bot.id}</p>
                <p><strong>Token:</strong> ${tokenPreview}</p>
                <p><strong>Added:</strong> ${addedDate}</p>
            </div>
            <div class="bot-actions">
                <button class="btn btn-danger btn-small" onclick="deleteBot(${bot.id})">Delete</button>
            </div>
        `;
        botsList.appendChild(botItem);
    });
}

// Add Bot Modal
const addBotBtn = document.getElementById('addBotBtn');
const addBotModal = document.getElementById('addBotModal');
const addBotForm = document.getElementById('addBotForm');

if (addBotBtn) {
    addBotBtn.addEventListener('click', () => {
        addBotModal.style.display = 'block';
        addBotForm.reset();
        document.getElementById('addBotError').style.display = 'none';
    });
}

// Close modals
document.querySelectorAll('.close').forEach(closeBtn => {
    closeBtn.addEventListener('click', function() {
        this.closest('.modal').style.display = 'none';
    });
});

// Close modal when clicking outside
window.addEventListener('click', (event) => {
    if (event.target.classList.contains('modal')) {
        event.target.style.display = 'none';
    }
});

// Add Bot Form Submit
if (addBotForm) {
    addBotForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const botToken = document.getElementById('botToken').value.trim();
        const isDefault = document.getElementById('botIsDefault').checked;

        try {
            const response = await fetch(`${API_BASE}/user/bots/`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({
                    bot_token: botToken,
                    is_default: isDefault
                })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Failed to add bot');
            }

            // Success
            addBotModal.style.display = 'none';
            loadBots();
            loadChannels(); // Reload for channel bot dropdown
        } catch (error) {
            const errorDiv = document.getElementById('addBotError');
            errorDiv.textContent = error.message;
            errorDiv.style.display = 'block';
        }
    });
}

async function deleteBot(botId) {
    if (!confirm('Are you sure you want to delete this bot? All associated channels will also be removed.')) {
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/user/bots/${botId}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!response.ok) {
            throw new Error('Failed to delete bot');
        }

        loadBots();
        loadChannels();
    } catch (error) {
        alert('Error deleting bot: ' + error.message);
    }
}

// ============================================================================
// Channels Management
// ============================================================================

let allChannels = [];

async function loadChannels() {
    try {
        const response = await fetch(`${API_BASE}/user/channels`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!response.ok) throw new Error('Failed to load channels');

        const data = await response.json();
        allChannels = data.channels || [];

        displayChannels(allChannels);

        // Also load bots for the channel modal dropdown
        await loadBotsForDropdown();
    } catch (error) {
        console.error('Error loading channels:', error);
        document.getElementById('loadingChannels').textContent = 'Error loading channels';
    }
}

function displayChannels(channels) {
    const loadingChannels = document.getElementById('loadingChannels');
    const channelsList = document.getElementById('channelsList');
    const noChannels = document.getElementById('noChannels');

    loadingChannels.style.display = 'none';

    if (!channels || channels.length === 0) {
        noChannels.style.display = 'block';
        channelsList.innerHTML = '';
        return;
    }

    noChannels.style.display = 'none';
    channelsList.innerHTML = '';

    channels.forEach(channel => {
        const channelItem = document.createElement('div');
        channelItem.className = 'channel-item';

        const channelName = channel.channel_name || channel.identifier;
        const statusBadge = channel.is_active ?
            '<span class="channel-badge badge-active">Active</span>' :
            '<span class="channel-badge badge-inactive">Inactive</span>';
        const descriptionLine = channel.description ?
            `<p><strong>Description:</strong> ${channel.description}</p>` : '';
        const addedDate = new Date(channel.created_at).toLocaleDateString();

        channelItem.innerHTML = `
            <div class="channel-info">
                <h4>
                    📢 ${channelName}
                    ${statusBadge}
                </h4>
                <p><strong>Identifier:</strong> <code>${channel.identifier}</code></p>
                <p><strong>Channel ID:</strong> ${channel.channel_id}</p>
                ${descriptionLine}
                <p><strong>Added:</strong> ${addedDate}</p>
            </div>
            <div class="channel-actions">
                <button class="btn btn-danger btn-small" onclick="deleteChannel(${channel.id})">Delete</button>
            </div>
        `;
        channelsList.appendChild(channelItem);
    });
}

// Load bots for channel dropdown
async function loadBotsForDropdown() {
    try {
        const response = await fetch(`${API_BASE}/user/bots`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!response.ok) return;

        const data = await response.json();
        const bots = data.bots || [];

        const select = document.getElementById('channelBot');
        if (select) {
            select.innerHTML = '<option value="">-- Select a bot --</option>';
            bots.forEach(bot => {
                const option = document.createElement('option');
                option.value = bot.id;
                option.textContent = bot.bot_username || `Bot ${bot.id}`;
                select.appendChild(option);
            });
        }
    } catch (error) {
        console.error('Error loading bots for dropdown:', error);
    }
}

// Add Channel Modal
const addChannelBtn = document.getElementById('addChannelBtn');
const addChannelModal = document.getElementById('addChannelModal');
const addChannelForm = document.getElementById('addChannelForm');

if (addChannelBtn) {
    addChannelBtn.addEventListener('click', async () => {
        await loadBotsForDropdown();
        addChannelModal.style.display = 'block';
        addChannelForm.reset();
        document.getElementById('addChannelError').style.display = 'none';
    });
}

// Add Channel Form Submit
if (addChannelForm) {
    addChannelForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const botId = parseInt(document.getElementById('channelBot').value);
        const identifier = document.getElementById('channelIdentifier').value.trim();
        const channelId = document.getElementById('channelId').value.trim();
        const channelName = document.getElementById('channelName').value.trim();
        const description = document.getElementById('channelDescription').value.trim();

        try {
            const response = await fetch(`${API_BASE}/user/channels/`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({
                    bot_id: botId,
                    identifier: identifier,
                    channel_id: channelId,
                    channel_name: channelName,
                    description: description
                })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Failed to add channel');
            }

            // Success
            addChannelModal.style.display = 'none';
            loadChannels();
        } catch (error) {
            const errorDiv = document.getElementById('addChannelError');
            errorDiv.textContent = error.message;
            errorDiv.style.display = 'block';
        }
    });
}

async function deleteChannel(channelId) {
    if (!confirm('Are you sure you want to delete this channel?')) {
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/user/channels/${channelId}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        if (!response.ok) {
            throw new Error('Failed to delete channel');
        }

        loadChannels();
    } catch (error) {
        alert('Error deleting channel: ' + error.message);
    }
}
