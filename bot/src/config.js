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

  // Styled QR: composite the dynamic QRIS onto the branded template frame.
  qrStyled: (process.env.QR_STYLED || 'true') !== 'false',
  qrTemplatePath: process.env.QR_TEMPLATE_PATH || '',
  qrLeft: Number(process.env.QR_LEFT || 338),
  qrTop: Number(process.env.QR_TOP || 340),
  qrSize: Number(process.env.QR_SIZE || 720),
  qrRotate: Number(process.env.QR_ROTATE || 0),
  // Final image size/quality — smaller = faster to send. Note: downscaling the
  // composite can merge the QR modules and break scanning, so keep the width at
  // the template size (1280) and reduce size via JPEG quality instead.
  qrOutputWidth: Number(process.env.QR_OUTPUT_WIDTH || 1280),
  qrQuality: Number(process.env.QR_QUALITY || 62),
};
