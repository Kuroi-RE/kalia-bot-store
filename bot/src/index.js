import { createBot } from './bot.js';
import { config } from './config.js';

const bot = createBot();

// Register the command menu shown in Telegram clients.
bot.telegram
  .setMyCommands([
    { command: 'start', description: 'Open the store' },
  ])
  .catch(() => {});

bot.launch(() => {
    console.log('Kalia Store bot started (long polling).');
    console.log(`Backend: ${config.backendURL}`);
})

// Graceful shutdown.
process.once('SIGINT', () => bot.stop('SIGINT'));
process.once('SIGTERM', () => bot.stop('SIGTERM'));
