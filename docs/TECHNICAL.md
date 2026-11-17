# CoreGUI — technical details

This page is for people who want to know exactly what CoreGUI does to their system.
For normal use, see the [README](../README.md).

- [How it works, bit by bit](#how-it-works-bit-by-bit)
- [Same class of tool as VibranceGUI](#same-class-of-tool-as-vibrancegui)
- [Check it yourself](#check-it-yourself)
- [Performance](#performance)
- [Build from source](#build-from-source)
- [Releases](#releases)
- [Code layout](#code-layout)

## How it works, bit by bit

CoreGUI does **exactly what Task Manager's "Set affinity" does**, on a timer. It talks only to the
operating system's process scheduler. It never opens the game's memory, never injects code and never
touches game files.

### 1. The affinity mask

The OS stores, for every process, a bitmask of the logical CPUs it may run on: bit *N* set = CPU *N*
allowed. CoreGUI reads the system's mask and clears bit 0 (or bits 0 and 1):

```
16 logical CPUs, system mask          0xFFFF = 1111 1111 1111 1111
"Core 0"       clears 0x0001  →        0xFFFE = 1111 1111 1111 1110   (CS2 uses CPUs 1-15)
"Core 0 and 1" clears 0x0003  →        0xFFFC = 1111 1111 1111 1100   (CS2 uses CPUs 2-15)

target = system_mask AND NOT excluded
```

The system mask is detected at run time, so the result fits any CPU (4, 8, 16, 32... threads, up to 64).

### 2. Windows: every call made to the game's process

Every 3 seconds:

| Step | Win32 function | What it touches |
|---|---|---|
| Find CS2 (only while not found yet) | `CreateToolhelp32Snapshot` + `Process32First/Next` | The OS process list (names and PIDs). No handle to CS2 is opened. |
| Still the same CS2? | `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` → `GetExitCodeProcess`, `QueryFullProcessImageNameW` | Whether the PID is alive and its `.exe` name, both kept by the kernel. |
| Read affinity | `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` → `GetProcessAffinityMask` | The scheduler's mask for the process. |
| Set affinity (only if different) | `OpenProcess(PROCESS_SET_INFORMATION \| PROCESS_QUERY_LIMITED_INFORMATION)` → `SetProcessAffinityMask` | The scheduler's mask for the process. |
| Verify | `GetProcessAffinityMask` again | Confirms the OS accepted it. |
| Done | `CloseHandle` | Each handle is closed right away. |

The handle used to change the affinity is opened with access mask **`0x00001200`** and nothing else:

```
0x00001200 = 0001 0010 0000 0000
                │   └─ bit 9  PROCESS_SET_INFORMATION            (0x0200)  change scheduling settings
                └───── bit 12 PROCESS_QUERY_LIMITED_INFORMATION  (0x1000)  read basic info

Never requested:
  bit 1  PROCESS_CREATE_THREAD  (0x0002)   ← required to inject code
  bit 3  PROCESS_VM_OPERATION   (0x0008)   ← required to change game memory
  bit 4  PROCESS_VM_READ        (0x0010)   ← required to read game memory
  bit 5  PROCESS_VM_WRITE       (0x0020)   ← required to write game memory
```

Because those bits are off, **Windows itself refuses** any attempt to read or write CS2's memory through
CoreGUI's handles. CoreGUI never calls `ReadProcessMemory`, `WriteProcessMemory`, `VirtualAllocEx`,
`CreateRemoteThread`, `LoadLibrary` inside the game, hooks or drivers. It runs as a normal user
(`asInvoker` manifest: no admin prompt).

Everything else CoreGUI calls concerns **its own** window and settings: `user32`/`gdi32`/`dwmapi`/`uxtheme`
for the window, tray icon and menus, a named mutex (one instance only), the per-user registry key
`HKCU\Software\Microsoft\Windows\CurrentVersion\Run` (only if you turn on *Start with system*), and a
small settings file in `%APPDATA%\coregui\config`. No network access.

### 3. Linux

| Step | Call | What it touches |
|---|---|---|
| Find CS2 | read `/proc/<pid>/comm` | The process name (15 chars), kept by the kernel. |
| List threads | read the directory `/proc/<pid>/task` | Thread IDs (Linux affinity is per thread). |
| Set / read affinity | `sched_setaffinity` / `sched_getaffinity` on each thread | The scheduler's CPU mask. |

Never used: `/proc/<pid>/mem`, `ptrace`, `process_vm_readv`, `process_vm_writev`.

## Same class of tool as VibranceGUI

| | VibranceGUI | CoreGUI |
|---|---|---|
| Detects the game by | its process (when its window is in focus) | its process name |
| Changes | display vibrance, through the GPU driver's public API (NVIDIA NVAPI / AMD ADL) | CPU affinity, through the OS's public API (`SetProcessAffinityMask` / `sched_setaffinity`) |
| Reads or writes game memory | No | No |
| Injects code or hooks the game | No | No |
| Same effect done by hand with | NVIDIA / AMD control panel | Task Manager → Details → Set affinity |

Both are external tools that change a **system** setting while the game runs. Neither interacts with
the game itself.

## Check it yourself

Every call to the game's process is in [`platform_windows.go`](../platform_windows.go) and
[`platform_linux.go`](../platform_linux.go). To confirm there is no memory access anywhere in the code:

```sh
grep -rnE "ReadProcessMemory|WriteProcessMemory|VirtualAllocEx|CreateRemoteThread|VM_READ|VM_WRITE|ptrace|process_vm_|/mem\b" \
  --include=*.go . | grep -vE '^[^:]+:[0-9]+:\s*//'
# no output: the only matches are comments stating these are NOT used
```

Release binaries are built by GitHub Actions from the tagged source ([workflow](../.github/workflows/release.yml)),
and each release includes `SHA256SUMS.txt` so you can verify your download:

```powershell
Get-FileHash .\coregui.exe -Algorithm SHA256
```

## Performance

Measured with [`scripts/bench.ps1`](../scripts/bench.ps1) on a Ryzen 7 9800X3D (16 threads), Windows 11,
app in the tray:

| | CPU | RAM (working set) |
|---|---|---|
| Waiting for CS2 | 0.07% of one core (≈40 ms of CPU per minute) | 12.6 MB |
| CS2 open (verifying every 3 s) | 0.01% of one core (≈4 ms of CPU per minute) | 12.8 MB |

- CPU is measured in exact CPU cycles (`QueryProcessCycleTime`); Windows' usual CPU-time counter has
  ~15 ms resolution and reports 0 for a process this quiet.
- While CS2 is open, CoreGUI only re-checks the known process instead of scanning the process list, so
  it is cheapest exactly while you play.
- The Windows build has no GUI framework: the window is drawn with plain Win32/GDI, and the tray icon
  uses `Shell_NotifyIconW` directly.

Run `pwsh scripts\bench.ps1` (after building `dist\coregui.exe`) to reproduce it. It refuses to run while
CS2 is open and only stops the processes it started.

## Build from source

Requires [Go](https://go.dev) 1.26+.

```sh
go test ./...
go build -trimpath -ldflags "-H windowsgui -s -w" -o dist/coregui.exe .                 # Windows
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o dist/coregui-linux-amd64 .  # Linux
```

`go generate` redraws the icon and the Windows resource (icon + manifest) with no external tools.

## Releases

Pushing a tag `vX.Y.Z` makes GitHub Actions vet, test and build both binaries, then publish a GitHub
Release with `coregui.exe`, `coregui-linux-amd64` and `SHA256SUMS.txt`.

```sh
git tag v1.0.0
git push origin v1.0.0
```

## Code layout

| Path | What it does |
|---|---|
| `main.go` | Entry point |
| `enforcer.go` | Detection loop: compute target mask, apply, verify |
| `cpuset.go` | CPU mask helpers (pure, tested) |
| `platform_windows.go`, `platform_linux.go` | All OS calls: process list, affinity, single instance, autostart |
| `window_windows.go` | Settings window (native Win32, custom-drawn dark UI) |
| `tray_windows.go`, `tray_linux.go` | Tray icon and menu |
| `ui.go`, `i18n.go` | Status texts and the 10 built-in languages |
| `config.go` | Saved settings |
| `internal/canvas`, `internal/icon` | Anti-aliased drawing and the app icon |
| `tools/genicon` | Generates `assets/icon.png` and the Windows `.syso` resource |
