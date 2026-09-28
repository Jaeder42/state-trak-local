// Fullscreen toggle for the F11 shortcut, working in both app modes:
// - Wails desktop app: window.runtime is injected by the webview — toggle
//   the native window fullscreen (same as the mac ⤢ button; users can
//   click that too)
// - plain browser / standalone server: the DOM Fullscreen API instead.
// There is no WindowToggleFullscreen in the wails v2 runtime, so query
// first (a promise) and then call the matching enter/exit function.

export const toggleFullscreen = async () => {
  const rt = window.runtime;
  if (rt?.WindowFullscreen && rt?.WindowUnfullscreen && rt?.WindowIsFullscreen) {
    const fullscreen = await rt.WindowIsFullscreen();
    if (fullscreen) {
      rt.WindowUnfullscreen();
    } else {
      rt.WindowFullscreen();
    }
    return;
  }
  if (document.fullscreenElement) {
    await document.exitFullscreen();
  } else {
    await document.documentElement.requestFullscreen();
  }
};