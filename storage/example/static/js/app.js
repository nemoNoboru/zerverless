const API_BASE = '/example/api';

async function testHello() {
    try {
        const response = await fetch(`${API_BASE}/hello`);
        const data = await response.json();
        showResult('Hello Endpoint', data);
    } catch (error) {
        showResult('Error', { error: error.message });
    }
}

async function testUsers() {
    try {
        const response = await fetch(`${API_BASE}/users`);
        const data = await response.json();
        showResult('Users Endpoint', data);
    } catch (error) {
        showResult('Error', { error: error.message });
    }
}

async function testHealth() {
    try {
        const response = await fetch(`${API_BASE}/health`);
        const data = await response.json();
        showResult('Health Endpoint', data);
    } catch (error) {
        showResult('Error', { error: error.message });
    }
}

function showResult(title, data) {
    const resultDiv = document.getElementById('result');
    resultDiv.innerHTML = `<strong>${title}:</strong>\n${JSON.stringify(data, null, 2)}`;
}

