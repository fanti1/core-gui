# Measures CoreGUI's CPU and memory use on Windows.
#
#   1. Idle:   app in the tray, CS2 not running.
#   2. Active: a fake "cs2.exe" (a sleeping Go program) is running, so the app
#              applies the affinity and re-verifies it every 3 seconds.
#
# CPU is measured in exact CPU cycles (QueryProcessCycleTime): Windows' normal
# CPU-time counter has ~15 ms resolution and reports 0 for a process this quiet.
#
# Usage (from the repository root, after building dist\coregui.exe):
#   pwsh scripts\bench.ps1 [-Seconds 60]
#
# Refuses to run while the real CS2 is open. Only stops processes it started.

param([int]$Seconds = 60)
$ErrorActionPreference = 'Stop'

$exe = Join-Path $PSScriptRoot '..\dist\coregui.exe' | Resolve-Path
if (Get-Process cs2 -ErrorAction SilentlyContinue) { throw 'CS2 is running. Close it before benchmarking.' }
if (Get-Process coregui -ErrorAction SilentlyContinue) { throw 'CoreGUI is already running. Exit it first.' }

Add-Type @'
using System; using System.Runtime.InteropServices;
public static class Cycles {
  [DllImport("kernel32.dll")] static extern bool QueryProcessCycleTime(IntPtr h, out ulong c);
  public static ulong Of(IntPtr h) { ulong c; QueryProcessCycleTime(h, out c); return c; }
}
'@

# Build the fake cs2.exe in a temp folder.
$tmp = Join-Path ([IO.Path]::GetTempPath()) "coregui-bench-$(Get-Random)"
New-Item -ItemType Directory $tmp | Out-Null
Set-Content "$tmp\go.mod" 'module fakecs2'
Set-Content "$tmp\main.go" "package main`nimport `"time`"`nfunc main() { time.Sleep(time.Hour) }"
& go build -C $tmp -o "$tmp\cs2.exe" . | Out-Null

$cpus = [Environment]::ProcessorCount
$hz = (Get-CimInstance Win32_Processor | Select-Object -First 1).MaxClockSpeed * 1e6

function Measure-Phase([System.Diagnostics.Process]$p, [string]$name) {
    $c0 = [Cycles]::Of($p.Handle)
    $t0 = [Diagnostics.Stopwatch]::StartNew()
    $ws = @(); $priv = @()
    for ($i = 0; $i -lt $Seconds; $i++) {
        Start-Sleep -Seconds 1
        $p.Refresh()
        if ($p.HasExited) { throw "CoreGUI exited during '$name' (exit code $($p.ExitCode))." }
        $ws += $p.WorkingSet64; $priv += $p.PrivateMemorySize64
    }
    $cycles = [Cycles]::Of($p.Handle) - $c0
    $secs = $t0.Elapsed.TotalSeconds
    $cpuMs = 1000 * $cycles / $hz   # CPU time estimated at the CPU's base clock
    [pscustomobject]@{
        Phase               = $name
        'Cycles/s'          = '{0:N0}' -f ($cycles / $secs)
        'CPU ms/min'        = [math]::Round($cpuMs / $secs * 60, 2)
        'CPU % (one core)'  = [math]::Round(100 * $cpuMs / 1000 / $secs, 4)
        'CPU % (whole CPU)' = [math]::Round(100 * $cpuMs / 1000 / $secs / $cpus, 5)
        'RAM (MB)'          = [math]::Round(($ws | Measure-Object -Average).Average / 1MB, 1)
        'RAM peak (MB)'     = [math]::Round(($ws | Measure-Object -Maximum).Maximum / 1MB, 1)
        'Private (MB)'      = [math]::Round(($priv | Measure-Object -Average).Average / 1MB, 1)
        Threads             = $p.Threads.Count
    }
}

$app = $null; $fake = $null
try {
    $app = Start-Process $exe -ArgumentList '--minimized' -PassThru
    Start-Sleep -Seconds 3   # let it settle
    $results = @(Measure-Phase $app 'Idle (no CS2)')

    $fake = Start-Process "$tmp\cs2.exe" -PassThru -WindowStyle Hidden
    $results += Measure-Phase $app 'Active (CS2 open)'
    $fake.Refresh()

    $results | Format-Table -AutoSize | Out-String -Width 220
    'CPU: {0} ({1} logical) | fake cs2 affinity after test: 0x{2:X}' -f `
        (Get-CimInstance Win32_Processor | Select-Object -First 1).Name.Trim(), $cpus, [int64]$fake.ProcessorAffinity
    'Executable: {0:N2} MB' -f ((Get-Item $exe).Length / 1MB)
}
finally {
    if ($fake -and -not $fake.HasExited) { Stop-Process -Id $fake.Id -Force }
    if ($app -and -not $app.HasExited) { Stop-Process -Id $app.Id -Force }
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
