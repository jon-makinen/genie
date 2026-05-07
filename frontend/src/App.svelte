<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import {
    GetConfig,
    SetConfig,
    SetHotkey,
    GetPermissions,
    RequestMicrophone,
    PromptAccessibility,
    OpenAccessibilityPrefs,
    OpenMicrophonePrefs,
    ResetAccessibility,
    GetLaunchAtLogin,
    SetLaunchAtLogin,
    ListModels,
    DownloadModel,
    ModelProgress,
    CancelDownload,
    ToggleRecording,
    IsRecording,
    HideWindow,
    QuitApp,
  } from "../wailsjs/go/main/App.js";
  import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime.js";

  type ModelEntry = {
    filename: string;
    url: string;
    label: string;
    downloaded: boolean;
  };

  type Progress = {
    filename: string;
    downloaded: number;
    total: number;
    done: boolean;
    error?: string;
  };

  type Hotkey = {
    code: number;
    cmd: boolean;
    ctrl: boolean;
    shift: boolean;
    alt: boolean;
    label: string;
  };

  type Config = {
    model_name: string;
    magic_word: string;
    hotkey_enabled: boolean;
    hotkey: Hotkey;
    language: string;
  };

  type Permissions = { microphone: boolean; accessibility: boolean };

  type LaunchAtLogin = {
    enabled: boolean;
    supported: boolean;
    requires_approval: boolean;
  };

  let cfg: Config = {
    model_name: "ggml-large-v3-turbo-q5_0.bin",
    magic_word: "genie",
    hotkey_enabled: true,
    hotkey: { code: 0x0a, cmd: false, ctrl: false, shift: false, alt: false, label: "§" },
    language: "auto",
  };
  let perms: Permissions = { microphone: false, accessibility: false };
  let launch: LaunchAtLogin = { enabled: false, supported: true, requires_approval: false };
  let launchError = "";
  let models: ModelEntry[] = [];
  let progress: Progress = { filename: "", downloaded: 0, total: 0, done: false };
  let recording = false;
  let pollHandle: number | undefined;
  let errorMsg = "";

  $: hasModel = models.find((m) => m.filename === cfg.model_name)?.downloaded ?? false;

  async function refresh() {
    cfg = await GetConfig();
    perms = await GetPermissions();
    models = await ListModels();
    progress = await ModelProgress();
    recording = await IsRecording();
    launch = await GetLaunchAtLogin();
  }

  async function toggleLaunchAtLogin(e: Event) {
    const target = e.target as HTMLInputElement;
    launchError = "";
    try {
      launch = await SetLaunchAtLogin(target.checked);
    } catch (err: unknown) {
      launchError = String(err);
      launch = await GetLaunchAtLogin();
    }
  }

  onMount(async () => {
    await refresh();
    EventsOn("recording_state", (on: boolean) => (recording = on));
    EventsOn("model_progress", (p: Progress) => (progress = p));
    EventsOn("model_loaded", () => refresh());
    EventsOn("error", (msg: string) => (errorMsg = msg));
    pollHandle = window.setInterval(async () => {
      perms = await GetPermissions();
      if (progress.filename && !progress.done) {
        progress = await ModelProgress();
      }
    }, 1500);
  });

  onDestroy(() => {
    if (pollHandle !== undefined) clearInterval(pollHandle);
    EventsOff("recording_state");
    EventsOff("model_progress");
    EventsOff("model_loaded");
    EventsOff("error");
  });


  async function save() {
    cfg = await SetConfig(cfg);
  }

  // -- Hotkey capture --

  let capturing = false;
  let captureError = "";

  // Map a live keydown to a friendly label, e.g. ⌘⇧Space.
  function liveLabel(e: KeyboardEvent): string {
    let prefix = "";
    if (e.ctrlKey)  prefix += "⌃";
    if (e.altKey)   prefix += "⌥";
    if (e.shiftKey) prefix += "⇧";
    if (e.metaKey)  prefix += "⌘";
    return prefix + (e.code === "" ? "?" : prettyKeyName(e.code));
  }

  function prettyKeyName(code: string): string {
    if (code.startsWith("Key"))   return code.slice(3);
    if (code.startsWith("Digit")) return code.slice(5);
    if (code === "IntlBackslash") return "§";
    if (code === "Space")         return "Space";
    if (code === "Enter")         return "Return";
    if (code === "Escape")        return "Esc";
    if (code === "Tab")           return "Tab";
    if (code === "ArrowUp")       return "↑";
    if (code === "ArrowDown")     return "↓";
    if (code === "ArrowLeft")     return "←";
    if (code === "ArrowRight")    return "→";
    return code;
  }

  function startCapture() {
    capturing = true;
    captureError = "";
  }

  function cancelCapture() {
    capturing = false;
    captureError = "";
  }

  async function onCaptureKeydown(e: KeyboardEvent) {
    if (!capturing) return;
    e.preventDefault();
    e.stopPropagation();

    // Ignore lone modifier keypresses; wait for the actual key.
    if (
      e.code === "ShiftLeft" || e.code === "ShiftRight" ||
      e.code === "ControlLeft" || e.code === "ControlRight" ||
      e.code === "AltLeft" || e.code === "AltRight" ||
      e.code === "MetaLeft" || e.code === "MetaRight" ||
      e.code === "OSLeft" || e.code === "OSRight"
    ) return;

    if (e.code === "Escape") {
      cancelCapture();
      return;
    }

    try {
      const result = await SetHotkey(e.code, e.metaKey, e.ctrlKey, e.shiftKey, e.altKey);
      cfg.hotkey = result;
      capturing = false;
    } catch (err: unknown) {
      captureError = String(err);
    }
  }

  function fmtSize(bytes: number) {
    if (!bytes) return "—";
    const mb = bytes / (1024 * 1024);
    return mb >= 1024 ? `${(mb / 1024).toFixed(2)} GB` : `${mb.toFixed(0)} MB`;
  }

  function pct(p: Progress) {
    if (!p.total) return 0;
    return Math.min(100, Math.round((p.downloaded / p.total) * 100));
  }

  async function resetAxAccess() {
    errorMsg = "";
    try {
      await ResetAccessibility();
    } catch (err: unknown) {
      errorMsg = String(err);
    }
  }
