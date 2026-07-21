import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import QRCode from 'qrcode';
import sharp from 'sharp';
import { config } from './config.js';

// The branded "KAL'S STORE / SCAN HERE" frame the dynamic QR is dropped into.
const here = path.dirname(fileURLToPath(import.meta.url));
const templatePath = config.qrTemplatePath || path.join(here, 'assets', 'qr_template.jpeg');

let templateBuf = null;
function template() {
  if (!templateBuf) templateBuf = readFileSync(templatePath);
  return templateBuf;
}

const DARK = '#4a4a4a'; // module colour, matching the template's soft charcoal

// buildQrSvg renders the QR payload as an SVG: data modules as rounded dots and
// the three position markers as rounded squares, on a white card with a faint
// graph-paper grid — mirroring the template's aesthetic.
function buildQrSvg(payload, sizePx) {
  const qr = QRCode.create(payload, { errorCorrectionLevel: 'M' });
  const n = qr.modules.size;
  const data = qr.modules.data; // length n*n, 1 = dark module
  const margin = 4; // quiet zone (modules) — required for reliable scanning
  const dim = n + margin * 2;

  const inFinder = (r, c) =>
    (r < 7 && c < 7) || (r < 7 && c >= n - 7) || (r >= n - 7 && c < 7);

  let dots = '';
  for (let r = 0; r < n; r++) {
    for (let c = 0; c < n; c++) {
      if (!data[r * n + c] || inFinder(r, c)) continue;
      // Rounded squares with high module coverage: soft "dotted" look that
      // still decodes reliably (unlike small circles).
      dots += `<rect x="${(c + margin + 0.04).toFixed(3)}" y="${(r + margin + 0.04).toFixed(3)}" width="0.92" height="0.92" rx="0.34" ry="0.34"/>`;
    }
  }

  const finder = (fr, fc) => {
    const x = fc + margin;
    const y = fr + margin;
    return (
      `<rect x="${x}" y="${y}" width="7" height="7" rx="2.2" ry="2.2" fill="${DARK}"/>` +
      `<rect x="${x + 1}" y="${y + 1}" width="5" height="5" rx="1.6" ry="1.6" fill="#ffffff"/>` +
      `<rect x="${x + 2}" y="${y + 2}" width="3" height="3" rx="1" ry="1" fill="${DARK}"/>`
    );
  };
  const finders = finder(0, 0) + finder(0, n - 7) + finder(n - 7, 0);

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${sizePx}" height="${sizePx}" viewBox="0 0 ${dim} ${dim}">
  <rect x="0" y="0" width="${dim}" height="${dim}" rx="2.5" ry="2.5" fill="#ffffff"/>
  <g fill="${DARK}">${dots}</g>
  ${finders}
</svg>`;
}

// renderStyledQR returns a JPEG buffer of the branded frame with the given
// QRIS payload rendered into it. Throws if compositing fails (caller falls
// back to a plain QR).
export async function renderStyledQR(payload) {
  const svg = buildQrSvg(payload, config.qrSize);
  let qrPng = await sharp(Buffer.from(svg)).png().toBuffer();

  let { left, top } = { left: config.qrLeft, top: config.qrTop };
  if (config.qrRotate) {
    const before = await sharp(qrPng).metadata();
    qrPng = await sharp(qrPng)
      .rotate(config.qrRotate, { background: { r: 0, g: 0, b: 0, alpha: 0 } })
      .png()
      .toBuffer();
    const after = await sharp(qrPng).metadata();
    // Keep the QR centred on the same spot after rotation grows the canvas.
    left = Math.round(left - (after.width - before.width) / 2);
    top = Math.round(top - (after.height - before.height) / 2);
  }

  return sharp(template())
    .composite([{ input: qrPng, left, top }])
    .resize({ width: config.qrOutputWidth, withoutEnlargement: true })
    .jpeg({ quality: config.qrQuality, mozjpeg: true })
    .toBuffer();
}
