/**
 * Virtual WebAuthn authenticator helper.
 *
 * Enables a CDP-backed virtual authenticator on a Playwright BrowserContext so
 * passkey registration / authentication ceremonies complete without real
 * platform UI prompts. Returns the CDPSession so callers can issue further
 * `WebAuthn.*` commands (list credentials, remove credential, etc.).
 *
 * MUST be called BEFORE the page navigates to any URL that triggers a
 * WebAuthn ceremony — Chromium binds the authenticator to the context only
 * after `WebAuthn.enable` + `addVirtualAuthenticator` succeed.
 */
import { BrowserContext, CDPSession } from '@playwright/test'

export type VirtualAuthenticatorOptions = {
  /** ctap2 (default) or u2f. */
  protocol?: 'ctap2' | 'u2f'
  /** internal (platform) or usb / nfc / ble (roaming). Defaults to internal. */
  transport?: 'usb' | 'nfc' | 'ble' | 'internal'
  /** Resident keys (a.k.a. discoverable credentials). Defaults to true. */
  hasResidentKey?: boolean
  /** User verification (PIN/biometric) capability. Defaults to true. */
  hasUserVerification?: boolean
  /** Whether the virtual authenticator auto-passes UV. Defaults to true. */
  isUserVerified?: boolean
}

export type VirtualAuthenticatorHandle = {
  session: CDPSession
  authenticatorId: string
}

export async function enableVirtualAuthenticator(
  context: BrowserContext,
  options: VirtualAuthenticatorOptions = {},
): Promise<VirtualAuthenticatorHandle> {
  // CDP sessions attach to a Page, so we use a throw-away page for the
  // session itself. The authenticator is registered on the browser context's
  // device — any page in the same context will see it.
  const page = await context.newPage()
  const session = await context.newCDPSession(page)
  await session.send('WebAuthn.enable')
  const { authenticatorId } = (await session.send('WebAuthn.addVirtualAuthenticator', {
    options: {
      protocol: options.protocol ?? 'ctap2',
      transport: options.transport ?? 'internal',
      hasResidentKey: options.hasResidentKey ?? true,
      hasUserVerification: options.hasUserVerification ?? true,
      isUserVerified: options.isUserVerified ?? true,
      automaticPresenceSimulation: true,
    },
  } as Parameters<CDPSession['send']>[1])) as { authenticatorId: string }
  await page.close()
  return { session, authenticatorId }
}

/**
 * Tear down the virtual authenticator at the end of a test so credentials
 * don't leak between specs that share the same browser context.
 */
export async function removeVirtualAuthenticator(
  handle: VirtualAuthenticatorHandle,
): Promise<void> {
  try {
    await handle.session.send('WebAuthn.removeVirtualAuthenticator', {
      authenticatorId: handle.authenticatorId,
    } as Parameters<CDPSession['send']>[1])
  } catch {
    // best-effort: context may already be closed.
  }
  try {
    await handle.session.detach()
  } catch {
    // already detached.
  }
}
