param([string]$Path)
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
  $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
  $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' })[0]
function Await($WinRtTask, $ResultType) {
  $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
  $netTask = $asTask.Invoke($null, @($WinRtTask)); $netTask.Wait(-1) | Out-Null; $netTask.Result
}
[Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder,Windows.Graphics.Imaging,ContentType=WindowsRuntime] | Out-Null
[Windows.Media.Ocr.OcrEngine,Windows.Media.Ocr,ContentType=WindowsRuntime] | Out-Null
$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
Write-Output ("LANG=" + $engine.RecognizerLanguage.LanguageTag)
$file   = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($Path)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder= Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
Write-Output ("DIMS=" + $decoder.PixelWidth + "x" + $decoder.PixelHeight)
$bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
Write-Output ("BMP=" + $bitmap.PixelWidth + "x" + $bitmap.PixelHeight + " fmt=" + $bitmap.BitmapPixelFormat)
$result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
if ($null -eq $result) { Write-Output "RESULT=null"; exit }
$lines = @($result.Lines)
Write-Output ("LINES=" + $lines.Count)
foreach ($line in $lines) {
  ($line.Words | ForEach-Object { "{0}@{1},{2},{3},{4}" -f $_.Text,[int]$_.BoundingRect.X,[int]$_.BoundingRect.Y,[int]$_.BoundingRect.Width,[int]$_.BoundingRect.Height }) -join ' '
}
