class VolunteerClient {
    constructor() {
        this.ws = null;
        this.volunteerId = null;
        this.jobsCompleted = 0;
        this.startTime = null;
        this.heartbeatInterval = null;
        this.uptimeInterval = null;

        this.elements = {
            statusIndicator: document.getElementById('statusIndicator'),
            statusText: document.getElementById('statusText'),
            volunteerId: document.getElementById('volunteerId'),
            jobsCompleted: document.getElementById('jobsCompleted'),
            uptime: document.getElementById('uptime'),
            connectBtn: document.getElementById('connectBtn'),
            disconnectBtn: document.getElementById('disconnectBtn'),
            log: document.getElementById('log'),
        };

        this.elements.connectBtn.addEventListener('click', () => this.connect());
        this.elements.disconnectBtn.addEventListener('click', () => this.disconnect());
    }

    connect() {
        const wsUrl = `ws://${window.location.host}/ws/volunteer`;
        this.log('Connecting...', 'info');
        this.setStatus('connecting');

        try {
            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                this.log('Connected to orchestrator', 'success');
                this.startTime = Date.now();
                this.startHeartbeat();
                this.startUptimeCounter();
                this.elements.connectBtn.disabled = true;
                this.elements.disconnectBtn.disabled = false;
            };

            this.ws.onmessage = (event) => this.handleMessage(JSON.parse(event.data));

            this.ws.onclose = () => {
                this.log('Disconnected', 'info');
                this.cleanup();
            };

            this.ws.onerror = (error) => {
                this.log('Connection error', 'error');
                this.cleanup();
            };
        } catch (error) {
            this.log(`Failed to connect: ${error.message}`, 'error');
            this.cleanup();
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
        }
    }

    handleMessage(msg) {
        switch (msg.type) {
            case 'ack':
                this.volunteerId = msg.volunteer_id;
                this.elements.volunteerId.textContent = `ID: ${msg.volunteer_id.slice(0, 8)}...`;
                this.setStatus('connected');
                this.log(`Welcome! ${msg.message}`, 'success');
                this.sendReady();
                break;

            case 'job':
                this.setStatus('busy');
                this.log(`Received job: ${msg.job_id.slice(0, 8)}...`, 'info');
                this.executeJob(msg);
                break;

            case 'heartbeat':
                // Server acknowledged heartbeat
                break;

            case 'cancel':
                this.log(`Job cancelled: ${msg.job_id.slice(0, 8)}...`, 'info');
                this.setStatus('connected');
                this.sendReady();
                break;

            default:
                this.log(`Unknown message: ${msg.type}`, 'error');
        }
    }

    sendReady() {
        this.send({
            type: 'ready',
            capabilities: {
                wasm: true,
                max_memory_mb: 128
            }
        });
    }

    async executeJob(job) {
        try {
            // For Phase 1, just simulate execution
            this.log(`Executing job...`, 'info');
            await this.sleep(1000 + Math.random() * 2000);

            // Simulate result
            const result = { success: true, output: 42 };

            this.send({
                type: 'result',
                job_id: job.job_id,
                result: result
            });

            this.jobsCompleted++;
            this.elements.jobsCompleted.textContent = this.jobsCompleted;
            this.log(`Job completed: ${job.job_id.slice(0, 8)}...`, 'success');
            this.setStatus('connected');
            this.sendReady();

        } catch (error) {
            this.send({
                type: 'error',
                job_id: job.job_id,
                error: error.message
            });
            this.log(`Job failed: ${error.message}`, 'error');
            this.setStatus('connected');
            this.sendReady();
        }
    }

    send(msg) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(msg));
        }
    }

    startHeartbeat() {
        this.heartbeatInterval = setInterval(() => {
            this.send({ type: 'heartbeat' });
        }, 30000);
    }

    startUptimeCounter() {
        this.uptimeInterval = setInterval(() => {
            if (this.startTime) {
                const elapsed = Math.floor((Date.now() - this.startTime) / 1000);
                const mins = Math.floor(elapsed / 60);
                const secs = elapsed % 60;
                this.elements.uptime.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
            }
        }, 1000);
    }

    cleanup() {
        if (this.heartbeatInterval) clearInterval(this.heartbeatInterval);
        if (this.uptimeInterval) clearInterval(this.uptimeInterval);
        this.ws = null;
        this.volunteerId = null;
        this.startTime = null;
        this.setStatus('disconnected');
        this.elements.volunteerId.textContent = '';
        this.elements.connectBtn.disabled = false;
        this.elements.disconnectBtn.disabled = true;
    }

    setStatus(status) {
        this.elements.statusIndicator.className = 'status-indicator ' + status;
        const labels = {
            disconnected: 'Disconnected',
            connecting: 'Connecting...',
            connected: 'Ready',
            busy: 'Processing Job'
        };
        this.elements.statusText.textContent = labels[status] || status;
    }

    log(message, type = '') {
        const entry = document.createElement('div');
        entry.className = 'log-entry ' + type;
        const time = new Date().toLocaleTimeString();
        entry.innerHTML = `<span class="log-time">${time}</span>${message}`;
        this.elements.log.insertBefore(entry, this.elements.log.firstChild);

        // Keep only last 50 entries
        while (this.elements.log.children.length > 50) {
            this.elements.log.removeChild(this.elements.log.lastChild);
        }
    }

    sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }
}

// Initialize
const volunteer = new VolunteerClient();

