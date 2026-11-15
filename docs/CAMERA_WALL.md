# Camera wall

The Cameras page (`web/src/pages/Cameras.tsx`) is a browser-side monitoring view, not a recording service or a printer-control workflow. It reads the existing cameras/printers/state APIs; it does not start or pause physical prints.

## Layout and organization

The wall presents cameras in a single responsive grid. Card size can be automatic, small, medium or large. Users can select visible cameras, set favorites, reorder through accessible buttons, and filter by printer, printing status or favorites. Layout preferences are local to this browser, not synchronized with the server or other devices. Reset restores defaults. Blocked/unavailable browser storage must not prevent viewing cameras.

A focus layout emphasizes one camera without rendering a second copy of its stream. Individual enlargement uses a modal; mural mode hides the application chrome. Escape exits the active enlarged/mural view. Mobile layouts must remain single-column when necessary rather than force horizontal scrolling.

## Stream lifecycle and limits

- Enabled MJPEG and snapshot cameras can be previewed inline. RTSP/WebRTC entries do not acquire native playback merely by appearing in the wall.
- Pausing viewing is local and does not pause a print or disable the camera on the printer. Hidden, disabled and paused previews must detach their image source rather than leave an invisible stream connected.
- Errors are isolated per camera, with explicit retry rather than an unlimited reconnect loop.
- A snapshot is a still image, not proof of a current live feed. MJPEG image events do not establish frame-by-frame freshness; a frozen source may remain visually indistinguishable from a static scene.
- Smaller cards do not reduce the upstream MJPEG bitrate. Multiple streams can consume substantial bandwidth, decode resources and browser connections. Pause unnecessary cameras. Browser/server connection limits may prevent all streams from loading concurrently.
- Camera URLs are still fetched by the browser. Mixed-content rules, TLS, camera authentication and network reachability still apply. Do not put private URLs or credentials in shared screenshots. Layout storage must contain camera IDs/preferences only, not stream URLs.

## Validation

Run frontend lint/build and `cd web && node --test tests/camera-wall.test.mjs`. For the browser smoke, serve the frontend (for example `npm run preview -- --host 127.0.0.1 --port 3197` after building), install Playwright separately, then run `PLAYWRIGHT_MODULE=/PATH/TO/playwright BASE_URL=http://127.0.0.1:3197 node web/tests/camera-wall-browser-smoke.cjs` from the repository root. Set `CHROMIUM_EXECUTABLE` if Chromium is not at `/usr/bin/chromium`. The test intercepts API calls and uses a loopback multipart image server to check connection teardown; it never contacts real printers. Synthetic browser checks validate UI behavior, not physical cameras, network stability or freshness. Measure 1/4/9 real cameras separately: active connections, bandwidth, browser CPU/RSS, time to first image, recovery after source loss, and whether pausing closes the upstream connection. No tested real-camera capacity is established by a successful synthetic smoke.
