# Microsoft Office bridge for KuyMediaBox: converts one file through Word, Excel or PowerPoint.
# -App word|excel|powerpoint  -Mode pdf|docx  -In <source>  -Out <target>
param([string]$App, [string]$Mode = 'pdf', [string]$In, [string]$Out)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
# A dummy password makes protected files fail instead of waiting for a hidden prompt.
$noPw = 'kmb-no-password'
function Release($o) { if ($null -ne $o) { [void][Runtime.InteropServices.Marshal]::ReleaseComObject($o) } }
# Start an Office program and remember the process it created, so it can be ended for sure.
$script:ownPid = $null
function Start-Office([string]$progId, [string]$proc) {
  $before = @(Get-Process $proc -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
  $o = New-Object -ComObject $progId
  $new = @(Get-Process $proc -ErrorAction SilentlyContinue | Where-Object { $before -notcontains $_.Id })
  if ($new.Count -eq 1) { $script:ownPid = $new[0].Id }
  return $o
}
function Stop-Own {
  [GC]::Collect(); [GC]::WaitForPendingFinalizers()
  if ($script:ownPid) {
    Start-Sleep -Milliseconds 800
    Stop-Process -Id $script:ownPid -Force -ErrorAction SilentlyContinue
  }
}
switch ($App) {
  'word' {
    $w = Start-Office 'Word.Application' 'WINWORD'
    try {
      $w.Visible = $false
      $w.DisplayAlerts = 0
      $w.Options.ConfirmConversions = $false
      # Opening a PDF normally shows a "Word will now convert your PDF" prompt; switch it off
      # for this run and put the user's setting back afterwards.
      $optKey = "HKCU:\Software\Microsoft\Office\$($w.Version)\Word\Options"
      $oldWarn = $null
      if ($In.ToLower().EndsWith('.pdf')) {
        if (-not (Test-Path $optKey)) { New-Item -Path $optKey -Force | Out-Null }
        $oldWarn = (Get-ItemProperty -Path $optKey -Name DisableConvertPdfWarning -ErrorAction SilentlyContinue).DisableConvertPdfWarning
        Set-ItemProperty -Path $optKey -Name DisableConvertPdfWarning -Value 1 -Type DWord
      }
      # Open(FileName, ConfirmConversions, ReadOnly, AddToRecentFiles, PasswordDocument); Word wants refs.
      $f = $false; $t = $true; $src = $In
      try {
        $d = $w.Documents.Open([ref]$src, [ref]$f, [ref]$t, [ref]$f, [ref]$noPw)
      } finally {
        if ($In.ToLower().EndsWith('.pdf')) {
          if ($null -eq $oldWarn) { Remove-ItemProperty -Path $optKey -Name DisableConvertPdfWarning -ErrorAction SilentlyContinue }
          else { Set-ItemProperty -Path $optKey -Name DisableConvertPdfWarning -Value $oldWarn -Type DWord }
        }
      }
      if ($Mode -eq 'docx') { $d.SaveAs2($Out, 16) } else { $d.ExportAsFixedFormat($Out, 17) }
      $noSave = 0
      $d.Close([ref]$noSave)
      Release $d
    } finally { if ($script:ownPid) { $w.Quit() }; Release $w; Stop-Own }
  }
  'excel' {
    $x = Start-Office 'Excel.Application' 'EXCEL'
    try {
      $x.Visible = $false
      $x.DisplayAlerts = $false
      $x.AskToUpdateLinks = $false
      # Open(FileName, UpdateLinks, ReadOnly, Format, Password)
      $wb = $x.Workbooks.Open($In, 0, $true, [Type]::Missing, $noPw)
      $wb.ExportAsFixedFormat(0, $Out)
      $wb.Close($false)
      Release $wb
    } finally { if ($script:ownPid) { $x.Quit() }; Release $x; Stop-Own }
  }
  'powerpoint' {
    $p = Start-Office 'PowerPoint.Application' 'POWERPNT'
    try {
      # Open(FileName, ReadOnly, Untitled, WithWindow)
      $pr = $p.Presentations.Open($In + '::' + $noPw, -1, 0, 0)
      $pr.SaveAs($Out, 32)
      $pr.Close()
      Release $pr
    } finally { if ($script:ownPid) { $p.Quit() }; Release $p; Stop-Own }
  }
  default { throw "unknown app $App" }
}
[Console]::Out.WriteLine('ok')
