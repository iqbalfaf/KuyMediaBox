# Windows OCR bridge for KuyMediaBox.
# -List prints "tag<TAB>name" per installed OCR language.
# Otherwise it reads image paths from stdin (one per line) and prints one JSON line per image:
# {"lines":[{"w":[[text,x,y,w,h],...]}, ...]} with pixel coordinates.
param([string]$Lang = '', [switch]$List)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::InputEncoding = [System.Text.Encoding]::UTF8
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType = WindowsRuntime]
$null = [Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation, ContentType = WindowsRuntime]
$null = [Windows.Storage.StorageFile, Windows.Storage, ContentType = WindowsRuntime]
$asTask = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
  $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1'
} | Select-Object -First 1
function Await($op, [Type]$t) {
  $task = $asTask.MakeGenericMethod($t).Invoke($null, @($op))
  $task.Wait(-1) | Out-Null
  $task.Result
}
if ($List) {
  [Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages | ForEach-Object { "$($_.LanguageTag)`t$($_.DisplayName)" }
  exit 0
}
if ($Lang) { $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage([Windows.Globalization.Language]::new($Lang)) }
else { $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages() }
if ($null -eq $engine) { [Console]::Error.WriteLine('no-engine'); exit 3 }
[Console]::Out.WriteLine('ready')
[Console]::Out.Flush()
while ($null -ne ($line = [Console]::In.ReadLine())) {
  if ($line -eq '') { continue }
  try {
    $file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($line)) ([Windows.Storage.StorageFile])
    $stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
    $decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
    $bmp = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
    $res = Await ($engine.RecognizeAsync($bmp)) ([Windows.Media.Ocr.OcrResult])
    $out = New-Object System.Collections.ArrayList
    foreach ($l in $res.Lines) {
      $words = New-Object System.Collections.ArrayList
      foreach ($w in $l.Words) {
        $r = $w.BoundingRect
        [void]$words.Add(@($w.Text, [math]::Round($r.X, 1), [math]::Round($r.Y, 1), [math]::Round($r.Width, 1), [math]::Round($r.Height, 1)))
      }
      [void]$out.Add(@{ w = $words })
    }
    $stream.Dispose()
    [Console]::Out.WriteLine((ConvertTo-Json -InputObject @{ lines = $out } -Depth 6 -Compress))
  } catch {
    [Console]::Out.WriteLine((ConvertTo-Json -InputObject @{ error = $_.Exception.Message } -Compress))
  }
  [Console]::Out.Flush()
}
