# Scanner bridge for KuyMediaBox: shows the Windows scan dialog and saves the page as JPEG.
param([string]$Out)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$dlg = New-Object -ComObject WIA.CommonDialog
# ShowAcquireImage(DeviceType=Scanner, Intent=Color, Bias=MaximizeQuality, Format=JPEG, AlwaysSelectDevice, UseCommonUI, CancelError)
try {
  $img = $dlg.ShowAcquireImage(1, 1, 131072, '{B96B3CAE-0728-11D3-9D7B-0000F81EF32E}', $false, $true, $false)
} catch {
  if ($_.Exception.HResult -eq -2145320939) { [Console]::Out.WriteLine('no-device'); exit 0 }
  throw
}
if ($null -eq $img) { [Console]::Out.WriteLine('canceled'); exit 0 }
if (Test-Path $Out) { Remove-Item $Out -Force }
$img.SaveFile($Out)
[Console]::Out.WriteLine('ok')
