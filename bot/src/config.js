import 'dotenv/config';

function required(name) {
  const v = process.env[name];
  if (!v) {
    console.error(`Missing required env var: ${name}`);
    process.exit(1);
  }
  return v;
}

export const config = {
  botToken: required('TELEGRAM_BOT_TOKEN'),
  backendURL: (process.env.BACKEND_URL || 'http://localhost:8080').replace(/\/+$/, ''),
  botServiceToken: required('BOT_SERVICE_TOKEN'),
  orderCommand: (process.env.ORDER_COMMAND || 'order').replace(/^\//, ''),
  pollIntervalMs: Number(process.env.POLL_INTERVAL_MS || 10000),
  pollMaxAttempts: Number(process.env.POLL_MAX_ATTEMPTS || 90),
};
