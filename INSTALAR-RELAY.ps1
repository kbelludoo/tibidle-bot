$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe  = Join-Path $root 'bin\ticketd.exe'

Write-Host '[MAHI68] Compilando relay de tickets (ticketd.exe)...'
Push-Location $root
go build -o $exe .\cmd\ticketd
if ($LASTEXITCODE -ne 0) {
    Pop-Location
    Write-Host '[MAHI68] Falha na compilacao.'
    exit 1
}
Pop-Location

$lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'TibidleTicketd.lnk'
$s = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
$s.TargetPath = $exe
$s.Arguments = '-accounts accounts.json'
$s.WorkingDirectory = $root
$s.Save()
Write-Host "[MAHI68] Atalho criado no Iniciar: $lnk"

Start-Process -FilePath $exe -ArgumentList '-accounts','accounts.json' -WorkingDirectory $root
Write-Host '[MAHI68] Relay rodando agora. Debug: http://127.0.0.1:3901/status | log: ticketd.log'
Read-Host 'Pressione Enter para fechar'
