// YourMove Playwright bridge: a deliberately narrow automation surface.
//
// The Go process owns authorization, posting ownership, session lifecycle,
// and outcome reporting. This driver only launches one visible managed
// browser, evaluates authorized page scripts, sets user-approved files on
// identified inputs, and reports observed results. It never navigates,
// never submits, and never reads values back into anything but the
// operation's own return value.
//
// Protocol: newline-delimited JSON over stdio.
// Request:  {"id":1,"op":"launch","engine":"firefox","url":"...",
//            "profileDir":"...","headless":false}
// Response: {"id":1,"ok":true,"value":{...}}
//           {"id":1,"ok":false,"error":{"kind":"closed","message":"..."}}
// Error kinds: launch, closed, timeout, crash, eval, protocol.
import { createRequire } from 'module';
import readline from 'readline';

const require = createRequire(import.meta.url);
const { chromium, firefox, webkit } = require('playwright-core');

const ENGINES = { chromium, firefox, webkit };

let context = null;
let dead = false;
let lastPage = null;

function currentPage() {
  if (!context) return null;
  const pages = context.pages().filter((p) => !p.isClosed());
  if (pages.length === 0) return null;
  if (lastPage && !lastPage.isClosed() && pages.includes(lastPage)) return lastPage;
  lastPage = pages[pages.length - 1];
  return lastPage;
}

function fail(message, kind = 'eval') {
  return { ok: false, error: { kind, message: String(message ?? 'unknown error').slice(0, 2000) } };
}

function classifyError(err) {
  const message = String(err?.message ?? err).slice(0, 2000);
  if (dead) return { kind: 'closed', message };
  if (/closed|crash|disconnected|has been closed/i.test(message)) return { kind: 'closed', message };
  if (/timeout|exceeded/i.test(message)) return { kind: 'timeout', message };
  return { kind: 'eval', message };
}

async function handle(req) {
  const { op } = req;
  try {
    switch (op) {
      case 'launch': {
        if (context) return fail('browser already launched', 'launch');
        const engine = ENGINES[req.engine];
        if (!engine) return fail(`unsupported engine: ${req.engine}`, 'launch');
        try {
          context = await engine.launchPersistentContext(req.profileDir || '', {
            headless: req.headless === true,
            viewport: { width: 1150, height: 850 },
            acceptDownloads: false,
            timeout: 20000,
          });
        } catch (err) {
          return { ok: false, error: { kind: 'launch', message: String(err?.message ?? err).slice(0, 2000) } };
        }
        context.on('page', (p) => {
          lastPage = p;
        });
        const browser = context.browser();
        if (browser) {
          browser.on('disconnected', () => {
            dead = true;
          });
        }
        const page = context.pages()[0] || (await context.newPage());
        lastPage = page;
        try {
          await page.goto(req.url, { waitUntil: 'domcontentloaded', timeout: 30000 });
        } catch (err) {
          return { ok: false, error: { kind: 'launch', message: `open application: ${err?.message ?? err}`.slice(0, 2000) } };
        }
        return { ok: true, value: { engine: req.engine, version: browser ? browser.version() : 'unknown' } };
      }
      case 'evaluate': {
        const page = currentPage();
        if (!page) return { ok: false, error: { kind: 'closed', message: 'the managed browser has no open pages' } };
        try {
          // Invoke a caller-supplied function source with JSON arguments.
          // The call is composed by plain concatenation, so the function
          // source is never re-parsed as a template and needs no escaping.
          // It goes through Playwright's debugger-privileged evaluation
          // (like CDP Runtime.evaluate), so page content-security policies
          // do not block it and no in-page eval is needed. Callers always
          // send a function source; immediately invoked expressions are
          // wrapped Go-side so trailing semicolons cannot break parsing.
          const src = String(req.expression ?? '');
          const argJson = JSON.stringify(Array.isArray(req.args) ? req.args : []);
          const value = await page.evaluate('(' + src + ')(...' + argJson + ')', { timeout: 12000 });
          return { ok: true, value: value ?? null };
        } catch (err) {
          const c = classifyError(err);
          return { ok: false, error: c };
        }
      }
      case 'pageUrl': {
        const page = currentPage();
        if (!page) return { ok: false, error: { kind: 'closed', message: 'the managed browser has no open pages' } };
        try {
          return { ok: true, value: { url: page.url() } };
        } catch (err) {
          const c = classifyError(err);
          return { ok: false, error: c };
        }
      }
      case 'tag': {
        const page = currentPage();
        if (!page) return { ok: false, error: { kind: 'closed', message: 'the managed browser has no open pages' } };
        try {
          const token = `ym${Date.now().toString(36)}${Math.floor(Math.random() * 1e6).toString(36)}`;
          const found = await page.evaluate(
            ([index, attr, tok]) => {
              const el = document.querySelectorAll('input[type="file"]')[index];
              if (!el) return false;
              el.setAttribute(attr, tok);
              return true;
            },
            [req.index, 'data-yourmove-upload', token],
          );
          if (!found) return fail('file input disappeared before selection', 'eval');
          return { ok: true, value: { token } };
        } catch (err) {
          const c = classifyError(err);
          return { ok: false, error: c };
        }
      }
      case 'setFiles': {
        const page = currentPage();
        if (!page) return { ok: false, error: { kind: 'closed', message: 'the managed browser has no open pages' } };
        try {
          await page.locator(`[data-yourmove-upload="${req.token}"]`).setInputFiles(req.paths, { timeout: 12000 });
          return { ok: true, value: { selected: true } };
        } catch (err) {
          const c = classifyError(err);
          return { ok: false, error: c };
        }
      }
      case 'close': {
        try {
          await context?.close();
        } catch {
          // Closing a dead browser is the expected path, not a failure.
        }
        context = null;
        return { ok: true, value: { closed: true } };
      }
      default:
        return fail(`unknown op: ${op}`, 'protocol');
    }
  } catch (err) {
    const c = classifyError(err);
    return { ok: false, error: c };
  }
}

const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
rl.on('line', async (line) => {
  if (!line.trim()) return;
  let req;
  try {
    req = JSON.parse(line);
  } catch {
    process.stdout.write(JSON.stringify({ id: 0, ...fail('malformed request', 'protocol') }) + '\n');
    return;
  }
  const result = await handle(req);
  process.stdout.write(JSON.stringify({ id: req.id, ...result }) + '\n');
  if (req.op === 'close') {
    process.exit(0);
  }
});
