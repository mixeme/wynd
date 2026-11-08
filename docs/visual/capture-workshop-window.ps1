# Capture a real rendered frame of the Wynd Workshop window (wailsWindow)
# to a PNG via PrintWindow. PW_RENDERFULLCONTENT is required for WebView2
# content; plain PrintWindow returns an empty frame.
#
# Computer Use screenshots are NOT an alternative here: the current Windows
# Computer Use helper build has no Windows Graphics Capture implementation.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File docs/visual/capture-workshop-window.ps1 -OutPath shot.png [-ProcId 123]
# Without -ProcId the single running "wynd-workshop" process is used.
param(
  [Parameter(Mandatory = $true)][string]$OutPath,
  [int]$ProcId = 0
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
Add-Type @'
using System;
using System.Runtime.InteropServices;
public class WorkshopWindowCap {
  [StructLayout(LayoutKind.Sequential)]
  public struct RECT { public int Left, Top, Right, Bottom; }
  [DllImport("user32.dll")]
  public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint nFlags);
  [DllImport("user32.dll")]
  public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
}
'@

if ($ProcId -ne 0) {
  $procs = @(Get-Process -Id $ProcId -ErrorAction Stop)
} else {
  $procs = @(Get-Process -Name 'wynd-workshop' -ErrorAction Stop)
}
if ($procs.Count -ne 1) {
  $ids = ($procs | ForEach-Object { $_.Id }) -join ', '
  throw "Expected exactly one wynd-workshop process, found $($procs.Count): $ids. Pass -ProcId."
}
$proc = $procs[0]
$hwnd = $proc.MainWindowHandle
if ($hwnd -eq [IntPtr]::Zero) { throw "Process $($proc.Id) has no main window" }

$rect = New-Object WorkshopWindowCap+RECT
[void][WorkshopWindowCap]::GetWindowRect($hwnd, [ref]$rect)
$w = $rect.Right - $rect.Left
$h = $rect.Bottom - $rect.Top
if ($w -le 0 -or $h -le 0) { throw "Empty window size" }

$bmp = New-Object System.Drawing.Bitmap($w, $h)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$hdc = $g.GetHdc()
$ok = [WorkshopWindowCap]::PrintWindow($hwnd, $hdc, 2)  # 2 = PW_RENDERFULLCONTENT
$g.ReleaseHdc($hdc)
$g.Dispose()
if (-not $ok) { $bmp.Dispose(); throw "PrintWindow returned false" }
$bmp.Save($OutPath, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
Write-Output ("OK pid={0} hwnd=0x{1:X} size={2}x{3} -> {4}" -f $proc.Id, $hwnd.ToInt64(), $w, $h, $OutPath)
