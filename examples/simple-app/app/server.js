/**
 * Simple Express.js web server
 */
const express = require('express');

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware for JSON parsing
app.use(express.json());

// Health check endpoint
app.get('/health', (req, res) => {
    res.json({ 
        status: 'healthy', 
        timestamp: new Date().toISOString(),
        uptime: process.uptime()
    });
});

// Root endpoint
app.get('/', (req, res) => {
    res.json({
        message: 'Hello from Simple WebApp!',
        version: '1.0.0',
        environment: process.env.NODE_ENV || 'development'
    });
});

// API endpoint
app.get('/api/status', (req, res) => {
    res.json({
        server: 'running',
        memory: process.memoryUsage(),
        uptime: process.uptime()
    });
});

// Start server
app.listen(PORT, '0.0.0.0', () => {
    console.log(`Server running on port ${PORT}`);
    console.log(`Environment: ${process.env.NODE_ENV || 'development'}`);
    console.log(`Health check: http://localhost:${PORT}/health`);
});