# Optional login startup

This package is independent of Wails. Public integration API:

```go
err := autostart.Set(true, absoluteExecutablePath) // explicit opt-in only
registered, err := autostart.Enabled()
err = autostart.Set(false, "") // idempotent; executable ignored
```

Importing the package, constructing a manager, reading `Enabled`, and disabling a
missing registration do not create configuration directories or registry keys.
Do not call `Set` automatically from application initialization. The UI should
surface errors rather than persisting a preference that disagrees with the OS.

- Linux: `$XDG_CONFIG_HOME/autostart/io.picofarm.desktop.desktop`, falling back
  to the OS user config directory. Exec is one quoted argument with desktop
  entry backslash and percent escaping; no shell is started.
- macOS: `~/Library/LaunchAgents/io.picofarm.desktop.plist`, with XML-escaped
  `ProgramArguments` and `RunAtLoad`. Changes apply at the next login; this does
  not bootstrap/unload a running launchd job or terminate a running app.
- Windows: current-user `Software\Microsoft\Windows\CurrentVersion\Run`,
  value `PicofarmDesktop`, containing a quoted executable path. A matching
  ownership record is kept under `Software\Picofarm\Autostart`. No batch file,
  shell command, machine-wide registry setting, or elevation is used.

Enabling requires an existing regular file at an absolute path without control
characters. It does not verify executable permissions, signatures, or packaging
compatibility. Use the installed executable, not a transient development build.
An application bundle on macOS needs the executable inside `Contents/MacOS`, not
the `.app` directory itself.

File registrations have an ownership marker. Symlink entries and existing
unmarked entries are never overwritten or removed. Registry mutations require
an exact match between the Run value and its ownership record; unmatched values
are preserved and return `ErrNotOwned`. Ownership markers are collision guards,
not a security boundary against another process running as the same user.

`Enabled` reports registration, not OS policy or successful launch. External
changes such as a disabled entry in Windows Startup Apps or desktop environment
policy may prevent launch while registration remains present. File content is
not fully revalidated by `Enabled`; manually modifying marked files is unsupported.

`NewFileManager(platform, configDir)` and `NewRegistryManager(store)` provide
injection points. Tests use temporary directories and an in-memory registry;
they never call the native default manager. The process mutex serializes this
package's operations, but not other processes. Atomic file replacement limits
partial writes; it does not eliminate filesystem TOCTOU races or malicious
symlink ancestors. Registry ownership and Run updates are not transactional;
failed ownership writes attempt to roll back Run, and rollback failures are
returned. File operations are not fsynced for crash durability.