</script>

<svelte:window on:keydown|capture={onCaptureKeydown} />

<main>
  <div class="title-block">
    <h1>Genie</h1>
    <div class="version">v0.1.0 · local whisper · macos</div>
  </div>

  <div class="intro">
    Press <span class="kbd">{cfg.hotkey.label || "§"}</span> or click the menu bar icon to record.
    Speech is transcribed locally and pasted into the focused input.
    Say <strong>"{cfg.magic_word}"</strong> to stop.
  </div>

  {#if errorMsg}
    <div class="banner">{errorMsg}</div>
  {/if}

  {#if !hasModel}
    <div class="banner">No whisper model installed — pick one below to download.</div>
  {/if}

  <h2>Status</h2>
  <div class="row">
    <span>
      {#if recording}
        <span class="recording-indicator"></span>
        <span class="label">REC</span>
        <div class="meta">listening · say "{cfg.magic_word}" to stop</div>
      {:else}
        <span class="label muted">Idle</span>
        <div class="meta">{hasModel ? "ready" : "model not loaded"}</div>
      {/if}
    </span>
    <button class="primary" on:click={ToggleRecording} disabled={!hasModel}>
      {recording ? "Stop" : "Start"}
    </button>
  </div>

  <h2>Permissions</h2>
  <div class="row">
    <span>
      <span class="dot {perms.microphone ? 'good' : 'bad'}"></span>
      <span class="label">Microphone</span>
      <div class="meta">required to capture audio</div>
    </span>
    {#if perms.microphone}
      <span class="dim">granted</span>
    {:else}
      <button on:click={() => { RequestMicrophone(); OpenMicrophonePrefs(); }}>Grant</button>
    {/if}
  </div>
  <div class="row">
    <span>
      <span class="dot {perms.accessibility ? 'good' : 'bad'}"></span>
      <span class="label">Accessibility</span>
      <div class="meta">
        global § hotkey + synthetic Cmd+V.
        {#if !perms.accessibility}
          <br/>
          if Genie is already toggled on in System Settings but this still says
          off, click <strong>Reset access</strong> — that's a stale TCC entry
          from a previous build.
        {/if}
      </div>
    </span>
    <span style="display: flex; gap: 6px; flex-shrink: 0;">
      {#if perms.accessibility}
        <span class="dim">granted</span>
      {:else}
        <button on:click={() => { PromptAccessibility(); OpenAccessibilityPrefs(); }}>Grant</button>
      {/if}
      <button on:click={resetAxAccess}>Reset access</button>
    </span>
  </div>

  <h2>Model</h2>
  {#each models as m}
    <div class="row">
      <span>
        <span class="dot {m.downloaded ? 'good' : 'bad'}"></span>
        <span class="label">{m.label}</span>
        <div class="meta">{m.filename}</div>
        {#if progress.filename === m.filename && !progress.done}
          <div class="progress"><div style="width: {pct(progress)}%"></div></div>
          <div class="meta">{fmtSize(progress.downloaded)} / {fmtSize(progress.total)} · {pct(progress)}%</div>
        {/if}
      </span>
      <span style="display: flex; gap: 6px; flex-shrink: 0;">
        {#if m.filename === cfg.model_name}
          <span class="dim">active</span>
        {:else}
          <button on:click={() => { cfg.model_name = m.filename; save(); }} disabled={!m.downloaded}>Use</button>
        {/if}
        {#if m.downloaded}
          <span class="dim">installed</span>
        {:else if progress.filename === m.filename && !progress.done}
          <button on:click={CancelDownload}>Cancel</button>
        {:else}
          <button class="primary" on:click={() => DownloadModel(m.filename)}>Download</button>
        {/if}
      </span>
    </div>
  {/each}

  <h2>Behavior</h2>
  <div class="row">
    <span>
      <span class="label">Magic word</span>
      <div class="meta">say this to stop recording</div>
    </span>
    <input type="text" bind:value={cfg.magic_word} on:change={save} />
  </div>
  <div class="row">
    <span>
      <span class="label">Language</span>
      <div class="meta">auto-detect or pin a primary language</div>
    </span>
    <select bind:value={cfg.language} on:change={save}>
      <option value="auto">Auto-detect</option>
      <option value="en">English</option>
      <option value="fi">Finnish</option>
      <option value="sv">Swedish</option>
      <option value="de">German</option>
      <option value="fr">French</option>
      <option value="es">Spanish</option>
    </select>
  </div>
  <div class="row">
    <span>
      <span class="label">Global hotkey</span>
      <div class="meta">
        {#if capturing}
          press any combination · <span class="dim">esc to cancel</span>
        {:else}
          press from anywhere to toggle recording
        {/if}
        {#if captureError}<div class="banner" style="margin-top: 6px">{captureError}</div>{/if}
      </div>
    </span>
    <span style="display: flex; gap: 6px; align-items: center;">
      <span class="kbd">{capturing ? "…" : (cfg.hotkey.label || "—")}</span>
      {#if capturing}
        <button on:click={cancelCapture}>Cancel</button>
      {:else}
        <button on:click={startCapture}>Change</button>
      {/if}
    </span>
  </div>
  <div class="row">
    <span>
      <span class="label">Hotkey enabled</span>
      <div class="meta">turn off to ignore the global shortcut while keeping it set</div>
    </span>
    <label class="toggle">
      <input type="checkbox" bind:checked={cfg.hotkey_enabled} on:change={save} />
      <span class="switch"></span>
    </label>
  </div>
  <div class="row">
    <span>
      <span class="label">Launch at login</span>
      <div class="meta">
        {#if !launch.supported}
          requires macOS 13 or newer
        {:else if launch.requires_approval}
          approve in System Settings → General → Login Items to enable
        {:else}
          start Genie automatically when you log in
        {/if}
        {#if launchError}<div class="banner" style="margin-top: 6px">{launchError}</div>{/if}
      </div>
    </span>
    <label class="toggle">
      <input
        type="checkbox"
        checked={launch.enabled}
        disabled={!launch.supported}
        on:change={toggleLaunchAtLogin}
      />
      <span class="switch"></span>
    </label>
  </div>

  <div class="actions">
    <button on:click={HideWindow}>Close</button>
    <button on:click={QuitApp}>Quit</button>
  </div>
</main>
