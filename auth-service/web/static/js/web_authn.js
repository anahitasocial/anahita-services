/*
 * Passkey sign-in for the server-rendered login page.
 *
 * Vanilla, no build step. The SPA settings page should use
 * @simplewebauthn/browser instead — it already has a bundler, and the
 * registration ceremony has more edge cases than are worth hand-rolling
 * twice. This file only covers login.
 *
 * WebAuthn speaks ArrayBuffers; JSON does not. The server sends binary
 * fields base64url-encoded, so every one has to be decoded on the way
 * in and re-encoded on the way out. That conversion is most of what
 * this file does.
 *
 * ONLY ONE CEREMONY MAY BE IN FLIGHT AT A TIME.
 *
 * The browser rejects a second concurrent navigator.credentials.get()
 * with "A request is already pending." That is easy to trip here,
 * because conditional mediation (the autofill prompt) deliberately
 * stays pending from page load until the person interacts with the
 * email field — so it is *always* outstanding when the button is
 * clicked. Every path below therefore goes through startCeremony,
 * which aborts and drains whatever was running first.
 *
 * No-JS fallback: this script only ever hides or shows the passkey
 * button. The email/password form is untouched and works regardless.
 */
(function () {
  'use strict';

  var BEGIN_URL = '/oauth/login/passkey/begin';
  var FINISH_URL = '/oauth/login/passkey/finish';

  var button = document.getElementById('passkey-button');
  var section = document.getElementById('passkey-section');
  var errorBox = document.getElementById('passkey-error');
  var identifier = document.getElementById('identifier');

  if (!button || !section) return;

  // The ceremony currently in flight, and the controller that cancels
  // it. Both null when idle.
  var activeCeremony = null;
  var activeController = null;

  /* ---------- base64url <-> ArrayBuffer ---------- */

  function fromBase64Url(value) {
    var padded = value.replace(/-/g, '+').replace(/_/g, '/');
    while (padded.length % 4) padded += '=';
    var binary = atob(padded);
    var bytes = new Uint8Array(binary.length);
    for (var i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return bytes.buffer;
  }

  function toBase64Url(buffer) {
    var bytes = new Uint8Array(buffer);
    var binary = '';
    for (var i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }

  /* ---------- feature detection ---------- */

  // Four things have to be true: the API exists, we can cancel a
  // request, we're in a secure context (WebAuthn is unavailable over
  // plain HTTP except on localhost), and credentials.get is present.
  // If any fails the button never appears, and the person sees an
  // ordinary password form rather than a control that does nothing.
  function isSupported() {
    return !!(
      window.PublicKeyCredential &&
      navigator.credentials &&
      navigator.credentials.get &&
      window.AbortController &&
      window.isSecureContext
    );
  }

  function showError(message) {
    if (!errorBox) return;
    errorBox.textContent = message;
    errorBox.style.display = 'flex';
  }

  function clearError() {
    if (!errorBox) return;
    errorBox.textContent = '';
    errorBox.style.display = 'none';
  }

  function setBusy(busy) {
    button.disabled = busy;
    button.textContent = busy ? 'Waiting for your device…' : 'Sign in with a passkey';
  }

  function isAbort(err) {
    return !!err && (err.name === 'AbortError' || err.name === 'NotAllowedError');
  }

  /* ---------- ceremony scheduling ---------- */

  // Cancels the in-flight ceremony and resolves once it has fully
  // settled.
  //
  // Draining matters, not just aborting. Each ceremony calls /begin,
  // which writes a fresh challenge to the session — so if the old
  // ceremony's /begin response lands after the new one's, it
  // overwrites the challenge the new assertion will be signed
  // against, and verification fails server-side for no visible
  // reason. Waiting for the old promise to settle closes that window.
  function drainActive() {
    if (activeController) {
      activeController.abort();
      activeController = null;
    }
    var pending = activeCeremony;
    activeCeremony = null;
    return Promise.resolve(pending).catch(function () {});
  }

  function startCeremony(mediation) {
    return drainActive().then(function () {
      activeController = new AbortController();
      activeCeremony = signIn(mediation, activeController.signal);
      return activeCeremony;
    });
  }

  /* ---------- the ceremony ---------- */

  function signIn(mediation, signal) {
    var isConditional = mediation === 'conditional';

    clearError();
    if (!isConditional) setBusy(true);

    return fetch(BEGIN_URL, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      signal: signal
    })
      .then(function (res) {
        if (!res.ok) throw new Error('begin failed');
        return res.json();
      })
      .then(function (options) {
        var publicKey = options.publicKey;

        publicKey.challenge = fromBase64Url(publicKey.challenge);

        if (publicKey.allowCredentials) {
          publicKey.allowCredentials = publicKey.allowCredentials.map(function (c) {
            return Object.assign({}, c, { id: fromBase64Url(c.id) });
          });
        }

        var request = { publicKey: publicKey, signal: signal };
        if (mediation) request.mediation = mediation;

        return navigator.credentials.get(request);
      })
      .then(function (credential) {
        if (!credential) throw new Error('no credential');

        // Re-encode every binary field the server needs to verify.
        var payload = {
          id: credential.id,
          rawId: toBase64Url(credential.rawId),
          type: credential.type,
          response: {
            clientDataJSON: toBase64Url(credential.response.clientDataJSON),
            authenticatorData: toBase64Url(credential.response.authenticatorData),
            signature: toBase64Url(credential.response.signature),
            userHandle: credential.response.userHandle
              ? toBase64Url(credential.response.userHandle)
              : null
          },
          clientExtensionResults: credential.getClientExtensionResults
            ? credential.getClientExtensionResults()
            : {}
        };

        return fetch(FINISH_URL, {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
      })
      .then(function (res) {
        return res.json().then(function (body) {
          if (!res.ok) throw new Error(body.message || 'Sign-in failed.');
          return body;
        });
      })
      .then(function (body) {
        // The server returns a destination rather than a redirect,
        // because fetch would follow a 302 invisibly and leave the
        // person sitting on the login page.
        if (!body.redirect_to) throw new Error('Sign-in failed.');
        window.location.assign(body.redirect_to);
      })
      .catch(function (err) {
        if (!isConditional) setBusy(false);

        // Aborted, dismissed, or no passkey for this site. Not worth
        // surfacing — the person either cancelled deliberately or
        // never engaged, and the password form is right above.
        if (isAbort(err)) {
          if (!isConditional) rearmConditional();
          return;
        }

        if (isConditional) {
          // Conditional failures are invisible by nature. Log and
          // leave the button as the working path rather than showing
          // an error for something the person never triggered.
          if (window.console && console.debug) {
            console.debug('passkey autofill unavailable:', err);
          }
          return;
        }

        showError(
          err && err.message
            ? err.message
            : "That passkey wasn't accepted. Try again, or sign in with your password."
        );
        rearmConditional();
      });
  }

  /* ---------- conditional mediation (autofill) ---------- */

  var conditionalSupported = false;

  // Re-arm autofill after an explicit attempt ends without navigating.
  // Without this, one dismissed prompt disables the autofill path for
  // the rest of the page's life.
  function rearmConditional() {
    if (!conditionalSupported) return;
    startCeremony('conditional');
  }

  /* ---------- wire up ---------- */

  if (!isSupported()) return;

  // Reveal the button only once we know the browser can honour it.
  section.style.display = 'block';

  button.addEventListener('click', function () {
    startCeremony(null);
  });

  // Autofill UI, where available. The browser surfaces any passkey for
  // this site inside the email field, so someone who has one never has
  // to notice the button. Requires autocomplete="username webauthn" on
  // that input. Silently skipped where unsupported — this is an
  // enhancement, not the path.
  if (
    identifier &&
    typeof window.PublicKeyCredential.isConditionalMediationAvailable === 'function'
  ) {
    window.PublicKeyCredential.isConditionalMediationAvailable()
      .then(function (available) {
        if (!available) return;
        conditionalSupported = true;
        startCeremony('conditional');
      })
      .catch(function () {
        /* enhancement only — the button remains */
      });
  }
})();
