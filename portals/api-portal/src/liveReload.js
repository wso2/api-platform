/*
 * Dev-only live-reload helper.
 * Registers an SSE endpoint (GET /__dev_reload) that the browser connects to.
 * Call notify() to broadcast a reload signal (done from the SIGUSR2 handler so
 * the message reaches the browser before the server goes down).
 * In production this module is never loaded.
 */
const clients = new Set();

function setup(app) {
    app.get('/__dev_reload', (req, res) => {
        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');
        res.setHeader('X-Accel-Buffering', 'no');
        res.flushHeaders();
        res.write('data: connected\n\n');

        const heartbeat = setInterval(() => {
            try { res.write(': ping\n\n'); } catch (_) {}
        }, 20000);

        clients.add(res);
        req.on('close', () => {
            clients.delete(res);
            clearInterval(heartbeat);
        });
    });
}

function notify() {
    for (const client of clients) {
        try { client.write('data: reload\n\n'); } catch (_) {}
    }
}

module.exports = { setup, notify };
