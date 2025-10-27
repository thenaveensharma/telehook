// Analytics functionality
// API_BASE is already declared in dashboard.js

// Chart instances
let timelineChart = null;
let statusChart = null;
let priorityChart = null;
let channelChart = null;

// Current time range
let currentRange = '24h';

// Token is already declared in dashboard.js, just reuse it

// Load analytics data - make it globally accessible
window.loadAnalytics = async function(timeRange = '24h') {
    console.log('loadAnalytics called with timeRange:', timeRange);
    currentRange = timeRange;

    const loadingEl = document.getElementById('loadingAnalytics');
    const contentEl = document.getElementById('analyticsContent');
    const noDataEl = document.getElementById('noAnalyticsData');

    console.log('Elements found:', {
        loadingEl: !!loadingEl,
        contentEl: !!contentEl,
        noDataEl: !!noDataEl
    });

    // Show loading
    if (loadingEl) loadingEl.style.display = 'block';
    if (contentEl) contentEl.style.display = 'none';
    if (noDataEl) noDataEl.style.display = 'none';

    try {
        console.log('Fetching analytics from:', `${API_BASE}/user/analytics?range=${timeRange}`);
        const response = await fetch(`${API_BASE}/user/analytics?range=${timeRange}`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });

        console.log('Response status:', response.status);

        if (response.status === 401) {
            localStorage.clear();
            window.location.href = '/login';
            return;
        }

        if (!response.ok) {
            throw new Error('Failed to fetch analytics: ' + response.status);
        }

        const data = await response.json();
        console.log('Analytics data received:', data);

        // Check if there's any data
        if (data.summary.total_messages === 0) {
            if (loadingEl) loadingEl.style.display = 'none';
            if (noDataEl) noDataEl.style.display = 'block';
            return;
        }

        // Display analytics
        displayAnalytics(data);

        if (loadingEl) loadingEl.style.display = 'none';
        if (contentEl) contentEl.style.display = 'block';
        console.log('Analytics loaded successfully');
    } catch (error) {
        console.error('Error loading analytics:', error);
        if (loadingEl) loadingEl.style.display = 'none';
        if (noDataEl) noDataEl.style.display = 'block';
    }
};

// Display analytics data
function displayAnalytics(data) {
    // Update summary cards
    document.getElementById('totalMessages').textContent = data.summary.total_messages;
    document.getElementById('successRate').textContent = data.summary.success_rate.toFixed(1) + '%';
    document.getElementById('avgPerHour').textContent = data.summary.avg_per_hour.toFixed(1);

    const peakHour = data.summary.peak_hour;
    const peakHourFormatted = peakHour === 0 ? '12 AM' :
                              peakHour < 12 ? `${peakHour} AM` :
                              peakHour === 12 ? '12 PM' :
                              `${peakHour - 12} PM`;
    document.getElementById('peakHour').textContent = `${peakHourFormatted} (${data.summary.peak_hour_count})`;

    // Create charts
    createTimelineChart(data.timeline);
    createStatusChart(data.status_distribution);
    createPriorityChart(data.priority_distribution);
    createChannelChart(data.channel_distribution);
}

// Create timeline chart (line chart)
function createTimelineChart(timeline) {
    const ctx = document.getElementById('timelineChart').getContext('2d');

    // Destroy existing chart
    if (timelineChart) {
        timelineChart.destroy();
    }

    const labels = timeline.map(point => {
        const date = new Date(point.timestamp);
        if (currentRange === '24h') {
            return date.toLocaleTimeString('en-US', { hour: 'numeric', hour12: true });
        } else if (currentRange === '7d') {
            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: 'numeric' });
        } else {
            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        }
    });

    timelineChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Success',
                    data: timeline.map(point => point.success_count),
                    borderColor: 'rgb(34, 197, 94)',
                    backgroundColor: 'rgba(34, 197, 94, 0.1)',
                    tension: 0.4,
                    fill: true
                },
                {
                    label: 'Failed',
                    data: timeline.map(point => point.failed_count),
                    borderColor: 'rgb(239, 68, 68)',
                    backgroundColor: 'rgba(239, 68, 68, 0.1)',
                    tension: 0.4,
                    fill: true
                },
                {
                    label: 'Filtered',
                    data: timeline.map(point => point.filtered_count),
                    borderColor: 'rgb(251, 191, 36)',
                    backgroundColor: 'rgba(251, 191, 36, 0.1)',
                    tension: 0.4,
                    fill: true
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            plugins: {
                legend: {
                    position: 'top',
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return context.dataset.label + ': ' + context.parsed.y + ' messages';
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        precision: 0
                    }
                }
            }
        }
    });
}

// Create status distribution chart (doughnut)
function createStatusChart(statusDist) {
    const ctx = document.getElementById('statusChart').getContext('2d');

    if (statusChart) {
        statusChart.destroy();
    }

    const statusColors = {
        'success': 'rgb(34, 197, 94)',
        'failed': 'rgb(239, 68, 68)',
        'filtered': 'rgb(251, 191, 36)',
        'pending': 'rgb(156, 163, 175)'
    };

    statusChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: statusDist.map(item => item.status.charAt(0).toUpperCase() + item.status.slice(1)),
            datasets: [{
                data: statusDist.map(item => item.count),
                backgroundColor: statusDist.map(item => statusColors[item.status] || 'rgb(156, 163, 175)'),
                borderWidth: 2,
                borderColor: '#fff'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom',
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            const label = context.label || '';
                            const value = context.parsed;
                            const percentage = statusDist[context.dataIndex].percentage.toFixed(1);
                            return `${label}: ${value} (${percentage}%)`;
                        }
                    }
                }
            }
        }
    });
}

// Create priority distribution chart (bar)
function createPriorityChart(priorityDist) {
    const ctx = document.getElementById('priorityChart').getContext('2d');

    if (priorityChart) {
        priorityChart.destroy();
    }

    const priorityColors = {
        1: 'rgb(239, 68, 68)',   // Urgent - Red
        2: 'rgb(251, 146, 60)',  // High - Orange
        3: 'rgb(59, 130, 246)',  // Normal - Blue
        4: 'rgb(156, 163, 175)'  // Low - Gray
    };

    priorityChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: priorityDist.map(item => item.label),
            datasets: [{
                label: 'Messages',
                data: priorityDist.map(item => item.count),
                backgroundColor: priorityDist.map(item => priorityColors[item.priority]),
                borderWidth: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            const value = context.parsed.y;
                            const percentage = priorityDist[context.dataIndex].percentage.toFixed(1);
                            return `${value} messages (${percentage}%)`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        precision: 0
                    }
                }
            }
        }
    });
}

// Create channel distribution chart (horizontal bar)
function createChannelChart(channelDist) {
    const ctx = document.getElementById('channelChart').getContext('2d');

    if (channelChart) {
        channelChart.destroy();
    }

    // Limit to top 10 channels
    const topChannels = channelDist.slice(0, 10);

    channelChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: topChannels.map(item => item.channel_identifier || 'Unknown'),
            datasets: [{
                label: 'Messages',
                data: topChannels.map(item => item.count),
                backgroundColor: 'rgb(99, 102, 241)',
                borderWidth: 0
            }]
        },
        options: {
            indexAxis: 'y',
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            const value = context.parsed.x;
                            const percentage = topChannels[context.dataIndex].percentage.toFixed(1);
                            return `${value} messages (${percentage}%)`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    beginAtZero: true,
                    ticks: {
                        precision: 0
                    }
                }
            }
        }
    });
}

// Initialize event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    console.log('Analytics.js DOMContentLoaded');
    console.log('window.loadAnalytics defined?', typeof window.loadAnalytics);

    // Time range selector event listeners
    document.querySelectorAll('.range-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            // Update active state
            document.querySelectorAll('.range-btn').forEach(b => b.classList.remove('active'));
            this.classList.add('active');

            // Load analytics with new range
            const range = this.getAttribute('data-range');
            window.loadAnalytics(range);
        });
    });
});
